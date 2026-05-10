output "oauth_client_id" {
  value       = var.oauth_client_id
  description = "OAuth 2.0 client ID for IAP (consumed by the cc-tunnel module's backend service iap block)."
}

output "oauth_client_secret" {
  value       = var.oauth_client_secret
  description = "OAuth 2.0 client secret for IAP."
  sensitive   = true
}
