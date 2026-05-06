// Throwaway environment for verifying the metadata-server / AR-pull
// reachability assumptions in
// adr/2026-05/2026-05-06T11:00:45+09:00_01_container_manager_on_vm.md.
//
// Workflow (Cloud Build builds the Packer image — no local packer needed):
//   1. Push the branch containing apps/metadata-test-vm/ to GitHub
//      (the Cloud Build GitHub connection must already be authorized for
//      pollenjp/cc-tunnel; the existing cc-tunnel module sets that up).
//   2. Set `github_branch_name` below to the branch you just pushed
//      (use the feature branch while iterating; switch back to "main"
//      after merge), then run `terragrunt apply`.
//        -> creates the Cloud Build trigger + Packer builder SA, runs the
//           trigger once against `github_branch_name`, waits for the
//           image, then provisions the test VM.
//   3. Inspect the result (verify.sh writes to /var/log/metadata-test.log
//      and to the serial console):
//        $(terragrunt output -raw serial_log_command)
//      or SSH and re-run manually:
//        $(terragrunt output -raw ssh_command)
//        sudo /opt/metadata-test/verify.sh
//   4. Tear it all down:
//        terragrunt destroy

include "root" {
  path   = find_in_parent_folders("root.hcl")
  expose = true
}

dependency "artifact_registry" {
  config_path = "./../artifact_registry/"
}

terraform {
  source = "./../../../modules//metadata_test"
}

inputs = {
  project_id                = "${include.root.locals.gcp_project_id}"
  deploy_env                = "${include.root.locals.env}"
  terraform_runner_sa_email = "${include.root.locals.terraform_runner_sa_email}"

  artifact_registry_repository_location = "${dependency.artifact_registry.outputs.artifact_registry_repository_location}"
  artifact_registry_repository_name     = "${dependency.artifact_registry.outputs.artifact_registry_repository_name}"

  // Branch the Cloud Build trigger watches AND uses for the one-shot
  // run-on-apply. Set to the feature branch while iterating; switch back
  // to "main" after merge. github_owner / github_repo_name default to
  // pollenjp/cc-tunnel and rarely need to be overridden.
  github_branch_name = "claude/setup-metadata-test-env-HvAJT"
}
