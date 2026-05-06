resource "random_bytes" "cc_login_encryption_key" {
  length = 32
}

resource "google_secret_manager_secret" "cc_login_encryption_key" {
  secret_id = "${local.cloud_run_name}-login-encryption-key"
  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "cc_login_encryption_key" {
  secret      = google_secret_manager_secret.cc_login_encryption_key.id
  secret_data = random_bytes.cc_login_encryption_key.hex
}

# Disabled while Cloud Run runtime SA is disabled. Re-enable by removing the surrounding block comment.
/*
resource "google_secret_manager_secret_iam_member" "cc_runtime_login_encryption_key_accessor" {
  secret_id = google_secret_manager_secret.cc_login_encryption_key.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.runtime_sa.email}"
}
*/
