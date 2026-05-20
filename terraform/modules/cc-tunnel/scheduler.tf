# Cloud Scheduler -> cc-tunnel POST /internal/reconcile-vms
#
# Safety-net VM reap path (every 6 h). Catches GCE VMs whose on-VM
# container-manager self-reaper (the primary, 10-min path) is dead
# (OOM, systemd failure, etc.). See:
#   adr/2026-05/2026-05-20T20:46:00+09:00_01_vm_reap_dual_path.md
#   docs/docker-gce-design.md §5.2
#
# Authentication: Cloud Scheduler signs a Google OIDC ID token with the
# scheduler SA as subject and `local.reconcile_vms_audience` as the
# `aud` claim. cc-tunnel validates the token via
# google.golang.org/api/idtoken.Validate and checks the email against
# RECONCILE_VMS_ALLOWED_EMAILS (set in main.tf to the scheduler SA).
#
# The audience is a static custom string rather than the Cloud Run URL
# to avoid a cyclic terraform dependency between the env var on the
# Cloud Run service and `google_cloud_run_v2_service.cloud_run.uri`.

locals {
  scheduler_sa_suffix    = "-${random_string.unique_id.result}-sched"
  scheduler_sa_name      = "${substr(local.cloud_run_name, 0, 30 - length(local.scheduler_sa_suffix))}${local.scheduler_sa_suffix}"
  reconcile_vms_audience = "cc-tunnel-reconcile-vms"
}

resource "google_service_account" "scheduler_sa" {
  account_id   = local.scheduler_sa_name
  display_name = "cc-tunnel Cloud Scheduler SA (reconcile-vms)"
}

# Scheduler SA must be a Cloud Run invoker so the HTTPS request from
# Cloud Scheduler is not rejected at the platform edge before reaching
# cc-tunnel. cc-tunnel still performs its own OIDC validation in-app.
resource "google_cloud_run_v2_service_iam_member" "scheduler_invoker" {
  location = google_cloud_run_v2_service.cloud_run.location
  name     = google_cloud_run_v2_service.cloud_run.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.scheduler_sa.email}"
}

resource "google_cloud_scheduler_job" "reconcile_vms" {
  name        = "${local.cloud_run_name}-reconcile-vms"
  description = "Safety-net VM reap for docker_gce. Primary reap is on-VM self-reaper (10 min)."
  schedule    = "0 */6 * * *"
  time_zone   = "Etc/UTC"
  region      = local.cloud_run_location

  retry_config {
    retry_count = 1
  }

  http_target {
    http_method = "POST"
    uri         = "${google_cloud_run_v2_service.cloud_run.uri}/internal/reconcile-vms"

    oidc_token {
      service_account_email = google_service_account.scheduler_sa.email
      audience              = local.reconcile_vms_audience
    }
  }

  depends_on = [google_cloud_run_v2_service_iam_member.scheduler_invoker]
}
