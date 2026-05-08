# Subnet used by Cloud Run Direct VPC egress so that the cc-tunnel service can
# reach the container-manager HTTP API on each GCE VM (by internal IP).
# The CIDR matches firewall.tf source_ranges and is sized /28 per Direct VPC
# egress requirement (minimum /28 in the same region as the Cloud Run service).
resource "google_compute_subnetwork" "cc_tunnel_egress" {
  name          = "cc-tunnel-egress"
  project       = var.project_id
  region        = local.cloud_run_location
  network       = var.network_name
  ip_cidr_range = var.vpc_connector_subnet_cidr
  description   = "Direct VPC egress subnet for cc-tunnel Cloud Run -> GCE VM internal IPs"
}
