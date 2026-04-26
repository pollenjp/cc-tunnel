locals {
  # NOTE: GCS bucket name length limitation 3-63 characters
  bucket_name_suffix = "-${random_string.unique_id.result}-tfstate"
  bucket_name = "${substr("${var.deploy_env}", 0, 64 - length(local.bucket_name_suffix))}${local.bucket_name_suffix}"
}

resource "random_string" "unique_id" {
  length  = 4
  numeric = false
  lower   = true
  upper   = false
  special = false
}

resource "google_storage_bucket" "tfstate_bucket" {
  name = local.bucket_name

  // https://docs.cloud.google.com/storage/docs/locations?hl=ja
  location = "ASIA-NORTHEAST1" # 東京 シングルリージョン
  # location = "ASIA1" # 東京・大阪 デュアルリージョン
  # location = "ASIA"  # アジア マルチリージョン

  force_destroy               = false
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"

  versioning {
    enabled = true
  }

  # 古いバージョンのステートが増えすぎないように
  lifecycle_rule {
    action {
      type = "Delete"
    }
    condition {
      num_newer_versions         = 10
      # days_since_noncurrent_time = 90
    }
  }
}
