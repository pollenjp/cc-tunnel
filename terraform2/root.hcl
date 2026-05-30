locals {
  product_name = "cc-tunnel"
  possible_envs = [
    "local",
  ]

  # ---------------------------------------------------------------------------
  # Logical path (the key to a zero-state-move migration to Terragrunt Stacks)
  # ---------------------------------------------------------------------------
  # When a unit is materialized from a terragrunt.stack.hcl, Terragrunt renders
  # it under a generated ".terragrunt-stack" directory, e.g.
  #     live/local/.terragrunt-stack/cc-tunnel
  # and path_relative_to_include() returns *that* generated path.
  #
  # If we fed that straight into the backend prefix / impersonation exception,
  # the state key would change (live/local/.terragrunt-stack/cc-tunnel instead
  # of live/local/cc-tunnel) and Terraform would treat every resource as new.
  #
  # Dropping the generated ".terragrunt-stack" segment yields the same path the
  # classic (non-stack) layout used, so terraform2 adopts the EXISTING state
  # with no state moves and `terragrunt plan` shows no changes:
  #     live/local/.terragrunt-stack/cc-tunnel        -> live/local/cc-tunnel
  #     prepare/local/.terragrunt-stack/terraform_sa  -> prepare/local/terraform_sa
  # For a non-generated config it is a no-op.
  #
  # NB: Terragrunt emits doubled slashes around the generated directory, e.g.
  # "live/local//.terragrunt-stack//init", so we split on "/" and drop both the
  # empty segments AND the ".terragrunt-stack" segment before re-joining — a
  # naive replace("/.terragrunt-stack/", "/") would leave "live/local///init"
  # and silently point the GCS backend at a brand-new (empty) state key.
  path_parts   = [for p in split("/", path_relative_to_include()) : p if p != "" && p != ".terragrunt-stack"]
  logical_path = join("/", local.path_parts)

  # Layout is "<category>/<env>/<unit>" (category = live | prepare), so env is
  # the 2nd path component. Falls back to "" when the path is not a known
  # environment (e.g. when `terragrunt hcl validate` parses a catalog unit in
  # place). Every env-dependent value below then degrades to an empty/known
  # placeholder, so validation stays happy while real runs — where env resolves
  # to "local" — produce byte-for-byte the same config as the classic layout.
  # An empty sentinel (not null) is required because these values are
  # interpolated into the generate blocks' string templates, and interpolating
  # null raises "Invalid template interpolation value".
  env_candidate = length(local.path_parts) >= 2 ? local.path_parts[1] : ""
  env           = contains(local.possible_envs, local.env_candidate) ? local.env_candidate : ""

  # Base directory of the shared Terraform modules. During the migration the
  # modules still live under terraform/modules; the final cutover step moves
  # them under terraform2/modules and ONLY this single line changes.
  modules_base_dir = "${get_repo_root()}/terraform/modules"

  developer_principals = lookup({
    local = ["user:polleninjp@gmail.com", ]
  }, local.env, [])

  gcp_project_id = lookup({
    local = "cc-tunnel-local"
  }, local.env, "")

  terraform_runner_sa_email = lookup({
    local = "local-guhp-tf-runner@cc-tunnel-local.iam.gserviceaccount.com"
  }, local.env, "")

  tfstate_bucket_name = lookup({
    local = "local-gsaq-tfstate"
  }, local.env, "")

  # NOTE:
  # Define values for each unit used in such like 'generate' block.
  each_unit_vars = try(
    read_terragrunt_config("${get_terragrunt_dir()}/vars_for_root.hcl"),
    { locals = {} }
  )
  provider_default_region = lookup(local.each_unit_vars.locals, "provider_default_region", "us-central1")
  provider_default_labels = merge(
    {
      env        = local.env
      managed_by = "terraform"
    },
    lookup(local.each_unit_vars.locals, "provider_default_labels", {}),
  )
}

generate "terraform" {
  path      = "autogen_terraform.tf"
  if_exists = "overwrite_terragrunt"

  contents = <<EOF
terraform {
  required_version = "~> 1.0"

  required_providers {
    time = {
      source  = "hashicorp/time"
      version = "~> 0.13"
    }
    google = {
      source  = "hashicorp/google"
      version = "~> 7.29.0"
    }
    google-beta = {
      source  = "hashicorp/google-beta"
      version = "~> 7.29.0"
    }
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 5"
    }
  }

  %{if local.tfstate_bucket_name != ""~}
  backend "gcs" {
    bucket = "${local.tfstate_bucket_name}"
    prefix = "${local.logical_path}"
  }
  %{endif~}
}
EOF
}

generate "provider" {
  path      = "autogen_providers.tf"
  if_exists = "overwrite_terragrunt"

  contents = <<EOF
provider "google" {
  project        = "${local.gcp_project_id}"
  region         = "${local.provider_default_region}"
  default_labels = ${jsonencode(local.provider_default_labels)}

  %{if local.terraform_runner_sa_email != ""
  && local.logical_path != "prepare/${local.env}/terraform_sa"~}
  impersonate_service_account = "${local.terraform_runner_sa_email}"
  %{endif~}
}

provider "google-beta" {
  project        = "${local.gcp_project_id}"
  region         = "${local.provider_default_region}"
  default_labels = ${jsonencode(local.provider_default_labels)}

  %{if local.terraform_runner_sa_email != ""
&& local.logical_path != "prepare/${local.env}/terraform_sa"~}
  impersonate_service_account = "${local.terraform_runner_sa_email}"
  %{endif~}
}

# API token は環境変数 CLOUDFLARE_API_TOKEN から取得
provider "cloudflare" {}
EOF
}

generate "outputs" {
  # NOTE: By setting at least one output in each terragrunt unit,
  #       it becomes easier to define dependencies.
  path      = "autogen_outputs.tf"
  if_exists = "overwrite_terragrunt"

  contents = <<EOF
output "done" {
  description = "Flag indicating that this has been executed at least once"
  value = true
}
EOF
}
