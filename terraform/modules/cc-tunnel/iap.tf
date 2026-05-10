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
#   - Cloud Run service の invoker は変更しない (allUsers のまま) — ingress が
#     INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER なので .run.app への直接アクセスは
#     ブロックされ、LB 経由のみ通る。LB 前段の IAP がユーザ認証を担う。
#   - 許可ユーザは var.iap_allowed_members に IAM 形式で列挙
#     (例: ["user:foo@example.com", "group:team@example.com"])。

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
