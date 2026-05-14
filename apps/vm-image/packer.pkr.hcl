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

variable "container_manager_image" {
  type        = string
  description = "Fully-qualified container-manager image reference (e.g. <region>-docker.pkg.dev/<project>/<repo>/container-manager:latest). The image is loaded from the tarball below; no pull is performed at VM runtime."
}

variable "container_manager_image_tar" {
  type        = string
  description = "Path on the Packer host (relative to the working directory) of the container-manager image saved via 'docker save'. Uploaded to the VM and loaded with 'docker load'."
  default     = "./container-manager.tar"
}

variable "container_manager_port" {
  type        = number
  default     = 9090
  description = "Host port that container-manager listens on."
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

  # 1. Install Docker (no TCP listener; Unix socket only — only the
  #    container-manager container talks to dockerd via the bind-mounted
  #    socket). Configure dockerd to use the gcplogs log driver so every
  #    container's stdout/stderr is shipped to Cloud Logging directly. Also
  #    install the Google Cloud Ops Agent for systemd/kernel/journald logs.
  #    See: adr/2026-05/.../gce_logging_strategy.md
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
      "mkdir -p /etc/docker",
      "cat > /etc/docker/daemon.json <<'EOF'",
      "{",
      "  \"log-driver\": \"gcplogs\",",
      "  \"log-opts\": {",
      "    \"max-size\": \"10m\",",
      "    \"max-file\": \"3\"",
      "  }",
      "}",
      "EOF",
      "systemctl enable docker",
      "curl -sSO https://dl.google.com/cloudagents/add-google-cloud-ops-agent-repo.sh",
      "bash add-google-cloud-ops-agent-repo.sh --also-install",
      "rm -f add-google-cloud-ops-agent-repo.sh",
    ]
  }

  # 2. Upload the container-manager image tarball pre-built by the upstream
  #    Cloud Build step.
  provisioner "file" {
    source      = var.container_manager_image_tar
    destination = "/tmp/container-manager.tar"
  }

  # 3. Load the image and install a systemd unit that runs container-manager
  #    in --network=bridge with a port mapping and the docker socket
  #    bind-mounted. container-manager is the only client of dockerd.
  #
  #    Values are interpolated at Packer build time (HCL ${...}) and the
  #    heredoc is quoted (<<'EOF') so the runtime shell does not try to
  #    re-expand anything. We deliberately do not rely on Packer's
  #    environment_vars here because the overridden execute_command would
  #    need {{ .Vars }} to propagate them — direct interpolation is simpler.
  provisioner "shell" {
    execute_command = "sudo -E bash '{{ .Path }}'"
    inline_shebang  = "/bin/bash -euo pipefail"
    inline = [
      "systemctl start docker",
      "docker load -i /tmp/container-manager.tar",
      "rm -f /tmp/container-manager.tar",
      "cat > /etc/systemd/system/container-manager.service <<'EOF'",
      "[Unit]",
      "Description=cc-tunnel container-manager",
      "After=docker.service",
      "Requires=docker.service",
      "",
      "[Service]",
      "Type=simple",
      "Restart=always",
      "RestartSec=5",
      "ExecStartPre=-/usr/bin/docker rm -f container-manager",
      "ExecStart=/usr/bin/docker run --rm --name container-manager --network=bridge -p ${var.container_manager_port}:9090 -v /var/run/docker.sock:/var/run/docker.sock --log-driver=gcplogs --log-opt labels=component --label component=container-manager ${var.container_manager_image}",
      "ExecStop=/usr/bin/docker stop container-manager",
      "",
      "[Install]",
      "WantedBy=multi-user.target",
      "EOF",
      "systemctl daemon-reload",
      "systemctl enable container-manager.service",
      "apt-get clean",
      "rm -rf /var/lib/apt/lists/*",
    ]
  }
}
