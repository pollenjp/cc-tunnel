variable "project_id" {
  type = string
}

variable "deploy_env" {
  description = "Deployment environment name"
  type        = string
}

variable "artifact_registry_repository_location" {
  type = string
}

variable "artifact_registry_repository_name" {
  type = string
}

variable "terraform_runner_sa_email" {
  type        = string
  description = "Service account email to impersonate when running gcloud commands"
}

variable "image_name" {
  description = "Image name for cloud build trigger"
  type        = string
  default     = "cc-tunnel"
}

variable "enable_public_access" {
  type        = bool
  description = "Enable public access to Cloud Run"
  default     = false
}

variable "container_port" {
  type        = number
  description = "Port the container listens on"
  default     = 5173
}
