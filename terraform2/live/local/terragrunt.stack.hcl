# Live stack for the "local" environment.
#
# Each unit below is generated under ./.terragrunt-stack/<path>/ by
# `terragrunt stack generate` (or implicitly by `terragrunt stack run`).
# root.hcl strips the ".terragrunt-stack" segment from the path so every unit
# adopts its EXISTING state (bucket=local-gsaq-tfstate, prefix=live/local/<path>)
# with no state move — `terragrunt plan` shows no changes after generation.
#
# Dependency graph (declared by the dependency blocks inside each catalog unit):
#
#   init ─┬─ artifact_registry ─┐
#         ├─ cc-tunnel-iap ─────┼─► cc-tunnel
#         └─ vm_image_cleaner
#
# `path` MUST match the classic directory name (init / artifact_registry /
# cc-tunnel-iap / cc-tunnel / vm_image_cleaner) so the derived state prefix
# matches the resources already managed under terraform/live/local/<name>.

locals {
  catalog = "${get_repo_root()}/terraform2/catalog/units"
}

unit "init" {
  source = "${local.catalog}/init"
  path   = "init"
}

unit "artifact_registry" {
  source = "${local.catalog}/artifact_registry"
  path   = "artifact_registry"
}

unit "cc-tunnel-iap" {
  source = "${local.catalog}/cc-tunnel-iap"
  path   = "cc-tunnel-iap"
}

unit "vm_image_cleaner" {
  source = "${local.catalog}/vm_image_cleaner"
  path   = "vm_image_cleaner"
}

unit "cc-tunnel" {
  source = "${local.catalog}/cc-tunnel"
  path   = "cc-tunnel"
}
