locals {
  product_name = "cc-tunnel"
  possible_envs = [
    "local",
  ]

  # Check the parent or grandparent directory name of the directory where terragrunt.hcl is located,
  # and set the one that matches any of the environment names as 'env'.
  env = (
    # pattern 1: live/dev/sample1/terragrunt.hcl
    #                 ^^^
    contains(local.possible_envs, basename(dirname(get_terragrunt_dir())))
    ? basename(dirname(get_terragrunt_dir()))
    : (
      # pattern 2: live/dev/sample1/app/terragrunt.hcl
      #                 ^^^
      contains(local.possible_envs, basename(dirname(dirname(get_terragrunt_dir()))))
      ? basename(dirname(dirname(get_terragrunt_dir())))
      : null
    )
  )

  developer_principals = {
    local = ["user:polleninjp@gmail.com", ],
  }[local.env]

  gcp_project_id = {
    local = "cc-tunnel-local"
  }[local.env]

  terraform_runner_sa_email = {
    local = "local-guhp-tf-runner@cc-tunnel-local.iam.gserviceaccount.com"
  }[local.env]

  tfstate_bucket_name = {
    local = "local-gsaq-tfstate"
  }[local.env]

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
    external = {
      source  = "hashicorp/external"
      version = "~> 2.3"
    }
  }

  %{if local.tfstate_bucket_name != null~}
  backend "gcs" {
    bucket = "${local.tfstate_bucket_name}"
    prefix = "${path_relative_to_include()}"
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

  %{if local.terraform_runner_sa_email != null
  && path_relative_to_include() != "prepare/${local.env}/terraform_sa"~}
  impersonate_service_account = "${local.terraform_runner_sa_email}"
  %{endif~}
}

provider "google-beta" {
  project        = "${local.gcp_project_id}"
  region         = "${local.provider_default_region}"
  default_labels = ${jsonencode(local.provider_default_labels)}

  %{if local.terraform_runner_sa_email != null
&& path_relative_to_include() != "prepare/${local.env}/terraform_sa"~}
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
