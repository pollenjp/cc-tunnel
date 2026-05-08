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

variable "container_manager_image_name" {
  type    = string
  default = "container-manager"
}

variable "container_manager_port" {
  description = "Host port that container-manager listens on inside each cc-tunnel VM"
  type        = number
  default     = 9090
}

variable "lb_fqdn" {
  type        = string
  description = "FQDN for HTTPS LB (Google-managed SSL cert はこのドメインで発行)"
}

variable "gce_max_containers" {
  description = "Maximum number of cc-remote-agent containers per GCE instance"
  type        = number
  default     = 10
}

variable "network_name" {
  description = "VPC network name for GCE instances"
  type        = string
  default     = "default"
}

variable "vpc_connector_subnet_cidr" {
  description = "CIDR range of the VPC Connector subnet (source for the container-manager firewall rule)"
  type        = string
}

variable "cloudflare_zone_id" {
  type        = string
  description = "Cloudflare Zone ID for the DNS record (Cloudflare Dashboard > 該当 zone の Overview ページ右下から取得)"

  validation {
    condition     = length(var.cloudflare_zone_id) > 0
    error_message = "cloudflare_zone_id is required. Set the CLOUDFLARE_ZONE_ID environment variable."
  }
}

variable "cloudflare_dns_ttl" {
  type        = number
  description = "TTL (seconds). proxied=true の場合は 1 (Auto)"
  default     = 1
}

variable "cloudflare_dns_proxied" {
  type        = bool
  description = "Cloudflare のプロキシ (orange cloud) を有効にするか"
  default     = false
}

variable "cloudflare_dns_comment" {
  type        = string
  description = "Cloudflare DNS record の comment"
  default     = "cc-tunnel LB (managed by terraform)"
}
