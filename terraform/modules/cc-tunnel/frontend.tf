locals {
  fe_fqim = "${var.artifact_registry_repository_location}-docker.pkg.dev/${var.project_id}/${var.artifact_registry_repository_name}/${var.frontend_image_name}:latest"

  fe_builder_suffix  = "-${random_string.unique_id.result}-fe-builder"
  fe_builder_sa_name = "${substr(var.frontend_image_name, 0, 30 - length(local.fe_builder_suffix))}${local.fe_builder_suffix}"

  fe_trigger_suffix = "-${random_string.unique_id.result}-fe-trigger"
  fe_trigger_name   = "${substr(var.frontend_image_name, 0, 64 - length(local.fe_trigger_suffix))}${local.fe_trigger_suffix}"

  fe_dockerfile_dir = "apps/frontend"

  fe_cloud_run_name_suffix = "-${random_string.unique_id.result}-fe-cr"
  fe_cloud_run_name        = "${substr(var.deploy_env, 0, 30 - length(local.fe_cloud_run_name_suffix))}${local.fe_cloud_run_name_suffix}"

  fe_runtime_sa_suffix = "-${random_string.unique_id.result}-fe-rt"
  fe_runtime_sa_name   = "${substr(local.fe_cloud_run_name, 0, 30 - length(local.fe_runtime_sa_suffix))}${local.fe_runtime_sa_suffix}"
}

resource "google_service_account" "fe_builder_sa" {
  account_id   = local.fe_builder_sa_name
  display_name = "Frontend Cloud Build Builder SA"
}

resource "google_project_iam_member" "fe_builder_sa_roles" {
  for_each = toset([
    "roles/logging.logWriter",
  ])
  project = var.project_id
  role    = each.key
  member  = "serviceAccount:${google_service_account.fe_builder_sa.email}"
}

resource "google_artifact_registry_repository_iam_member" "fe_registry_writer" {
  location   = var.artifact_registry_repository_location
  repository = var.artifact_registry_repository_name
  role       = "roles/artifactregistry.writer"
  member     = "serviceAccount:${google_service_account.fe_builder_sa.email}"
}

resource "google_cloudbuild_trigger" "fe_trigger" {
  name     = local.fe_trigger_name
  location = var.artifact_registry_repository_location

  service_account = google_service_account.fe_builder_sa.id

  github {
    owner = var.github_owner
    name  = var.github_repo_name
    push {
      branch = "^${var.github_branch_name}$"
    }
  }

  included_files = ["apps/frontend/**"]

  build {
    options {
      logging = "CLOUD_LOGGING_ONLY"
    }
    step {
      name = "gcr.io/cloud-builders/docker"
      dir  = local.fe_dockerfile_dir
      args = ["build", "-t", "${local.fe_fqim}", "-f", "Dockerfile", "."]
    }
    step {
      name = "gcr.io/cloud-builders/docker"
      dir  = local.fe_dockerfile_dir
      args = ["push", "${local.fe_fqim}"]
    }
  }
}

resource "terraform_data" "fe_run_trigger_once" {
  depends_on = [
    google_cloudbuild_trigger.fe_trigger,
    google_artifact_registry_repository_iam_member.fe_registry_writer,
    google_project_iam_member.fe_builder_sa_roles,
  ]

  triggers_replace = [google_cloudbuild_trigger.fe_trigger.id]

  provisioner "local-exec" {
    interpreter = ["bash", "-euo", "pipefail", "-c"]
    command     = <<-EOT
      impersonate_flag="--impersonate-service-account=${var.terraform_runner_sa_email}"
      project_flag="--project=${var.project_id}"

      echo "==> Running Cloud Build trigger: ${google_cloudbuild_trigger.fe_trigger.name}"
      BUILD_ID=$(
        gcloud \
          "$impersonate_flag" \
          "$project_flag" \
          builds triggers run "${google_cloudbuild_trigger.fe_trigger.name}" \
            --region="${google_cloudbuild_trigger.fe_trigger.location}" \
            --branch="${var.github_branch_name}" \
            --format="value(metadata.build.id)"
      )

      echo "==> Waiting for build $BUILD_ID to complete..."
      while true; do
        STATUS=$(
          gcloud "$impersonate_flag" "$project_flag" \
            builds describe "$BUILD_ID" \
              --region="${google_cloudbuild_trigger.fe_trigger.location}" \
              --format="value(status)"
        )
        echo "    Build status: $STATUS"
        case "$STATUS" in
          SUCCESS)
            echo "==> Build completed successfully."
            break
            ;;
          FAILURE|INTERNAL_ERROR|TIMEOUT|CANCELLED|EXPIRED)
            echo "==> Build failed with status: $STATUS" >&2
            exit 1
            ;;
          *)
            sleep 10
            ;;
        esac
      done

      echo "==> Verifying image exists: ${local.fe_fqim}"
      gcloud "$impersonate_flag" "$project_flag" \
        artifacts docker images describe "${local.fe_fqim}"
      echo "==> Verified: ${local.fe_fqim} exists."
    EOT
  }
}

resource "google_service_account" "fe_runtime_sa" {
  account_id   = local.fe_runtime_sa_name
  display_name = "Frontend Cloud Run Runtime SA"
}

resource "google_cloud_run_v2_service" "fe_cloud_run" {
  depends_on = [
    terraform_data.run_trigger_once,
  ]

  name                = local.fe_cloud_run_name
  location            = var.artifact_registry_repository_location
  ingress             = "INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER"
  deletion_protection = false

  template {
    service_account = google_service_account.fe_runtime_sa.email
    timeout         = "3600s"
    containers {
      image = local.fe_fqim
      ports {
        container_port = var.frontend_container_port
      }
      env {
        name  = "API_UPSTREAM"
        value = google_cloud_run_v2_service.cloud_run.uri
      }
      env {
        name  = "BACKEND_URL"
        value = "/api"
      }
    }
  }

  lifecycle {
    ignore_changes = [
      template[0].annotations["deploy-timestamp"],
    ]
  }
}

resource "google_cloud_run_v2_service_iam_member" "fe_public_access" {
  count = var.frontend_enable_public_access ? 1 : 0

  location = google_cloud_run_v2_service.fe_cloud_run.location
  name     = google_cloud_run_v2_service.fe_cloud_run.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}
