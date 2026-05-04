locals {
  activate_apis = [
    "artifactregistry.googleapis.com",
    "cloudbuild.googleapis.com",
    "compute.googleapis.com",
    "iam.googleapis.com",
    "run.googleapis.com",           # Cloud Run v2
    "sqladmin.googleapis.com",      # Cloud SQL (cc-tunnel module)
    "secretmanager.googleapis.com", # Secret Manager (cc-tunnel module / DB password)
    "eventarc.googleapis.com",      # Eventarc (cc-tunnel auto-redeploy)
    "workflows.googleapis.com",     # Workflows (cc-tunnel auto-redeploy)
    "logging.googleapis.com",       # Audit logs / workflow call logs
  ]
}

# sleep 120s for waiting API enabled
#
# > If you enabled this API recently, wait a few minutes for the action to
# > propagate to our systems and retry.
#
module "project-services" {
  source  = "terraform-google-modules/project-factory/google//modules/project_services"
  version = "18.2.0"

  project_id  = var.project_id

  activate_apis = local.activate_apis
}

resource "time_sleep" "wait_project_services" {
  depends_on = [module.project-services]

  create_duration = "120s"

  triggers = {
    # If the project_id or activated_apis list changes, we wait again.
    project_id     = var.project_id
    activated_apis = join(",", local.activate_apis)
  }
}

# Eventarc で AR の `CreateDockerImage` イベントを検知するために
# Data Access Audit Logs を有効化する
resource "google_project_iam_audit_config" "artifactregistry" {
  depends_on = [ module.project-services, time_sleep.wait_project_services ]

  project = var.project_id
  service = "artifactregistry.googleapis.com"

  audit_log_config {
    log_type = "DATA_WRITE"
  }
}

# Workflows / Eventarc の Service Agent (P4SA) を明示的に provision する。
# API を enable しただけでは agent が作られないことがあり、その場合
# google_workflows_workflow / google_eventarc_trigger の作成が
# "service agent does not exist" で失敗する。
resource "google_project_service_identity" "workflows" {
  provider = google-beta
  depends_on = [module.project-services, time_sleep.wait_project_services]

  project = var.project_id
  service = "workflows.googleapis.com"
}

resource "google_project_service_identity" "eventarc" {
  provider = google-beta
  depends_on = [module.project-services, time_sleep.wait_project_services]

  project = var.project_id
  service = "eventarc.googleapis.com"
}

# Eventarc agent には trigger が Pub/Sub topic を作るために
# roles/eventarc.serviceAgent が必要 (通常は API enable 時に自動付与されるが
# 明示しておく)
resource "google_project_iam_member" "eventarc_service_agent" {
  project = var.project_id
  role    = "roles/eventarc.serviceAgent"
  member  = "serviceAccount:${google_project_service_identity.eventarc.email}"
}
