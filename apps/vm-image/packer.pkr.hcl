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
  default = "cc-tunnel-vm"
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

source "googlecompute" "cc_tunnel_vm" {
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
  }
}

build {
  name    = "cc-tunnel-vm"
  sources = ["source.googlecompute.cc_tunnel_vm"]

  provisioner "shell" {
    execute_command = "sudo -E bash '{{ .Path }}'"
    inline_shebang  = "/bin/bash -euo pipefail"
    inline = [
      "export DEBIAN_FRONTEND=noninteractive",
      "for i in 1 2 3 4 5; do apt-get update && break || sleep 5; done",
      "apt-get install -y --no-install-recommends ca-certificates curl gnupg",
      "install -m 0755 -d /etc/apt/keyrings",
      "curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg",
      "chmod a+r /etc/apt/keyrings/docker.gpg",
      "echo 'deb [arch=amd64 signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu noble stable' > /etc/apt/sources.list.d/docker.list",
      "for i in 1 2 3 4 5; do apt-get update && break || sleep 5; done",
      "apt-get install -y --no-install-recommends docker-ce docker-ce-cli containerd.io",
      "mkdir -p /etc/systemd/system/docker.service.d",
      "cat > /etc/systemd/system/docker.service.d/tcp.conf <<'EOF'",
      "[Service]",
      "ExecStart=",
      "ExecStart=/usr/bin/dockerd -H fd:// -H tcp://0.0.0.0:2375 --containerd=/run/containerd/containerd.sock",
      "EOF",
      "systemctl daemon-reload",
      "systemctl enable docker",
      "apt-get clean",
      "rm -rf /var/lib/apt/lists/*",
    ]
  }
}
