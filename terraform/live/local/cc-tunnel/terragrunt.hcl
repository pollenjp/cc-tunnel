include "root" {
  path   = find_in_parent_folders("root.hcl")
  expose = true
}

dependency "artifact_registry" {
  config_path = "./../artifact_registry/"
}

terraform {
  source = "./../../../modules//cc-tunnel"
}

inputs = {
  project_id                            = "${include.root.locals.gcp_project_id}"
  artifact_registry_repository_location = "${dependency.artifact_registry.outputs.artifact_registry_repository_location}"
  artifact_registry_repository_name     = "${dependency.artifact_registry.outputs.artifact_registry_repository_name}"
  terraform_runner_sa_email             = "${include.root.locals.terraform_runner_sa_email}"
  deploy_env                            = "${include.root.locals.env}"
}
