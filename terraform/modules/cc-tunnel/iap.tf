# IAP (Identity-Aware Proxy) for the External HTTPS LB.
#
# 構成方針:
#   - var.iap_enabled = true で LB の両 backend service (cc-tunnel / frontend) に
#     IAP を有効化する。
#   - Cloud Run service の invoker は変更しない (allUsers のまま) — ingress が
#     INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER なので .run.app への直接アクセスは
#     ブロックされ、LB 経由のみ通る。LB 前段の IAP がユーザ認証を担う。
#   - OAuth ブランドはプロジェクトに 1 個しか作れない。既存ブランドがある場合は
#     var.iap_create_brand = false にして var.iap_existing_brand_name に
#     "projects/<num>/brands/<num>" 形式で渡す。
#   - 許可ユーザは var.iap_allowed_members に IAM 形式で列挙
#     (例: ["user:foo@example.com", "group:team@example.com"])。

resource "google_iap_brand" "brand" {
  count = var.iap_enabled && var.iap_create_brand ? 1 : 0

  project           = var.project_id
  support_email     = var.iap_support_email
  application_title = var.iap_application_title
}

resource "google_iap_client" "client" {
  count = var.iap_enabled ? 1 : 0

  display_name = "${var.deploy_env}-iap-client"
  brand        = var.iap_create_brand ? google_iap_brand.brand[0].name : var.iap_existing_brand_name
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
