locals {
  # NOTE: SA name length limitation 6-30 characters
  runner_sa_name_suffix = "-${random_string.unique_id.result}-tf-runner"
  runner_sa_name = "${substr(var.deploy_env, 0, 30 - length(local.runner_sa_name_suffix))}${local.runner_sa_name_suffix}"
}

resource "random_string" "unique_id" {
  length  = 4
  special = false
  upper   = false
  lower   = true
  numeric = false
}

# create a service account for this project
resource "google_service_account" "terraform_sa" {
  account_id   = local.runner_sa_name
  display_name = "Terraform Execution SA"
}

# grant roles/storage.admin to the service account
resource "google_project_iam_member" "sa_roles" {
  for_each = toset([
    # tfstate 等の bucket 管理
    "roles/storage.admin",

    "roles/run.admin",                        # Cloud Run v2 サービス管理 (cc-tunnel module)
    "roles/cloudsql.admin",                  # Cloud SQL Instance 管理 (cc-tunnel module)
    "roles/secretmanager.admin",             # Secret Manager 管理 (cc-tunnel module / DB password)
    "roles/cloudbuild.builds.editor",        # Cloud Build trigger 作成・更新 + run/describe (cc-tunnel module)
    "roles/artifactregistry.admin",          # Artifact Registry リポジトリの管理

    # https://docs.cloud.google.com/run/docs/securing/identity-aware-proxy-cloud-run#terraform
    # "roles/iap.admin",

    # Google Cloud API 有効化に必要
    # https://github.com/terraform-google-modules/terraform-google-project-factory/tree/main/modules/project_services#prerequisites
    "roles/serviceusage.serviceUsageAdmin",

    # project レベルでの IAM 権限を設定に必要 (ex: google_project_iam_member)
    "roles/resourcemanager.projectIamAdmin",

    # SAアカウントの作成等
    "roles/iam.serviceAccountAdmin",

    # impersonate 用 (build trigger 等に SA を指定する際に必要)
    "roles/iam.serviceAccountUser",

    # Compute Engine インスタンスの管理
    "roles/compute.admin",

    # OS Login による SSH アクセス
    "roles/compute.osLogin",
  ])

  project = var.project_id
  role    = each.value
  member  = "serviceAccount:${google_service_account.terraform_sa.email}"
}

# grant roles/iam.serviceAccountTokenCreator to developers
resource "google_service_account_iam_member" "allow_impersonation" {
  for_each = toset(var.principals)

  service_account_id = google_service_account.terraform_sa.name
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = each.value
}
