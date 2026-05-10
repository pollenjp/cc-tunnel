output "cc_tunnel_url" {
  value       = google_cloud_run_v2_service.cloud_run.uri
  description = "cc-tunnel API Cloud Run service URL"
}

output "frontend_url" {
  value = google_cloud_run_v2_service.fe_cloud_run.uri
}

output "cloud_sql_instance_connection_name" {
  value       = google_sql_database_instance.cs_instance.connection_name
  description = "Cloud SQL Instance connection name (PROJECT:REGION:INSTANCE)"
}

output "cloud_sql_db_name" {
  value       = google_sql_database.cs_db.name
  description = "Cloud SQL database name"
}

output "cloud_sql_database_url_secret_id" {
  value       = google_secret_manager_secret.cs_database_url_secret.secret_id
  description = "Secret Manager secret_id for DATABASE_URL"
  sensitive   = true
}

output "cc_login_encryption_key_secret_id" {
  value       = google_secret_manager_secret.cc_login_encryption_key.secret_id
  description = "Secret Manager secret_id for CC_LOGIN_ENCRYPTION_KEY"
  sensitive   = true
}

output "cc_remote_agent_image" {
  value       = local.cra_fqim
  description = "Artifact Registry image URL for cc-remote-agent"
}

output "lb_ip" {
  value       = google_compute_global_address.lb_ip.address
  description = "Global LB external IP address (Cloudflare の A レコードに設定)"
}

output "lb_https_url" {
  value       = "https://${var.lb_fqdn}"
  description = "LB HTTPS endpoint URL"
}

output "cloudflare_dns_record_id" {
  value       = cloudflare_dns_record.lb.id
  description = "Cloudflare DNS record ID for the LB"
}

output "cloudflare_dns_record_hostname" {
  value       = cloudflare_dns_record.lb.name
  description = "Cloudflare DNS record hostname (FQDN)"
}

output "iap_oauth_client_id" {
  value       = var.iap_enabled ? google_iap_client.client[0].client_id : null
  description = "OAuth client ID used by IAP (null when iap_enabled=false)"
}

output "iap_brand_name" {
  value       = var.iap_enabled ? (var.iap_create_brand ? google_iap_brand.brand[0].name : var.iap_existing_brand_name) : null
  description = "OAuth brand resource name used by IAP (null when iap_enabled=false)"
}
