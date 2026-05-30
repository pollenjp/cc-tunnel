include "root" {
  path   = find_in_parent_folders("root.hcl")
  expose = true
}

terraform {
  source = "${include.root.locals.modules_base_dir}//prepare_tfstate_bucket"
}

inputs = {
  deploy_env = include.root.locals.env
}
