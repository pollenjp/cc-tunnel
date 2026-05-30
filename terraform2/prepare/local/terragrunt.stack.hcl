# Bootstrap stack for the "local" environment.
#
# These two units provision the Terraform Runner SA and the tfstate GCS bucket
# that the rest of the setup depends on. They already exist (created by the
# classic terraform/prepare/local/* units), so after generation `terragrunt
# plan` must show no changes.
#
# IMPORTANT: `path` MUST stay "terraform_sa" / "tfstate_bucket" so that:
#   * the state prefix matches prepare/local/<name> (existing state), and
#   * root.hcl's impersonation exception ("prepare/local/terraform_sa") keeps
#     matching, i.e. the SA unit still runs with the caller's ADC (no
#     impersonation) — otherwise it would try to impersonate the SA it creates.

locals {
  catalog = "${get_repo_root()}/terraform2/catalog/units"
}

unit "terraform_sa" {
  source = "${local.catalog}/prepare_terraform_sa"
  path   = "terraform_sa"
}

unit "tfstate_bucket" {
  source = "${local.catalog}/prepare_tfstate_bucket"
  path   = "tfstate_bucket"
}
