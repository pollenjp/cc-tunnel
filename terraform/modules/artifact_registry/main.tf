resource "google_artifact_registry_repository" "repo" {
  location      = var.location
  repository_id = var.artifact_registry_repository_name
  format        = "DOCKER"
}
