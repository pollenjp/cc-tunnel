variable "oauth_client_id" {
  type        = string
  description = "OAuth 2.0 client ID created manually in GCP Console (APIs & Services > Credentials). Required because google_iap_client is deprecated and its underlying API has been shut down."

  validation {
    condition     = can(regex("^[0-9]+-[0-9a-z]+\\.apps\\.googleusercontent\\.com$", var.oauth_client_id))
    error_message = "oauth_client_id must be non-empty and look like '<project_number>-<hash>.apps.googleusercontent.com'. Set the IAP_OAUTH_CLIENT_ID env var before running terragrunt."
  }
}

variable "oauth_client_secret" {
  type        = string
  description = "OAuth 2.0 client secret paired with oauth_client_id."
  sensitive   = true

  validation {
    condition     = length(var.oauth_client_secret) >= 16
    error_message = "oauth_client_secret must be non-empty (at least 16 chars). Set the IAP_OAUTH_CLIENT_SECRET env var before running terragrunt."
  }
}
