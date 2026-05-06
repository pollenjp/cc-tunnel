// Standalone Packer image used ONLY for the metadata-server / Artifact Registry
// reachability check described in
// adr/2026-05/2026-05-06T11:00:45+09:00_01_container_manager_on_vm.md.
//
// The image installs Docker (Unix socket only — no TCP 2375 listener) and
// bakes the verification script at /opt/metadata-test/verify.sh.
// It is intentionally separate from the production cc-tunnel-vm image so the
// experiment cannot accidentally affect production VMs.
//
// Local usage:
//   gcloud auth application-default login
//   packer init  apps/metadata-test-vm/packer.pkr.hcl
//   packer build -var=project_id=cc-tunnel-local \
//                -var=image_name=metadata-test-vm-$(date +%s) \
//                apps/metadata-test-vm/packer.pkr.hcl

packer {
  required_plugins {
    googlecompute = {
      source  = "github.com/hashicorp/googlecompute"
      version = "~> 1.1"
    }
  }
}

variable "project_id" {
  type = string
}

variable "zone" {
  type    = string
  default = "us-central1-a"
}

variable "image_name" {
  type        = string
  description = "Output image name (must be unique per build)"
}

variable "image_family" {
  type    = string
  default = "metadata-test-vm"
}

variable "source_image_family" {
  type    = string
  default = "ubuntu-2404-lts-amd64"
}

variable "source_image_project" {
  type    = string
  default = "ubuntu-os-cloud"
}

variable "machine_type" {
  type    = string
  default = "e2-small"
}

source "googlecompute" "metadata_test_vm" {
  project_id              = var.project_id
  zone                    = var.zone
  source_image_family     = var.source_image_family
  source_image_project_id = [var.source_image_project]
  image_name              = var.image_name
  image_family            = var.image_family
  machine_type            = var.machine_type
  ssh_username            = "packer"
  disk_size               = 20
  image_labels = {
    managed-by = "cc-tunnel"
    builder    = "packer"
    purpose    = "metadata-test"
  }
}

build {
  name    = "metadata-test-vm"
  sources = ["source.googlecompute.metadata_test_vm"]

  provisioner "file" {
    source      = "verify.sh"
    destination = "/tmp/verify.sh"
  }

  provisioner "shell" {
    execute_command = "sudo -E bash '{{ .Path }}'"
    inline_shebang  = "/bin/bash -euo pipefail"
    inline = [
      "export DEBIAN_FRONTEND=noninteractive",
      "for i in 1 2 3 4 5; do apt-get update && break || sleep 5; done",
      "apt-get install -y --no-install-recommends ca-certificates curl gnupg jq",
      "install -m 0755 -d /etc/apt/keyrings",
      "curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg",
      "chmod a+r /etc/apt/keyrings/docker.gpg",
      "echo 'deb [arch=amd64 signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu noble stable' > /etc/apt/sources.list.d/docker.list",
      "for i in 1 2 3 4 5; do apt-get update && break || sleep 5; done",
      "apt-get install -y --no-install-recommends docker-ce docker-ce-cli containerd.io",
      // Docker daemon は Unix socket のみ。 TCP listener は意図的に有効化しない。
      "systemctl enable docker",
      "install -d -m 0755 /opt/metadata-test",
      "install -m 0755 /tmp/verify.sh /opt/metadata-test/verify.sh",
      "rm -f /tmp/verify.sh",
      "apt-get clean",
      "rm -rf /var/lib/apt/lists/*",
    ]
  }
}
