resource "google_compute_firewall" "cc_tunnel_docker_daemon" {
  name    = "cc-tunnel-docker-daemon"
  network = var.network_name

  allow {
    protocol = "tcp"
    ports    = ["2375"]
  }

  # Allow only from VPC Connector subnet; deny all other ingress.
  source_ranges = [var.vpc_connector_subnet_cidr]
  target_tags   = ["cc-tunnel-agent"]

  description = "Allow Docker daemon TCP from cc-tunnel VPC Connector only"
}
