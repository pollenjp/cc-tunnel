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

# Self-delete role for the VM SA. Each VM runs container-manager which
# observes its own Docker daemon and, after SELF_REAP_TIMEOUT seconds of
# zero cc-remote-agent containers, calls compute.instances.delete on
# itself (apps/container-manager/internal/selfreaper). This is the
# primary VM reap path; see
#   adr/2026-05/2026-05-20T20:46:00+09:00_01_vm_reap_dual_path.md
#
# Scope: the role contains only compute.instances.delete, and the
# binding below carries an IAM condition that restricts the
# permission to VMs whose name starts with "cc-tunnel-" (the prefix
# DockerGCEProvider uses when provisioning — see
# apps/cc-tunnel/internal/provider/dockergce/provider.go:417). This
# means a compromised VM SA cannot delete arbitrary instances in the
# project, only instances whose name matches the cc-tunnel prefix.
# In practice each VM's only path to invoke this is on its own
# instance (it has no list permission), but the condition pins the
# upper bound.
resource "google_project_iam_custom_role" "vm_self_delete" {
  role_id     = replace("ccTunnelVmSelfDelete${random_string.unique_id.result}", "-", "")
  title       = "cc-tunnel VM self-delete"
  description = "Allows a cc-tunnel-managed VM SA to delete its own VM instance via the container-manager self-reaper."
  permissions = ["compute.instances.delete"]
}

resource "google_project_iam_member" "vm_runtime_sa_self_delete" {
  project = var.project_id
  role    = google_project_iam_custom_role.vm_self_delete.id
  member  = "serviceAccount:${google_service_account.vm_runtime_sa.email}"

  condition {
    title       = "self_delete_cc_tunnel_vms_only"
    description = "Restrict compute.instances.delete to instances whose relative name starts with cc-tunnel-."
    expression  = "resource.name.startsWith(\"projects/${var.project_id}/zones/${var.gce_zone}/instances/cc-tunnel-\")"
  }
}
