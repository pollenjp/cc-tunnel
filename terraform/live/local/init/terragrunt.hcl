include "root" {
  path = find_in_parent_folders("root.hcl")
  expose = true
}

terraform {
  source = "../../../modules//init_project"
}

inputs = {
  project_id = "${include.root.locals.gcp_project_id}"
}
