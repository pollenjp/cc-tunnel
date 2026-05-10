# Project-scoped IAP resources (OAuth brand + OAuth client) used by the
# cc-tunnel External HTTPS LB. Split out from cc-tunnel module so that:
#
#   - the OAuth brand (which is a project-singleton) is owned by a dedicated
#     unit and is not torn down when cc-tunnel itself is destroyed, and
#   - the per-LB IAM bindings (which need backend-service names) can stay in
#     the cc-tunnel module while the credentials flow in via terragrunt
#     dependency outputs.

resource "google_iap_brand" "brand" {
  count = var.create_brand ? 1 : 0

  project           = var.project_id
  support_email     = var.support_email
  application_title = var.application_title
}

resource "google_iap_client" "client" {
  display_name = "${var.deploy_env}-iap-client"
  brand        = var.create_brand ? google_iap_brand.brand[0].name : var.existing_brand_name
}
