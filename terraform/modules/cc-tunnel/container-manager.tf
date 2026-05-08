locals {
  cm_image_name = var.container_manager_image_name
  cm_fqim       = "${var.artifact_registry_repository_location}-docker.pkg.dev/${var.project_id}/${var.artifact_registry_repository_name}/${local.cm_image_name}:latest"

  cm_builder_suffix  = "-${random_string.unique_id.result}-cm-builder"
  cm_builder_sa_name = "${substr(local.cm_image_name, 0, 30 - length(local.cm_builder_suffix))}${local.cm_builder_suffix}"

  cm_trigger_suffix = "-${random_string.unique_id.result}-cm-trigger"
  cm_trigger_name   = "${substr(local.cm_image_name, 0, 64 - length(local.cm_trigger_suffix))}${local.cm_trigger_suffix}"

  cm_dockerfile_dir = "apps/container-manager"
}

resource "google_service_account" "cm_builder_sa" {
  account_id   = local.cm_builder_sa_name
  display_name = "container-manager Cloud Build Builder SA"
}

resource "google_project_iam_member" "cm_builder_sa_roles" {
  for_each = toset(["roles/logging.logWriter"])
  project  = var.project_id
  role     = each.key
  member   = "serviceAccount:${google_service_account.cm_builder_sa.email}"
}

resource "google_artifact_registry_repository_iam_member" "cm_registry_writer" {
  location   = var.artifact_registry_repository_location
  repository = var.artifact_registry_repository_name
  role       = "roles/artifactregistry.writer"
  member     = "serviceAccount:${google_service_account.cm_builder_sa.email}"
}

resource "google_cloudbuild_trigger" "cm_trigger" {
  name     = local.cm_trigger_name
  location = var.artifact_registry_repository_location

  service_account = google_service_account.cm_builder_sa.id

  github {
    owner = local.github_owner
    name  = local.github_repo_name
    push { branch = "^${local.github_branch_name}$" }
  }
  included_files = ["${local.cm_dockerfile_dir}/**"]
  build {
    options { logging = "CLOUD_LOGGING_ONLY" }
    step {
      name = "gcr.io/cloud-builders/docker"
      dir  = local.cm_dockerfile_dir
      args = ["build", "-t", local.cm_fqim, "-f", "Dockerfile", "."]
    }
    step {
      name = "gcr.io/cloud-builders/docker"
      dir  = local.cm_dockerfile_dir
      args = ["push", local.cm_fqim]
    }
  }
}

resource "terraform_data" "cm_run_trigger_once" {
  depends_on = [
    google_cloudbuild_trigger.cm_trigger,
    google_artifact_registry_repository_iam_member.cm_registry_writer,
    google_project_iam_member.cm_builder_sa_roles,
  ]
  triggers_replace = [google_cloudbuild_trigger.cm_trigger.id]
  provisioner "local-exec" {
    interpreter = ["bash", "-euo", "pipefail", "-c"]
    command     = <<-EOT
      impersonate_flag="--impersonate-service-account=${var.terraform_runner_sa_email}"
      project_flag="--project=${var.project_id}"
      BUILD_ID=$(gcloud $impersonate_flag $project_flag \
        builds triggers run "${google_cloudbuild_trigger.cm_trigger.name}" \
        --region="${google_cloudbuild_trigger.cm_trigger.location}" \
        --branch="${local.github_branch_name}" \
        --format="value(metadata.build.id)")
      while true; do
        STATUS=$(gcloud $impersonate_flag $project_flag \
          builds describe "$BUILD_ID" \
          --region="${google_cloudbuild_trigger.cm_trigger.location}" \
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

# The Packer Cloud Build worker pulls the container-manager image from
# Artifact Registry, so its SA needs reader access. (cc-remote-agent already
# has its own reader binding for Cloud Run + VM; we add one for the Packer
# build SA.)
resource "google_artifact_registry_repository_iam_member" "cm_packer_builder_reader" {
  location   = var.artifact_registry_repository_location
  repository = var.artifact_registry_repository_name
  role       = "roles/artifactregistry.reader"
  member     = "serviceAccount:${google_service_account.vm_image_builder_sa.email}"
}
