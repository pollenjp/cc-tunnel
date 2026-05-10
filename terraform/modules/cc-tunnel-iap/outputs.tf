output "oauth_client_id" {
  value       = google_iap_client.client.client_id
  description = "IAP OAuth client ID (consumed by the cc-tunnel module's backend service iap block)"
}

output "oauth_client_secret" {
  value       = google_iap_client.client.secret
  description = "IAP OAuth client secret"
  sensitive   = true
}

output "brand_name" {
  value       = var.create_brand ? google_iap_brand.brand[0].name : var.existing_brand_name
  description = "OAuth brand resource name in use"
}
