variable "project_id" {
  description = "ID of the project in which to create the Artifact Registry repository."
  type = string
}

variable "location" {
  description = "Location where the Artifact Registry repository will be created. (e.g., 'us-central1')"
  type = string
}

variable "artifact_registry_repository_name" {
  description = "Name of the Artifact Registry repository. (e.g., 'cc-tunnel')"
  type = string
  default = "cc-tunnel"
}
