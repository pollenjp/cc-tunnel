resource "google_compute_firewall" "cc_tunnel_container_manager" {
  name    = "cc-tunnel-container-manager"
  network = google_compute_network.cc_tunnel.id

  allow {
    protocol = "tcp"
    ports = [
      # container-manager HTTP API (lifecycle endpoints).
      tostring(var.container_manager_port),
      # cc-remote-agent host port range (one published port per agent
      # container, allocated from this range by cc-tunnel). cc-tunnel
      # polls these directly for the agent's /auth/status, so the VPC
      # firewall must permit the whole range.
      "${var.cc_remote_agent_host_port_start}-${var.cc_remote_agent_host_port_end}",
    ]
  }

  # Allow only from the VPC Connector subnet (cc-tunnel Cloud Run egress)
  # to the container-manager API and to each cc-remote-agent container's
  # published host port. dockerd itself is bound to the Unix socket and is
  # not network-reachable.
  source_ranges = [var.vpc_connector_subnet_cidr]
  target_tags   = ["cc-tunnel-agent"]

  description = "Allow container-manager and cc-remote-agent TCP from cc-tunnel VPC Connector only"
}

# Optional SSH access to cc-remote-agent VMs via IAP TCP forwarding only.
# Enabled by `var.enable_ssh_debug`; default is off. The source range is the
# fixed Google-owned IAP block (35.235.240.0/20), so the VM's external IP
# alone cannot be SSH'd into directly — the caller must come through IAP and
# therefore must hold roles/iap.tunnelResourceAccessor on the project (or
# the specific instance). Pair with OS Login or metadata SSH keys for auth.
#
# Reference: https://cloud.google.com/iap/docs/using-tcp-forwarding
resource "google_compute_firewall" "cc_tunnel_iap_ssh" {
  count = var.enable_ssh_debug ? 1 : 0

  name    = "cc-tunnel-iap-ssh"
  network = google_compute_network.cc_tunnel.id

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }

  source_ranges = ["35.235.240.0/20"]
  target_tags   = ["cc-tunnel-agent"]

  description = "Allow SSH (TCP/22) from IAP TCP forwarding range for debugging"
}
