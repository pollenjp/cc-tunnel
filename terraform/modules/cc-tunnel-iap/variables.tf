variable "project_id" {
  type = string
}

variable "deploy_env" {
  description = "Deployment environment name (used as a prefix for the OAuth client display name)"
  type        = string
}

variable "terraform_runner_sa_email" {
  type        = string
  description = "Service account email to impersonate when running gcloud (matches the google provider's impersonate_service_account)."
  default     = ""
}

variable "brand_name" {
  type        = string
  description = "Existing IAP OAuth brand resource name in the form 'projects/<project_number>/brands/<brand_id>'. The brand must be created via GCP Console (APIs & Services > OAuth consent screen) since google_iap_brand is deprecated and the underlying API has been shut down. Empty string is accepted by validation only to allow terragrunt hcl validate without IAP_BRAND_NAME set; the data.external lookup at apply time still requires a real value."

  validation {
    condition     = var.brand_name == "" || can(regex("^projects/[0-9]+/brands/[0-9]+$", var.brand_name))
    error_message = "brand_name must be of the form projects/<project_number>/brands/<brand_id>."
  }
}
