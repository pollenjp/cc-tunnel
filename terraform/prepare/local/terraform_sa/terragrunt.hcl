include "root" {
  path = find_in_parent_folders("root.hcl")
  expose = true
}

terraform {
  source = "../../../modules//prepare_terraform_sa"
}

inputs = {
  project_id = "${include.root.locals.gcp_project_id}"
  deploy_env = "${include.root.locals.env}"
  principals = include.root.locals.developer_principals
}
