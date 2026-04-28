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
  default     = 8080
}

variable "frontend_image_name" {
  type    = string
  default = "frontend"
}

variable "frontend_container_port" {
  type    = number
  default = 8080
}

variable "frontend_enable_public_access" {
  type    = bool
  default = false
}

variable "cloud_sql_region" {
  type        = string
  description = "Cloud SQL region"
  default     = "us-central1"
}

variable "cloud_sql_version" {
  type        = string
  description = "Cloud SQL Postgres version (POSTGRES_17 or POSTGRES_18 if GA)"
  default     = "POSTGRES_17"
}

variable "cloud_sql_tier" {
  type        = string
  description = "Cloud SQL machine tier"
  default     = "db-custom-1-3840"
}

variable "cloud_sql_db_name" {
  type        = string
  description = "Cloud SQL database name"
  default     = "cctunnel"
}

variable "cloud_sql_user" {
  type        = string
  description = "Cloud SQL user name"
  default     = "cctunnel"
}

variable "gce_zone" {
  type        = string
  description = "GCE zone for VM (docker_gce provider)"
  default     = "us-central1-a"
}

variable "gce_machine_type" {
  type        = string
  description = "GCE machine type for VM (docker_gce provider)"
  default     = "e2-medium"
}

variable "cc_remote_agent_image_name" {
  type    = string
  default = "cc-remote-agent"
}

variable "lb_fqdn" {
  type        = string
  description = "FQDN for HTTPS LB (Google-managed SSL cert はこのドメインで発行)"
}
