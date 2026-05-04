locals {
  # fqim: "LOCATION-docker.pkg.dev/PROJECT/REPO/IMAGE:TAG"
  image_tag        = regex(":([^:]+)$", var.fqim)[0]
  fqim_without_tag = replace(var.fqim, ":${local.image_tag}", "")
  fqim_parts       = split("/", local.fqim_without_tag)
  ar_repo_name     = local.fqim_parts[2]
  image_name       = local.fqim_parts[3]

  # SA account_id length: 6..30
  deployer_sa_postfix = "-deployer"
  deployer_sa_name    = "${substr(var.name_prefix, 0, 30 - length(local.deployer_sa_postfix))}${local.deployer_sa_postfix}"

  workflow_name = "${var.name_prefix}-redeploy"
  trigger_name  = "${var.name_prefix}-ar-push"
}

resource "google_service_account" "deployer_sa" {
  account_id   = local.deployer_sa_name
  display_name = "Cloud Run Deployer SA (${var.cloud_run_name})"
}

# Project-scoped roles required by the redeploy workflow.
# roles/run.developer must be granted at project level — when bound only to a
# specific service, Workflows' googleapis.run.v2.projects.locations.services.get
# call fails.
resource "google_project_iam_member" "deployer_sa_project_roles" {
  for_each = toset([
    "roles/run.developer",           # Cloud Run service update + operations.get
    "roles/eventarc.eventReceiver",  # Eventarc event delivery
    "roles/workflows.invoker",       # Workflows execution
    "roles/artifactregistry.reader", # Image pull on Cloud Run update
    "roles/logging.logWriter",       # Workflow call logs (call_log_level = LOG_ALL_CALLS)
  ])

  project = var.project_id
  role    = each.value
  member  = "serviceAccount:${google_service_account.deployer_sa.email}"
}

# Allow the deployer SA to act as the Cloud Run runtime SA.
resource "google_service_account_iam_member" "deployer_act_as_runtime" {
  service_account_id = var.cloud_run_runtime_sa_id
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.deployer_sa.email}"
}

# IAM binding propagation 待ち。
# deployer SA への role 付与が反映される前に Workflow / Eventarc trigger を
# 作ると、それらが内部で deployer SA の権限チェックを行う際に失敗することが
# あるため、各 binding 完了後に 2 分待つ。
resource "time_sleep" "wait_deployer_sa_iam" {
  depends_on = [
    google_project_iam_member.deployer_sa_project_roles,
    google_service_account_iam_member.deployer_act_as_runtime,
  ]

  create_duration = "120s"

  triggers = {
    project_roles = jsonencode([for r in google_project_iam_member.deployer_sa_project_roles : r.id])
    act_as        = google_service_account_iam_member.deployer_act_as_runtime.id
  }
}

resource "google_workflows_workflow" "redeploy" {
  depends_on = [time_sleep.wait_deployer_sa_iam]

  name            = local.workflow_name
  region          = var.cloud_run_location
  service_account = google_service_account.deployer_sa.id
  call_log_level  = "LOG_ALL_CALLS"

  source_contents = <<-YAML
    main:
      params: [event]
      steps:
        - init:
            assign:
              - target_image_name: "${local.image_name}"
              - target_tag: "${local.image_tag}"
              - service_name: "projects/${var.project_id}/locations/${var.cloud_run_location}/services/${var.cloud_run_name}"
              - resource_name: $${event.data.protoPayload.resourceName}
              - request_url: $${event.data.protoPayload.request.requestUrl}
        - check_image:
            switch:
              - condition: $${not(text.match_regex(resource_name, "dockerImages/" + target_image_name))}
                return: "skipped"
        - check_tag:
            switch:
              - condition: $${not(text.match_regex(request_url, "/manifests/" + target_tag))}
                return: "skipped"
        - get_current_time:
            call: sys.now
            result: current_time
        - get_service:
            call: googleapis.run.v2.projects.locations.services.get
            args:
              name: $${service_name}
            result: service
        - ensure_annotations:
            switch:
              - condition: $${not("annotations" in service.template)}
                assign:
                  - service.template.annotations: {}
        - force_new_revision:
            assign:
              - service.template.annotations["deploy-timestamp"]: $${string(current_time)}
        - patch_service:
            call: googleapis.run.v2.projects.locations.services.patch
            args:
              name: $${service_name}
              body: $${service}
            result: patch_result
        - done:
            return: $${patch_result}
  YAML
}

resource "google_eventarc_trigger" "ar_image_push" {
  depends_on = [time_sleep.wait_deployer_sa_iam]

  name     = local.trigger_name
  location = var.cloud_run_location

  matching_criteria {
    attribute = "type"
    value     = "google.cloud.audit.log.v1.written"
  }

  matching_criteria {
    attribute = "serviceName"
    value     = "artifactregistry.googleapis.com"
  }

  matching_criteria {
    attribute = "methodName"
    value     = "Docker-PutManifest"
  }

  matching_criteria {
    attribute = "resourceName"
    operator  = "match-path-pattern"
    value     = "projects/${var.project_id}/locations/${var.cloud_run_location}/repositories/${local.ar_repo_name}/dockerImages/${local.image_name}@sha256:*"
  }

  destination {
    workflow = google_workflows_workflow.redeploy.id
  }

  service_account = google_service_account.deployer_sa.email
}
