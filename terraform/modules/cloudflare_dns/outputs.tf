output "record_id" {
  value       = cloudflare_dns_record.this.id
  description = "Cloudflare DNS record ID"
}

output "record_hostname" {
  value       = cloudflare_dns_record.this.name
  description = "DNS record hostname (FQDN)"
}

output "record_value" {
  value       = cloudflare_dns_record.this.content
  description = "DNS record value"
}
