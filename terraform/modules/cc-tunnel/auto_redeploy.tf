# Auto-redeploy Cloud Run services whenever a new image manifest is pushed
# to Artifact Registry. Wired up via Eventarc → Workflows.
#
# Audit logs for artifactregistry.googleapis.com (DATA_WRITE) must be enabled
# at the project level — handled in modules/init_project.

# cc_tunnel_auto_redeploy disabled while cc-tunnel API Cloud Run is stopped.
/*
module "cc_tunnel_auto_redeploy" {
  source = "./cloud_run_auto_redeploy"

  project_id              = var.project_id
  name_prefix             = "${random_string.unique_id.result}-cr"
  cloud_run_name          = google_cloud_run_v2_service.cloud_run.name
  cloud_run_location      = google_cloud_run_v2_service.cloud_run.location
  cloud_run_runtime_sa_id = google_service_account.runtime_sa.id
  fqim                    = local.fqim
}
*/

module "frontend_auto_redeploy" {
  source = "./cloud_run_auto_redeploy"

  project_id              = var.project_id
  name_prefix             = "${random_string.unique_id.result}-fe"
  cloud_run_name          = google_cloud_run_v2_service.fe_cloud_run.name
  cloud_run_location      = google_cloud_run_v2_service.fe_cloud_run.location
  cloud_run_runtime_sa_id = google_service_account.fe_runtime_sa.id
  fqim                    = local.fe_fqim
}
