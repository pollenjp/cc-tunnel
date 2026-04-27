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
