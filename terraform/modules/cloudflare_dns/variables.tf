variable "cloudflare_zone_id" {
  type        = string
  description = "Cloudflare Zone ID for the DNS record (Cloudflare Dashboard > 該当 zone の Overview ページ右下から取得)"

  validation {
    condition     = length(var.cloudflare_zone_id) > 0
    error_message = "cloudflare_zone_id is required. Set the CLOUDFLARE_ZONE_ID environment variable."
  }
}

variable "record_name" {
  type        = string
  description = "DNS record name (FQDN). 例: cctunnel.pollenjp.com"
}

variable "record_value" {
  type        = string
  description = "DNS record value. A レコードの場合は IPv4 アドレス。"
}

variable "record_type" {
  type        = string
  description = "DNS record type"
  default     = "A"
}

variable "ttl" {
  type        = number
  description = "TTL (seconds). proxied=true の場合は 1 (Auto)"
  default     = 1
}

variable "proxied" {
  type        = bool
  description = "Cloudflare のプロキシ (orange cloud) を有効にするか"
  default     = false
}

variable "comment" {
  type        = string
  description = "Cloudflare DNS record の comment"
  default     = "Managed by Terraform"
}
