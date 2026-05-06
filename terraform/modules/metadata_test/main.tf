// Throwaway environment that verifies the two assumptions in
// adr/2026-05/2026-05-06T11:00:45+09:00_01_container_manager_on_vm.md:
//
//   1) A container on Docker's bridge network can reach the GCE metadata
//      server.
//   2) The VM Service Account's access token can pull from Artifact Registry.
//
// The VM is built from the dedicated `metadata-test-vm` image family produced
// by apps/metadata-test-vm/packer.pkr.hcl. /opt/metadata-test/verify.sh is
// baked into the image; the startup-script below just runs it once at boot
// and tees the output to /var/log/metadata-test.log AND the serial console so
// it shows up in `gcloud compute instances get-serial-port-output`.

locals {
  name_suffix = "-${random_string.unique_id.result}-mdtest"
  resource_base = "${substr(var.deploy_env, 0, 30 - length(local.name_suffix))}${local.name_suffix}"

  registry_host = "${var.artifact_registry_repository_location}-docker.pkg.dev"
  fqim          = "${local.registry_host}/${var.project_id}/${var.artifact_registry_repository_name}/${var.cc_remote_agent_image_name}:${var.cc_remote_agent_image_tag}"

  vm_image_url = "projects/${var.project_id}/global/images/family/${var.image_family}"

  startup_script = <<-EOT
    #!/bin/bash
    set -uo pipefail
    LOG=/var/log/metadata-test.log
    : >"$LOG"
    {
      echo "=== metadata-test verification start: $(date -Iseconds) ==="
      REGION="${var.artifact_registry_repository_location}" \
      PROJECT="${var.project_id}" \
      REPO="${var.artifact_registry_repository_name}" \
      IMAGE="${var.cc_remote_agent_image_name}" \
      TAG="${var.cc_remote_agent_image_tag}" \
        /opt/metadata-test/verify.sh
      rc=$?
      echo "=== metadata-test verification exit=$rc: $(date -Iseconds) ==="
      exit $rc
    } 2>&1 | tee -a "$LOG" >/dev/console
  EOT
}

resource "random_string" "unique_id" {
  length  = 4
  special = false
  upper   = false
  lower   = true
  numeric = false
}

resource "google_service_account" "vm_sa" {
  account_id   = local.resource_base
  display_name = "metadata-test VM SA (temporary)"
  description  = "Used only for the metadata server / AR pull reachability check"
}

resource "google_artifact_registry_repository_iam_member" "vm_sa_ar_reader" {
  location   = var.artifact_registry_repository_location
  repository = var.artifact_registry_repository_name
  role       = "roles/artifactregistry.reader"
  member     = "serviceAccount:${google_service_account.vm_sa.email}"
}

// IAM 反映待ち。新規 SA を作って即 VM をプロビジョンするとトークンに
// AR reader が反映されておらず docker pull が 403 になることがある。
resource "time_sleep" "wait_iam_propagation" {
  depends_on = [google_artifact_registry_repository_iam_member.vm_sa_ar_reader]
  create_duration = "60s"
  triggers = {
    binding = google_artifact_registry_repository_iam_member.vm_sa_ar_reader.id
  }
}

resource "google_compute_instance" "vm" {
  depends_on = [time_sleep.wait_iam_propagation]

  name         = local.resource_base
  machine_type = var.machine_type
  zone         = var.zone

  boot_disk {
    initialize_params {
      image = local.vm_image_url
      size  = 20
    }
  }

  network_interface {
    network    = var.network_name
    subnetwork = var.subnetwork_name
    // External IP so docker can pull the probe image (curlimages/curl) from
    // Docker Hub. Production VMs do not need this; this is a throwaway.
    access_config {}
  }

  service_account {
    email  = google_service_account.vm_sa.email
    scopes = ["cloud-platform"]
  }

  metadata = {
    startup-script             = local.startup_script
    serial-port-logging-enable = "TRUE"
  }

  labels = {
    purpose = "metadata-test"
    env     = var.deploy_env
  }

  // 検証が終わったら terragrunt destroy で消すため allow_stopping_for_update
  // のような細かい運用は不要。
}
