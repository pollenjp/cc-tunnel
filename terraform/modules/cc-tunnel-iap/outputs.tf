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
  value       = data.external.iap_brand.result.name
  description = "OAuth brand resource name verified to exist in the project"
}

output "brand_application_title" {
  value       = data.external.iap_brand.result.application_title
  description = "OAuth consent screen application title"
}

output "brand_support_email" {
  value       = data.external.iap_brand.result.support_email
  description = "OAuth consent screen support email"
}
