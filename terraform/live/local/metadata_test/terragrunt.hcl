// Throwaway environment for verifying the metadata-server / AR-pull
// reachability assumptions in
// adr/2026-05/2026-05-06T11:00:45+09:00_01_container_manager_on_vm.md.
//
// Workflow (Cloud Build builds the Packer image — no local packer needed):
//   1. Push the branch containing apps/metadata-test-vm/ to GitHub
//      (the Cloud Build GitHub connection must already be authorized for
//      pollenjp/cc-tunnel; the existing cc-tunnel module sets that up).
//   2. terragrunt apply
//        -> creates the Cloud Build trigger + Packer builder SA, runs the
//           trigger once against `packer_build_branch` (default "main";
//           set to the feature branch when applying before merge), waits
//           for the image, then provisions the test VM.
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

  // Override via TF_VAR_packer_build_branch when applying from a feature
  // branch before it is merged to main.
  // packer_build_branch = "claude/setup-metadata-test-env-HvAJT"
}
