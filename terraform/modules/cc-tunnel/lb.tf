# Disabled to save cost. Re-enable by removing the surrounding block comment.
/*
locals {
  lb_name_prefix = "${var.deploy_env}-${random_string.unique_id.result}-lb"
  lb_fqdn        = var.lb_fqdn
}

# Reserved global IP（Global LB）
resource "google_compute_global_address" "lb_ip" {
  name = "${local.lb_name_prefix}-ip"
}

# Serverless NEG: cc-tunnel
resource "google_compute_region_network_endpoint_group" "cc_tunnel_neg" {
  name                  = "${local.lb_name_prefix}-cct-neg"
  region                = local.cloud_run_location
  network_endpoint_type = "SERVERLESS"
  cloud_run {
    service = google_cloud_run_v2_service.cloud_run.name
  }
}

# Serverless NEG: frontend
resource "google_compute_region_network_endpoint_group" "frontend_neg" {
  name                  = "${local.lb_name_prefix}-fe-neg"
  region                = local.cloud_run_location
  network_endpoint_type = "SERVERLESS"
  cloud_run {
    service = google_cloud_run_v2_service.fe_cloud_run.name
  }
}

# Backend service: cc-tunnel
resource "google_compute_backend_service" "cc_tunnel_backend" {
  name                  = "${local.lb_name_prefix}-cct-backend"
  protocol              = "HTTP"
  load_balancing_scheme = "EXTERNAL_MANAGED"
  backend {
    group = google_compute_region_network_endpoint_group.cc_tunnel_neg.id
  }
  log_config { enable = true }
}

# Backend service: frontend
resource "google_compute_backend_service" "frontend_backend" {
  name                  = "${local.lb_name_prefix}-fe-backend"
  protocol              = "HTTP"
  load_balancing_scheme = "EXTERNAL_MANAGED"
  backend {
    group = google_compute_region_network_endpoint_group.frontend_neg.id
  }
  log_config { enable = true }
}

# URL map: /api/* → cc-tunnel（url_rewrite で /api strip）、default → frontend
resource "google_compute_url_map" "lb_url_map" {
  name            = "${local.lb_name_prefix}-url-map"
  default_service = google_compute_backend_service.frontend_backend.id

  host_rule {
    hosts        = [local.lb_fqdn]
    path_matcher = "main"
  }

  path_matcher {
    name            = "main"
    default_service = google_compute_backend_service.frontend_backend.id

    path_rule {
      paths   = ["/api", "/api/*"]
      service = google_compute_backend_service.cc_tunnel_backend.id
      route_action {
        url_rewrite {
          path_prefix_rewrite = "/"
        }
      }
    }
  }
}

# Google-managed SSL certificate（DNS 認証）
resource "google_compute_managed_ssl_certificate" "lb_cert" {
  name = "${local.lb_name_prefix}-cert"
  managed {
    domains = [local.lb_fqdn]
  }
}

# Target HTTPS proxy
resource "google_compute_target_https_proxy" "lb_https_proxy" {
  name             = "${local.lb_name_prefix}-https-proxy"
  url_map          = google_compute_url_map.lb_url_map.id
  ssl_certificates = [google_compute_managed_ssl_certificate.lb_cert.id]
}

# Forwarding rule（HTTPS:443）
resource "google_compute_global_forwarding_rule" "lb_https_forwarding_rule" {
  name                  = "${local.lb_name_prefix}-https-fwd"
  target                = google_compute_target_https_proxy.lb_https_proxy.id
  port_range            = "443"
  ip_address            = google_compute_global_address.lb_ip.id
  load_balancing_scheme = "EXTERNAL_MANAGED"
}
*/
