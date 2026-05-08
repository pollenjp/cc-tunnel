locals {
  cra_image_name = var.cc_remote_agent_image_name
  cra_fqim       = "${var.artifact_registry_repository_location}-docker.pkg.dev/${var.project_id}/${var.artifact_registry_repository_name}/${local.cra_image_name}:latest"

  cra_builder_suffix  = "-${random_string.unique_id.result}-cra-builder"
  cra_builder_sa_name = "${substr(local.cra_image_name, 0, 30 - length(local.cra_builder_suffix))}${local.cra_builder_suffix}"

  cra_trigger_suffix = "-${random_string.unique_id.result}-cra-trigger"
  cra_trigger_name   = "${substr(local.cra_image_name, 0, 64 - length(local.cra_trigger_suffix))}${local.cra_trigger_suffix}"

  cra_dockerfile_dir = "apps/cc-remote-agent"
}

resource "google_service_account" "cra_builder_sa" {
  account_id   = local.cra_builder_sa_name
  display_name = "cc-remote-agent Cloud Build Builder SA"
}

resource "google_project_iam_member" "cra_builder_sa_roles" {
  for_each = toset(["roles/logging.logWriter"])
  project  = var.project_id
  role     = each.key
  member   = "serviceAccount:${google_service_account.cra_builder_sa.email}"
}

resource "google_artifact_registry_repository_iam_member" "cra_registry_writer" {
  location   = var.artifact_registry_repository_location
  repository = var.artifact_registry_repository_name
  role       = "roles/artifactregistry.writer"
  member     = "serviceAccount:${google_service_account.cra_builder_sa.email}"
}

resource "google_cloudbuild_trigger" "cra_trigger" {
  name     = local.cra_trigger_name
  location = var.artifact_registry_repository_location

  service_account = google_service_account.cra_builder_sa.id

  github {
    owner = local.github_owner
    name  = local.github_repo_name
    push { branch = "^${local.github_branch_name}$" }
  }
  included_files = ["${local.cra_dockerfile_dir}/**"]
  build {
    options { logging = "CLOUD_LOGGING_ONLY" }
    step {
      name = "gcr.io/cloud-builders/docker"
      dir  = local.cra_dockerfile_dir
      args = ["build", "-t", local.cra_fqim, "-f", "Dockerfile", "."]
    }
    step {
      name = "gcr.io/cloud-builders/docker"
      dir  = local.cra_dockerfile_dir
      args = ["push", local.cra_fqim]
    }
  }
}

resource "terraform_data" "cra_run_trigger_once" {
  depends_on = [
    google_cloudbuild_trigger.cra_trigger,
    google_artifact_registry_repository_iam_member.cra_registry_writer,
    google_project_iam_member.cra_builder_sa_roles,
  ]
  triggers_replace = [google_cloudbuild_trigger.cra_trigger.id]
  provisioner "local-exec" {
    interpreter = ["bash", "-euo", "pipefail", "-c"]
    command     = <<-EOT
      impersonate_flag="--impersonate-service-account=${var.terraform_runner_sa_email}"
      project_flag="--project=${var.project_id}"
      BUILD_ID=$(gcloud $impersonate_flag $project_flag \
        builds triggers run "${google_cloudbuild_trigger.cra_trigger.name}" \
        --region="${google_cloudbuild_trigger.cra_trigger.location}" \
        --branch="${local.github_branch_name}" \
        --format="value(metadata.build.id)")
      while true; do
        STATUS=$(gcloud $impersonate_flag $project_flag \
          builds describe "$BUILD_ID" \
          --region="${google_cloudbuild_trigger.cra_trigger.location}" \
          --format="value(status)")
        case "$STATUS" in
          SUCCESS) break ;;
          FAILURE|INTERNAL_ERROR|TIMEOUT|CANCELLED|EXPIRED) exit 1 ;;
          *) sleep 10 ;;
        esac
      done
    EOT
  }
}

data "google_project" "current" {
  project_id = var.project_id
}
