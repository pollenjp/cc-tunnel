locals {
  activate_apis = [
    "artifactregistry.googleapis.com",
    "cloudbuild.googleapis.com",
    "compute.googleapis.com",
    "iam.googleapis.com",
    "run.googleapis.com",           # Cloud Run v2
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
