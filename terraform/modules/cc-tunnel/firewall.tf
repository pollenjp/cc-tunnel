resource "google_compute_firewall" "cc_tunnel_container_manager" {
  name    = "cc-tunnel-container-manager"
  network = google_compute_network.cc_tunnel.id

  allow {
    protocol = "tcp"
    ports    = [tostring(var.container_manager_port)]
  }

  # Allow only from the VPC Connector subnet (cc-tunnel Cloud Run egress)
  # to the container-manager API on each VM. dockerd itself is bound to
  # the Unix socket and is not network-reachable.
  source_ranges = [var.vpc_connector_subnet_cidr]
  target_tags   = ["cc-tunnel-agent"]

  description = "Allow container-manager TCP from cc-tunnel VPC Connector only"
}
