// Cloud Build trigger that builds the metadata-test-vm Packer image.
// Mirrors terraform/modules/cc-tunnel/vm_image.tf so that the throwaway
// experiment uses the same build pipeline as production.
//
// The trigger watches GitHub push events on var.github_branch_name. For
// first-time runs (or iterating on a feature branch before merge),
// `terraform_data.packer_run_trigger_once` invokes
// `gcloud builds triggers run --branch=<var.github_branch_name>` so the
// image is built immediately on apply against the branch that actually
// contains apps/metadata-test-vm/.

locals {
  packer_dir = "apps/metadata-test-vm"

  packer_builder_suffix  = "-${random_string.unique_id.result}-mdt-bld"
  packer_builder_sa_name = "${substr("metadata-test", 0, 30 - length(local.packer_builder_suffix))}${local.packer_builder_suffix}"

  packer_trigger_suffix = "-${random_string.unique_id.result}-mdt-trg"
  packer_trigger_name   = "${substr("metadata-test-vm-image", 0, 64 - length(local.packer_trigger_suffix))}${local.packer_trigger_suffix}"
}

data "google_project" "current" {
  project_id = var.project_id
}

resource "google_service_account" "packer_builder_sa" {
  account_id   = local.packer_builder_sa_name
  display_name = "metadata-test Packer (Cloud Build) builder SA"
}

// Packer needs to: write logs, create/delete a temp VM and the resulting
// image, and impersonate the default compute SA used by the temp VM.
resource "google_project_iam_member" "packer_builder_sa_roles" {
  for_each = toset([
    "roles/logging.logWriter",
    "roles/compute.instanceAdmin.v1",
  ])
  project = var.project_id
  role    = each.key
  member  = "serviceAccount:${google_service_account.packer_builder_sa.email}"
}

resource "google_service_account_iam_member" "packer_builder_use_default_compute_sa" {
  service_account_id = "projects/${var.project_id}/serviceAccounts/${data.google_project.current.number}-compute@developer.gserviceaccount.com"
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.packer_builder_sa.email}"
}

// IAM 反映待ち。新規 SA で即 Cloud Build を走らせると Packer が compute API
// を叩いた瞬間に permission denied になることがあるため待機する
// (cc-tunnel module の vm_image.tf と同じ理由)。
resource "time_sleep" "wait_packer_builder_iam" {
  depends_on = [
    google_project_iam_member.packer_builder_sa_roles,
    google_service_account_iam_member.packer_builder_use_default_compute_sa,
  ]
  create_duration = "120s"
  triggers = {
    project_roles = jsonencode([for r in google_project_iam_member.packer_builder_sa_roles : r.id])
    act_as        = google_service_account_iam_member.packer_builder_use_default_compute_sa.id
  }
}

resource "google_cloudbuild_trigger" "packer_trigger" {
  name     = local.packer_trigger_name
  location = var.artifact_registry_repository_location

  service_account = google_service_account.packer_builder_sa.id

  github {
    owner = var.github_owner
    name  = var.github_repo_name
    push { branch = "^${var.github_branch_name}$" }
  }
  included_files = ["${local.packer_dir}/**"]

  filename = "${local.packer_dir}/cloudbuild.yaml"

  substitutions = {
    _PROJECT_ID   = var.project_id
    _ZONE         = var.zone
    _IMAGE_FAMILY = var.image_family
  }
}

resource "terraform_data" "packer_run_trigger_once" {
  depends_on = [
    google_cloudbuild_trigger.packer_trigger,
    google_project_iam_member.packer_builder_sa_roles,
    google_service_account_iam_member.packer_builder_use_default_compute_sa,
    time_sleep.wait_packer_builder_iam,
  ]
  triggers_replace = [google_cloudbuild_trigger.packer_trigger.id]

  provisioner "local-exec" {
    interpreter = ["bash", "-euo", "pipefail", "-c"]
    command     = <<-EOT
      impersonate_flag="--impersonate-service-account=${var.terraform_runner_sa_email}"
      project_flag="--project=${var.project_id}"
      echo "==> Running Cloud Build trigger: ${google_cloudbuild_trigger.packer_trigger.name} (branch=${var.github_branch_name})"
      BUILD_ID=$(gcloud $impersonate_flag $project_flag \
        builds triggers run "${google_cloudbuild_trigger.packer_trigger.name}" \
        --region="${google_cloudbuild_trigger.packer_trigger.location}" \
        --branch="${var.github_branch_name}" \
        --format="value(metadata.build.id)")
      echo "==> Waiting for build $BUILD_ID..."
      while true; do
        STATUS=$(gcloud $impersonate_flag $project_flag \
          builds describe "$BUILD_ID" \
          --region="${google_cloudbuild_trigger.packer_trigger.location}" \
          --format="value(status)")
        echo "    Build status: $STATUS"
        case "$STATUS" in
          SUCCESS) echo "==> Build completed."; break ;;
          FAILURE|INTERNAL_ERROR|TIMEOUT|CANCELLED|EXPIRED)
            echo "==> Build failed: $STATUS" >&2; exit 1 ;;
          *) sleep 10 ;;
        esac
      done
    EOT
  }
}
