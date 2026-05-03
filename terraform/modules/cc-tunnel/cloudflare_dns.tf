terraform {
  required_providers {
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 5.0"
    }
  }
}

# Provider config は環境変数 CLOUDFLARE_API_TOKEN から取得
provider "cloudflare" {}

resource "cloudflare_dns_record" "lb" {
  zone_id = var.cloudflare_zone_id
  name    = var.lb_fqdn
  type    = "A"
  content = google_compute_global_address.lb_ip.address
  ttl     = var.cloudflare_dns_ttl
  proxied = var.cloudflare_dns_proxied
  comment = var.cloudflare_dns_comment
}
