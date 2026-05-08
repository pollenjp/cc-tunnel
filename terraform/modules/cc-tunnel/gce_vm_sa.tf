locals {
  # Dedicated SA attached to GCE VMs created by cc-tunnel (docker_gce provider).
  # Replaces the previous use of the default compute SA.
  vm_runtime_sa_suffix = "-${random_string.unique_id.result}-vmsa"
  vm_runtime_sa_name   = "${substr(local.cloud_run_name, 0, 30 - length(local.vm_runtime_sa_suffix))}${local.vm_runtime_sa_suffix}"
}

resource "google_service_account" "vm_runtime_sa" {
  account_id   = local.vm_runtime_sa_name
  display_name = "cc-tunnel GCE VM Runtime SA"
}

# Allow the VM SA to pull the cc-remote-agent image from Artifact Registry.
resource "google_artifact_registry_repository_iam_member" "vm_runtime_sa_ar_reader" {
  location   = var.artifact_registry_repository_location
  repository = var.artifact_registry_repository_name
  role       = "roles/artifactregistry.reader"
  member     = "serviceAccount:${google_service_account.vm_runtime_sa.email}"
}

# Standard VM workload roles for logs/metrics emitted from the VM.
resource "google_project_iam_member" "vm_runtime_sa_roles" {
  for_each = toset([
    "roles/logging.logWriter",
    "roles/monitoring.metricWriter",
  ])
  project = var.project_id
  role    = each.key
  member  = "serviceAccount:${google_service_account.vm_runtime_sa.email}"
}

# Cloud Run runtime SA must be able to attach this SA when creating VMs.
resource "google_service_account_iam_member" "cr_runtime_vm_sa_user" {
  service_account_id = google_service_account.vm_runtime_sa.name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.runtime_sa.email}"
}
