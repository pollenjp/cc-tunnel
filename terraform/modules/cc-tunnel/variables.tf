variable "project_id" {
  type = string
}

variable "deploy_env" {
  description = "Deployment environment name"
  type        = string
}

# ---------------------------------------------------------------------------
# GitHub source — referenced by every google_cloudbuild_trigger in this module
# (cc-tunnel / frontend / cc-remote-agent / container-manager / vm-image).
# ---------------------------------------------------------------------------
variable "github_owner" {
  description = "GitHub owner/org that hosts the cc-tunnel repository (used as the `owner` of every Cloud Build trigger in this module)."
  type        = string
  default     = "pollenjp"
}

variable "github_repo_name" {
  description = "GitHub repository name (used as the `name` of every Cloud Build trigger in this module)."
  type        = string
  default     = "cc-tunnel"
}

variable "github_branch_name" {
  description = "Branch whose pushes trigger image builds. Matched as `^$${github_branch_name}$$` in each Cloud Build trigger's push filter."
  type        = string
  default     = "main"
}

variable "cc_tunnel_dockerfile_dir" {
  description = "Path within the repository of the cc-tunnel Dockerfile directory. Used as both the Cloud Build trigger's `included_files` glob and the per-step `dir`."
  type        = string
  default     = "apps/cc-tunnel"
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
  description = "Custom VPC network name (created by this module; auto_create_subnetworks=false)"
  type        = string
  default     = "cc-tunnel"
}

variable "vpc_connector_subnet_cidr" {
  description = "CIDR range of the VPC Connector subnet (source for the container-manager firewall rule)"
  type        = string
}

variable "cc_remote_agent_subnet_cidr" {
  description = "CIDR for the cc-remote-agent VM subnet. VMs reach Artifact Registry / the public internet via the ephemeral external IP attached to each instance."
  type        = string
  default     = "10.16.0.0/20"
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

# ---------------------------------------------------------------------------
# IAP (Identity-Aware Proxy)
# ---------------------------------------------------------------------------
# OAuth brand / client は cc-tunnel-iap module 側で管理される。本モジュールは
# その outputs を terragrunt dependency 経由で受け取り、LB backend service の
# iap ブロックと IAM binding に使う。

variable "iap_enabled" {
  type        = bool
  description = "Enable IAP on the External HTTPS LB backend services. Requires iap.googleapis.com API enabled and the cc-tunnel-iap unit applied first."
  default     = false
}

variable "iap_oauth_client_id" {
  type        = string
  description = "IAP OAuth client ID (provided by the cc-tunnel-iap module). Required when iap_enabled=true."
  default     = ""
}

variable "iap_oauth_client_secret" {
  type        = string
  description = "IAP OAuth client secret (provided by the cc-tunnel-iap module). Required when iap_enabled=true."
  default     = ""
  sensitive   = true
}

variable "iap_allowed_members" {
  type        = list(string)
  description = "IAM members granted roles/iap.httpsResourceAccessor on both backend services. Format: 'user:foo@example.com' / 'group:team@example.com' / 'domain:example.com'."
  default     = []
}

# ---------------------------------------------------------------------------
# SSH debug access
# ---------------------------------------------------------------------------
variable "enable_ssh_debug" {
  type        = bool
  description = "Open SSH (TCP/22) on cc-remote-agent VMs from the IAP TCP forwarding range (35.235.240.0/20) only. Intended for ad-hoc debugging via the GCP console 'SSH in browser' button or `gcloud compute ssh --tunnel-through-iap`. The caller still needs roles/iap.tunnelResourceAccessor on the project (or instance) and an SSH credential (OS Login or metadata key)."
  default     = false
}
