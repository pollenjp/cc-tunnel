locals {
  cs_instance_suffix = "-${random_string.unique_id.result}-pg"
  cs_instance_name   = "${substr(var.deploy_env, 0, 30 - length(local.cs_instance_suffix))}${local.cs_instance_suffix}"
  cs_db_name         = var.cloud_sql_db_name
  cs_user_name       = var.cloud_sql_user
}

resource "random_password" "cs_password" {
  length  = 32
  special = false
  upper   = true
  lower   = true
  numeric = true
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

  # Skip DROP DATABASE on destroy. `tf:save_cost` tears down the parent
  # cs_instance wholesale, which removes the database with it. Without
  # ABANDON, `terragrunt destroy` aborts here with:
  #
  #   Error: Error when reading or editing Database: googleapi: Error 400:
  #   Invalid request: failed to delete database cctunnel.
  #   Detail: pq: database "cctunnel" is being accessed by other users
  #
  # because Cloud Run connections to Postgres have not fully drained by
  # the time the Cloud SQL Admin API tries the DROP.
  #
  # On the orphan risk: this resource is only ever destroyed together
  # with cs_instance (it has no independent lifecycle), and the database
  # is exclusive to that instance, so the "abandoned" object is reclaimed
  # within seconds when the instance itself is deleted. There is no path
  # by which a stale cctunnel database survives past cs_instance.
  deletion_policy = "ABANDON"
}

resource "google_sql_user" "cs_user" {
  name     = local.cs_user_name
  instance = google_sql_database_instance.cs_instance.name
  password = random_password.cs_password.result

  # Skip DROP ROLE on destroy. PostgreSQL refuses to drop cctunnel while
  # it still owns objects in the cctunnel database, so without ABANDON
  # `terragrunt destroy` aborts here with:
  #
  #   Error: Error, failed to deleteuser cctunnel in instance local-ehfv-pg:
  #   googleapi: Error 400: Invalid request: failed to delete user cctunnel:
  #   . role "cctunnel" cannot be dropped because some objects depend on it
  #   Details: 7 objects in database cctunnel.
  #
  # The parent instance teardown removes the role anyway.
  #
  # Same orphan-risk reasoning as cs_db above: the role is scoped to
  # cs_instance and disappears with it, so ABANDON only delays cleanup
  # by the few seconds it takes to destroy the instance.
  deletion_policy = "ABANDON"
}

resource "google_secret_manager_secret" "cs_database_url_secret" {
  secret_id = "${local.cs_instance_name}-database-url"
  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "cs_database_url_secret_version" {
  secret = google_secret_manager_secret.cs_database_url_secret.id
  secret_data = format(
    "postgres://%s:%s@/%s?host=/cloudsql/%s&sslmode=disable",
    local.cs_user_name,
    random_password.cs_password.result,
    local.cs_db_name,
    google_sql_database_instance.cs_instance.connection_name,
  )
}

resource "google_project_iam_member" "cs_runtime_sql_client" {
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.runtime_sa.email}"
}

resource "google_secret_manager_secret_iam_member" "cs_runtime_database_url_accessor" {
  secret_id = google_secret_manager_secret.cs_database_url_secret.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.runtime_sa.email}"
}
