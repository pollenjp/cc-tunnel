locals {
  cs_instance_suffix = "-${random_string.unique_id.result}-pg"
  cs_instance_name   = "${substr(var.deploy_env, 0, 30 - length(local.cs_instance_suffix))}${local.cs_instance_suffix}"
  cs_db_name         = var.cloud_sql_db_name
  cs_user_name       = var.cloud_sql_user
}

resource "random_password" "cs_password" {
  length           = 24
  special          = true
  override_special = "!@#$%^&*()-_=+[]"
}

resource "google_sql_database_instance" "cs_instance" {
  name                = local.cs_instance_name
  database_version    = var.cloud_sql_version
  region              = var.cloud_sql_region
  deletion_protection = false

  settings {
    tier              = var.cloud_sql_tier
    edition           = "ENTERPRISE"
    availability_type = "ZONAL"
    disk_type         = "PD_SSD"
    disk_size         = 10
    disk_autoresize   = true

    backup_configuration {
      enabled                        = true
      point_in_time_recovery_enabled = false
    }

    ip_configuration {
      ipv4_enabled = true
    }
  }
}

resource "google_sql_database" "cs_db" {
  name     = local.cs_db_name
  instance = google_sql_database_instance.cs_instance.name
}

resource "google_sql_user" "cs_user" {
  name     = local.cs_user_name
  instance = google_sql_database_instance.cs_instance.name
  password = random_password.cs_password.result
}

resource "google_secret_manager_secret" "cs_password_secret" {
  secret_id = "${local.cs_instance_name}-password"
  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "cs_password_secret_version" {
  secret      = google_secret_manager_secret.cs_password_secret.id
  secret_data = random_password.cs_password.result
}

resource "google_project_iam_member" "cs_runtime_sql_client" {
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.runtime_sa.email}"
}

resource "google_secret_manager_secret_iam_member" "cs_runtime_secret_accessor" {
  secret_id = google_secret_manager_secret.cs_password_secret.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.runtime_sa.email}"
}
