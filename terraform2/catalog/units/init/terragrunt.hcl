include "root" {
  path   = find_in_parent_folders("root.hcl")
  expose = true
}

terraform {
  source = "${include.root.locals.modules_base_dir}//init_project"
}

inputs = {
  project_id = include.root.locals.gcp_project_id
}
