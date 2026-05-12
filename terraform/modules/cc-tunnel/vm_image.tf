locals {
  vm_image_family = "cc-tunnel-vm"
  vm_image_url    = "projects/${var.project_id}/global/images/family/${local.vm_image_family}"

  vm_image_builder_suffix  = "-${random_string.unique_id.result}-vmi-bld"
  vm_image_builder_sa_name = "${substr("cc-tunnel", 0, 30 - length(local.vm_image_builder_suffix))}${local.vm_image_builder_suffix}"

  vm_image_trigger_suffix = "-${random_string.unique_id.result}-vmi-trigger"
  vm_image_trigger_name   = "${substr("cc-tunnel-vm-image", 0, 64 - length(local.vm_image_trigger_suffix))}${local.vm_image_trigger_suffix}"

  vm_image_dir = "apps/vm-image"
}

resource "google_service_account" "vm_image_builder_sa" {
  account_id   = local.vm_image_builder_sa_name
  display_name = "cc-tunnel VM image (Packer) builder SA"
}

# Cloud Build / Packer に必要な権限。
# - logging.logWriter: Cloud Build がログを書く
# - compute.instanceAdmin.v1: Packer が一時 VM とイメージを作成・削除
# - iam.serviceAccountUser: Packer が一時 VM に default compute SA を割当てる
resource "google_project_iam_member" "vm_image_builder_sa_roles" {
  for_each = toset([
    "roles/logging.logWriter",
    "roles/compute.instanceAdmin.v1",
  ])
  project = var.project_id
  role    = each.key
  member  = "serviceAccount:${google_service_account.vm_image_builder_sa.email}"
}

resource "google_service_account_iam_member" "vm_image_builder_sa_use_default_compute_sa" {
  service_account_id = "projects/${var.project_id}/serviceAccounts/${data.google_project.current.number}-compute@developer.gserviceaccount.com"
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.vm_image_builder_sa.email}"
}

# IAM binding propagation 待ち。
# Cloud Build trigger を起動した瞬間に Packer が compute API を叩くが、
# 上記 IAM 付与の反映に GCP 側で数十秒〜2 分の遅延があり、
# 反映前にビルドが走ると "permission denied" で失敗するため待機する。
resource "time_sleep" "wait_vm_image_builder_sa_iam" {
  depends_on = [
    google_project_iam_member.vm_image_builder_sa_roles,
    google_service_account_iam_member.vm_image_builder_sa_use_default_compute_sa,
  ]

  create_duration = "120s"

  triggers = {
    project_roles = jsonencode([for r in google_project_iam_member.vm_image_builder_sa_roles : r.id])
    act_as        = google_service_account_iam_member.vm_image_builder_sa_use_default_compute_sa.id
  }
}

resource "google_cloudbuild_trigger" "vm_image_trigger" {
  name     = local.vm_image_trigger_name
  location = var.artifact_registry_repository_location

  service_account = google_service_account.vm_image_builder_sa.id

  github {
    owner = var.github_owner
    name  = var.github_repo_name
    push { branch = "^${var.github_branch_name}$" }
  }
  # Rebuild the VM image when container-manager (baked into the image) changes.
  included_files = ["${local.vm_image_dir}/**", "${local.cm_dockerfile_dir}/**"]

  filename = "${local.vm_image_dir}/cloudbuild.yaml"

  substitutions = {
    _PROJECT_ID   = var.project_id
    _ZONE         = var.gce_zone
    _IMAGE_FAMILY = local.vm_image_family
    _CM_IMAGE     = local.cm_fqim
  }
}

resource "terraform_data" "vm_image_run_trigger_once" {
  depends_on = [
    google_cloudbuild_trigger.vm_image_trigger,
    google_project_iam_member.vm_image_builder_sa_roles,
    google_service_account_iam_member.vm_image_builder_sa_use_default_compute_sa,
    time_sleep.wait_vm_image_builder_sa_iam,
    # container-manager image must exist in AR before Packer pulls it.
    terraform_data.cm_run_trigger_once,
    google_artifact_registry_repository_iam_member.cm_packer_builder_reader,
  ]
  triggers_replace = [google_cloudbuild_trigger.vm_image_trigger.id]

  provisioner "local-exec" {
    interpreter = ["bash", "-euo", "pipefail", "-c"]
    command     = <<-EOT
      impersonate_flag="--impersonate-service-account=${var.terraform_runner_sa_email}"
      project_flag="--project=${var.project_id}"
      BUILD_ID=$(gcloud $impersonate_flag $project_flag \
        builds triggers run "${google_cloudbuild_trigger.vm_image_trigger.name}" \
        --region="${google_cloudbuild_trigger.vm_image_trigger.location}" \
        --branch="${var.github_branch_name}" \
        --format="value(metadata.build.id)")
      while true; do
        STATUS=$(gcloud $impersonate_flag $project_flag \
          builds describe "$BUILD_ID" \
          --region="${google_cloudbuild_trigger.vm_image_trigger.location}" \
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
