locals {
  # fully qualified image name (with tag)
  fqim = "${var.artifact_registry_repository_location}-docker.pkg.dev/${var.project_id}/${var.artifact_registry_repository_name}/${var.image_name}:latest"

  # SA name length restriction: at least 6 characters, at most 30 characters
  builder_suffix  = "-${random_string.unique_id.result}-builder"
  builder_sa_name = "${substr("${var.image_name}", 0, 30 - length(local.builder_suffix))}${local.builder_suffix}"

  # https://docs.cloud.google.com/build/docs/api/reference/rest/v1/projects.triggers
  trigger_suffix = "-${random_string.unique_id.result}-trigger"
  trigger_name   = "${substr("${var.image_name}", 0, 64 - length(local.trigger_suffix))}${local.trigger_suffix}"

  github_owner       = "pollenjp"
  github_repo_name   = "cc-tunnel"
  github_branch_name = "main"
  dockerfile_dir     = "apps/cc-tunnel"
}

resource "random_string" "unique_id" {
  length  = 4
  special = false
  upper   = false
  lower   = true
  numeric = false
}

resource "google_service_account" "cloudbuild_builder_sa" {
  account_id   = local.builder_sa_name
  display_name = "Cloud Build Builder SA"
}

resource "google_project_iam_member" "cloudbuild_builder_sa_roles" {
  for_each = toset([
    "roles/logging.logWriter",
  ])
  project = var.project_id
  role    = each.key
  member  = "serviceAccount:${google_service_account.cloudbuild_builder_sa.email}"
}

resource "google_artifact_registry_repository_iam_member" "cloudbuild_registry_writer" {
  location   = var.artifact_registry_repository_location
  repository = var.artifact_registry_repository_name
  role       = "roles/artifactregistry.writer"
  member     = "serviceAccount:${google_service_account.cloudbuild_builder_sa.email}"
}

resource "google_cloudbuild_trigger" "trigger" {
  name     = local.trigger_name
  location = var.artifact_registry_repository_location

  service_account = google_service_account.cloudbuild_builder_sa.id

  github {
    owner = local.github_owner
    name  = local.github_repo_name
    push {
      branch = "^${local.github_branch_name}$"
    }
  }

  included_files = ["${local.dockerfile_dir}/**"]

  build {
    options {
      logging = "CLOUD_LOGGING_ONLY"
    }
    step {
      name = "gcr.io/cloud-builders/docker"
      dir  = local.dockerfile_dir
      args = ["build", "-t", "${local.fqim}", "-f", "Dockerfile", "."]
    }
    step {
      name = "gcr.io/cloud-builders/docker"
      dir  = local.dockerfile_dir
      args = ["push", "${local.fqim}"]
    }
  }
}

resource "terraform_data" "run_trigger_once" {
  depends_on = [
    google_cloudbuild_trigger.trigger,
    google_artifact_registry_repository_iam_member.cloudbuild_registry_writer,
    google_project_iam_member.cloudbuild_builder_sa_roles,
  ]

  triggers_replace = [google_cloudbuild_trigger.trigger.id]

  provisioner "local-exec" {
    interpreter = ["bash", "-euo", "pipefail", "-c"]
    command     = <<-EOT
      impersonate_flag="--impersonate-service-account=${var.terraform_runner_sa_email}"
      project_flag="--project=${var.project_id}"

      echo "==> Running Cloud Build trigger: ${google_cloudbuild_trigger.trigger.name}"
      BUILD_ID=$(
        gcloud \
          "$impersonate_flag" \
          "$project_flag" \
          builds triggers run "${google_cloudbuild_trigger.trigger.name}" \
            --region="${google_cloudbuild_trigger.trigger.location}" \
            --branch="${local.github_branch_name}" \
            --format="value(metadata.build.id)"
      )

      echo "==> Waiting for build $BUILD_ID to complete..."
      while true; do
        STATUS=$(
          gcloud "$impersonate_flag" "$project_flag" \
            builds describe "$BUILD_ID" \
              --region="${google_cloudbuild_trigger.trigger.location}" \
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

      echo "==> Verifying image exists: ${local.fqim}"
      gcloud "$impersonate_flag" "$project_flag" \
        artifacts docker images describe "${local.fqim}"
      echo "==> Verified: ${local.fqim} exists."
    EOT
  }
}

# ==============================================================================

locals {
  cloud_run_location = var.artifact_registry_repository_location

  # SA name length restriction: at least 6 characters, at most 30 characters
  cloud_run_name_suffix = "-${random_string.unique_id.result}-cr"
  cloud_run_name        = "${substr("${var.deploy_env}", 0, 30 - length(local.cloud_run_name_suffix))}${local.cloud_run_name_suffix}"

  # SA name length restriction: at least 6 characters, at most 30 characters
  cr_runtime_sa_suffix = "-${random_string.unique_id.result}-runtime"
  cr_runtime_sa_name   = "${substr("${local.cloud_run_name}", 0, 30 - length(local.cr_runtime_sa_suffix))}${local.cr_runtime_sa_suffix}"
}

resource "google_service_account" "runtime_sa" {
  account_id   = local.cr_runtime_sa_name
  display_name = "Cloud Run Runtime SA"
}

resource "google_cloud_run_v2_service" "cloud_run" {
  name                = local.cloud_run_name
  location            = local.cloud_run_location
  ingress             = "INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER"
  deletion_protection = false

  template {
    service_account = google_service_account.runtime_sa.email
    timeout         = "3600s" # 60min
    containers {
      image = local.fqim
      ports {
        container_port = var.container_port
      }
      # 'PORT' is a special environment variable in Cloud Run. Don't set it manually.
      # https://docs.cloud.google.com/run/docs/configuring/services/environment-variables#best-practices
      volume_mounts {
        name       = "cloudsql"
        mount_path = "/cloudsql"
      }
      env {
        name = "DATABASE_URL"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.cs_database_url_secret.secret_id
            version = "latest"
          }
        }
      }
      env {
        name  = "EXECUTION_PROVIDER"
        value = "docker_gce"
      }
      env {
        name  = "GCE_PROJECT_ID"
        value = var.project_id
      }
      env {
        name  = "GCE_ZONE"
        value = var.gce_zone
      }
      env {
        name  = "GCE_MACHINE_TYPE"
        value = var.gce_machine_type
      }
      env {
        name  = "CC_REMOTE_AGENT_IMAGE"
        value = local.cra_fqim
      }
    }
    volumes {
      name = "cloudsql"
      cloud_sql_instance {
        instances = [google_sql_database_instance.cs_instance.connection_name]
      }
    }
  }

  depends_on = [
    google_project_iam_member.cs_runtime_sql_client,
    google_secret_manager_secret_iam_member.cs_runtime_database_url_accessor,
    google_secret_manager_secret_version.cs_database_url_secret_version,
    google_project_iam_member.cr_runtime_compute_admin,
    google_service_account_iam_member.cr_runtime_default_compute_sa_user,
    google_artifact_registry_repository_iam_member.cra_default_compute_sa_reader,
    terraform_data.cra_run_trigger_once,
  ]

  lifecycle {
    ignore_changes = [
      # NOTE: ignore changes to annotations to avoid unnecessary recreation
      #       'deploy-timestamp' is added by terraform-google-cloud-run-auto-deploy module
      template[0].annotations["deploy-timestamp"],
    ]
  }

  # NOTE:
  # Allow Terraform to create the service even if the image doesn't exist yet (though it might fail on first run)
  # In a real scenario, you'd run the build once before applying this, or use a placeholder image.
}

resource "google_cloud_run_v2_service_iam_member" "public_access" {
  count = var.enable_public_access ? 1 : 0

  location = google_cloud_run_v2_service.cloud_run.location
  name     = google_cloud_run_v2_service.cloud_run.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

resource "google_project_iam_member" "cr_runtime_compute_admin" {
  project = var.project_id
  role    = "roles/compute.instanceAdmin.v1"
  member  = "serviceAccount:${google_service_account.runtime_sa.email}"
}

resource "google_service_account_iam_member" "cr_runtime_default_compute_sa_user" {
  service_account_id = "projects/${var.project_id}/serviceAccounts/${data.google_project.current.number}-compute@developer.gserviceaccount.com"
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.runtime_sa.email}"
}
