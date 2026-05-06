output "job_name" {
  description = "Cloud Run Job name that performs the cleanup"
  value       = google_cloud_run_v2_job.cleanup.name
}

output "scheduler_job_id" {
  description = "Cloud Scheduler job ID that triggers the cleanup"
  value       = google_cloud_scheduler_job.cleanup.id
}

output "runner_sa_email" {
  description = "Service account email that runs the cleanup job"
  value       = google_service_account.runner_sa.email
}
