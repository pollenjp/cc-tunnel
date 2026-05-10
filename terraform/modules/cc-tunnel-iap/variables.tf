variable "oauth_client_id" {
  type        = string
  description = "OAuth 2.0 client ID created manually in GCP Console (APIs & Services > Credentials). Required because google_iap_client is deprecated and its underlying API has been shut down."

  validation {
    condition     = var.oauth_client_id == "" || can(regex("^[0-9]+-[0-9a-z]+\\.apps\\.googleusercontent\\.com$", var.oauth_client_id))
    error_message = "oauth_client_id must look like '<project_number>-<hash>.apps.googleusercontent.com' (or empty for hcl validate)."
  }
}

variable "oauth_client_secret" {
  type        = string
  description = "OAuth 2.0 client secret paired with oauth_client_id."
  sensitive   = true
}
