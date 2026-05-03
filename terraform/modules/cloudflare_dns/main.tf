resource "cloudflare_dns_record" "this" {
  zone_id = var.cloudflare_zone_id
  name    = var.record_name
  type    = var.record_type
  content = var.record_value
  ttl     = var.ttl
  proxied = var.proxied
  comment = var.comment
}
