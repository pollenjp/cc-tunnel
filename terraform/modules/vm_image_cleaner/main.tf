locals {
  # SA account_id length: 6..30
  runner_sa_suffix    = "-runner"
  runner_sa_name      = "${substr(var.name_prefix, 0, 30 - length(local.runner_sa_suffix))}${local.runner_sa_suffix}"
  scheduler_sa_suffix = "-sched"
  scheduler_sa_name   = "${substr(var.name_prefix, 0, 30 - length(local.scheduler_sa_suffix))}${local.scheduler_sa_suffix}"

  job_name       = substr("${var.name_prefix}-job", 0, 63)
  scheduler_name = substr("${var.name_prefix}-cron", 0, 500)

  cleanup_script = <<-EOT
    set -euo pipefail
    : "$${PROJECT_ID:?PROJECT_ID is required}"
    : "$${IMAGE_FAMILY:?IMAGE_FAMILY is required}"
    : "$${KEEP_COUNT:?KEEP_COUNT is required}"

    echo "==> Listing images in family=$${IMAGE_FAMILY} (project=$${PROJECT_ID})"
    images=$(gcloud compute images list \
      --project="$${PROJECT_ID}" \
      --filter="family=$${IMAGE_FAMILY}" \
      --sort-by=~creationTimestamp \
      --format="value(name)")

    total=$(printf '%s\n' "$${images}" | grep -c . || true)
    echo "    Found $${total} image(s); keep latest $${KEEP_COUNT}"

    if [ "$${total}" -le "$${KEEP_COUNT}" ]; then
      echo "==> Nothing to delete."
      exit 0
    fi

    to_delete=$(printf '%s\n' "$${images}" | tail -n +$$((KEEP_COUNT + 1)))
    printf '%s\n' "$${to_delete}" | while IFS= read -r name; do
      [ -z "$${name}" ] && continue
      echo "==> Deleting image: $${name}"
      gcloud compute images delete "$${name}" --project="$${PROJECT_ID}" --quiet
    done
    echo "==> Done."
  EOT
}

# Service account that the Cloud Run Job runs as.
# Needs permission to list/delete GCE images in the project.
resource "google_service_account" "runner_sa" {
  account_id   = local.runner_sa_name
  display_name = "VM image cleaner runner SA (${var.image_family})"
}

# roles/compute.storageAdmin grants compute.images.{list,delete,get}.
# Scoped at project level; the script filters by image family at runtime.
resource "google_project_iam_member" "runner_sa_compute_storage_admin" {
  project = var.project_id
  role    = "roles/compute.storageAdmin"
  member  = "serviceAccount:${google_service_account.runner_sa.email}"
}

# IAM binding propagation 待ち。
# Cloud Run Job 起動直後に gcloud が compute API を叩くので、
# 上記 IAM 反映前に走ると permission denied で失敗する。
resource "time_sleep" "wait_runner_sa_iam" {
  depends_on = [google_project_iam_member.runner_sa_compute_storage_admin]

  create_duration = "120s"

  triggers = {
    binding = google_project_iam_member.runner_sa_compute_storage_admin.id
  }
}

resource "google_cloud_run_v2_job" "cleanup" {
  depends_on = [time_sleep.wait_runner_sa_iam]

  name                = local.job_name
  location            = var.region
  deletion_protection = false

  template {
    template {
      service_account = google_service_account.runner_sa.email
      max_retries     = 1
      timeout         = "600s"

      containers {
        image   = "gcr.io/google.com/cloudsdktool/cloud-sdk:slim"
        command = ["/bin/bash", "-c"]
        args    = [local.cleanup_script]

        env {
          name  = "PROJECT_ID"
          value = var.project_id
        }
        env {
          name  = "IMAGE_FAMILY"
          value = var.image_family
        }
        env {
          name  = "KEEP_COUNT"
          value = tostring(var.keep_count)
        }
      }
    }
  }
}

# Service account that Cloud Scheduler uses to invoke the job.
resource "google_service_account" "scheduler_sa" {
  account_id   = local.scheduler_sa_name
  display_name = "VM image cleaner scheduler SA (${var.image_family})"
}

resource "google_cloud_run_v2_job_iam_member" "scheduler_invoker" {
  project  = google_cloud_run_v2_job.cleanup.project
  location = google_cloud_run_v2_job.cleanup.location
  name     = google_cloud_run_v2_job.cleanup.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.scheduler_sa.email}"
}

resource "time_sleep" "wait_scheduler_sa_iam" {
  depends_on = [google_cloud_run_v2_job_iam_member.scheduler_invoker]

  create_duration = "60s"

  triggers = {
    binding = google_cloud_run_v2_job_iam_member.scheduler_invoker.id
  }
}

resource "google_cloud_scheduler_job" "cleanup" {
  depends_on = [time_sleep.wait_scheduler_sa_iam]

  name        = local.scheduler_name
  region      = var.region
  schedule    = var.schedule
  time_zone   = var.time_zone
  description = "Daily cleanup of GCE images in family '${var.image_family}', keeping latest ${var.keep_count}"

  attempt_deadline = "320s"

  retry_config {
    retry_count = 1
  }

  http_target {
    http_method = "POST"
    uri         = "https://run.googleapis.com/v2/projects/${var.project_id}/locations/${var.region}/jobs/${google_cloud_run_v2_job.cleanup.name}:run"

    oauth_token {
      service_account_email = google_service_account.scheduler_sa.email
    }
  }
}
