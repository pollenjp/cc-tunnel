output "artifact_registry_repository_name" {
  description = "Name of the Artifact Registry repository. (e.g., 'cc-tunnel')"
  value       = google_artifact_registry_repository.repo.name
}

output "artifact_registry_repository_location" {
  description = "Location of the Artifact Registry repository. (e.g., 'us-central1')"
  value       = google_artifact_registry_repository.repo.location
}
