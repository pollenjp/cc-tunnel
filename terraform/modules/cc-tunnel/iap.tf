# Per-LB IAP IAM bindings.
#
# OAuth brand と OAuth client は別モジュール cc-tunnel-iap で管理し、
# 生成された client_id / client_secret を terragrunt の dependency 経由で
# 入力 (var.iap_oauth_client_id / var.iap_oauth_client_secret) として
# 受け取る。本モジュールでは backend service 名を直接参照する必要がある
# IAM binding のみを保持する。
#
# 構成方針:
#   - var.iap_enabled = true で LB の両 backend service (cc-tunnel / frontend) に
#     IAP を有効化する (lb.tf の dynamic "iap" ブロック)。
#   - LB 経由で IAP が Cloud Run を呼び出すために IAP service agent
#     (service-<PROJECT_NUMBER>@gcp-sa-iap.iam.gserviceaccount.com) を
#     プロビジョニングし、両 Cloud Run service に roles/run.invoker を付与する。
#     これが無いとブラウザログイン後に
#       "The IAP service account is not provisioned"
#     のエラーになる。
#   - Cloud Run service の invoker は IAP P4SA のみ (allUsers は撤去済み, Issue #62)。
#     ingress = INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER と合わせ defense-in-depth で
#     .run.app への直接アクセス / LB 経由の未認証アクセス双方を遮断する。
#   - 許可ユーザは var.iap_allowed_members に IAM 形式で列挙
#     (例: ["user:foo@example.com", "group:team@example.com"])。

# IAP service agent (P4SA) を project に作成する。冪等。
# `gcloud beta services identity create --service=iap.googleapis.com` 相当。
resource "google_project_service_identity" "iap" {
  provider = google-beta
  count    = var.iap_enabled ? 1 : 0

  project = var.project_id
  service = "iap.googleapis.com"
}

# IAP P4SA に Cloud Run invoker を付与し、LB 経由の IAP からの呼び出しを許可する。
resource "google_cloud_run_v2_service_iam_member" "cc_tunnel_iap_invoker" {
  count = var.iap_enabled ? 1 : 0

  project  = google_cloud_run_v2_service.cloud_run.project
  location = google_cloud_run_v2_service.cloud_run.location
  name     = google_cloud_run_v2_service.cloud_run.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_project_service_identity.iap[0].email}"
}

resource "google_cloud_run_v2_service_iam_member" "frontend_iap_invoker" {
  count = var.iap_enabled ? 1 : 0

  project  = google_cloud_run_v2_service.fe_cloud_run.project
  location = google_cloud_run_v2_service.fe_cloud_run.location
  name     = google_cloud_run_v2_service.fe_cloud_run.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_project_service_identity.iap[0].email}"
}

resource "google_iap_web_backend_service_iam_member" "cc_tunnel_iap_users" {
  for_each = var.iap_enabled ? toset(var.iap_allowed_members) : toset([])

  project             = var.project_id
  web_backend_service = google_compute_backend_service.cc_tunnel_backend.name
  role                = "roles/iap.httpsResourceAccessor"
  member              = each.value
}

resource "google_iap_web_backend_service_iam_member" "frontend_iap_users" {
  for_each = var.iap_enabled ? toset(var.iap_allowed_members) : toset([])

  project             = var.project_id
  web_backend_service = google_compute_backend_service.frontend_backend.name
  role                = "roles/iap.httpsResourceAccessor"
  member              = each.value
}
