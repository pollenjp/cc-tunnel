// Throwaway environment for verifying the metadata-server / AR-pull
// reachability assumptions in
// adr/2026-05/2026-05-06T11:00:45+09:00_01_container_manager_on_vm.md.
//
// Workflow:
//   1. Build the test image once (locally, ADC):
//        gcloud auth application-default login
//        cd apps/metadata-test-vm
//        packer init  packer.pkr.hcl
//        packer build -var=project_id=cc-tunnel-local \
//                     -var=image_name=metadata-test-vm-$(date +%s) \
//                     packer.pkr.hcl
//   2. Spin the VM:
//        cd terraform/live/local/metadata_test
//        terragrunt apply
//   3. Inspect the result (verify.sh writes to /var/log/metadata-test.log
//      and to the serial console):
//        terragrunt output serial_log_command   # then run it
//      or SSH and re-run manually:
//        terragrunt output ssh_command
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
  project_id = "${include.root.locals.gcp_project_id}"
  deploy_env = "${include.root.locals.env}"

  artifact_registry_repository_location = "${dependency.artifact_registry.outputs.artifact_registry_repository_location}"
  artifact_registry_repository_name     = "${dependency.artifact_registry.outputs.artifact_registry_repository_name}"
}
