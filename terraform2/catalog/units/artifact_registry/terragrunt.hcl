include "root" {
  path   = find_in_parent_folders("root.hcl")
  expose = true
}

dependency "init" {
  config_path = "../init"
}

terraform {
  source = "${include.root.locals.modules_base_dir}//artifact_registry"
}

inputs = {
  project_id = include.root.locals.gcp_project_id
  location   = include.root.locals.provider_default_region
}
