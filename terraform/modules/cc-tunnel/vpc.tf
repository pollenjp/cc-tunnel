# Custom VPC for cc-tunnel. auto_create_subnetworks=false なので、
# 必要なサブネットだけ明示的に下で定義する。default VPC には依存しない。
resource "google_compute_network" "cc_tunnel" {
  name                    = var.network_name
  project                 = var.project_id
  auto_create_subnetworks = false
  routing_mode            = "REGIONAL"
  description             = "Custom VPC for cc-tunnel (Cloud Run Direct VPC egress + cc-remote-agent VMs)"
}

# Subnet used by Cloud Run Direct VPC egress so that the cc-tunnel service can
# reach the container-manager HTTP API on each GCE VM (by internal IP).
# The CIDR matches firewall.tf source_ranges and is sized /28 per Direct VPC
# egress requirement (minimum /28 in the same region as the Cloud Run service).
resource "google_compute_subnetwork" "cc_tunnel_egress" {
  name          = "cc-tunnel-egress"
  project       = var.project_id
  region        = local.cloud_run_location
  network       = google_compute_network.cc_tunnel.id
  ip_cidr_range = var.vpc_connector_subnet_cidr
  description   = "Direct VPC egress subnet for cc-tunnel Cloud Run -> GCE VM internal IPs"
}

locals {
  # Derive region from zone (e.g. "us-central1-a" -> "us-central1").
  gce_region = regex("^(.*)-[a-z]$", var.gce_zone)[0]
}

# Subnet for cc-remote-agent VMs with Private Google Access enabled, so VMs
# without external IPs can still reach Artifact Registry (`*.pkg.dev`) and
# other Google APIs to pull the cc-remote-agent image.
resource "google_compute_subnetwork" "cc_remote_agent_vm" {
  name                     = "cc-remote-agent-vm"
  project                  = var.project_id
  region                   = local.gce_region
  network                  = google_compute_network.cc_tunnel.id
  ip_cidr_range            = var.cc_remote_agent_subnet_cidr
  private_ip_google_access = true
  description              = "cc-remote-agent VM subnet (PGA-enabled for Artifact Registry pull)"
}
