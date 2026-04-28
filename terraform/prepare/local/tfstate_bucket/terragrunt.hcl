include "root" {
  path   = find_in_parent_folders("root.hcl")
  expose = true
}

terraform {
  source = "../../../modules//prepare_tfstate_bucket"
}

inputs = {
  deploy_env = "${include.root.locals.env}"
}
