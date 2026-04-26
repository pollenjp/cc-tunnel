locals {
  # fully qualified image name (with tag)
  fqim = "${var.artifact_registry_repository_location}-docker.pkg.dev/${var.project_id}/${var.artifact_registry_repository_name}/${var.image_name}:latest"

  # SA name length restriction: at least 6 characters, at most 30 characters
  builder_postfix = "-${random_string.unique_id.result}-builder"
  builder_sa_name = "${substr("${var.image_name}", 0, 30 - length(local.builder_postfix))}${local.builder_postfix}"

  # https://docs.cloud.google.com/build/docs/api/reference/rest/v1/projects.triggers
  trigger_postfix = "-${random_string.unique_id.result}-trigger"
  trigger_name    = "${substr("${var.image_name}", 0, 64 - length(local.trigger_postfix))}${local.trigger_postfix}"

  github_owner = "pollenjp"
  github_repo_name  = "cc-tunnel"
  github_branch_name = "main"
  dockerfile_dir = "apps/cc-tunnel"
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
