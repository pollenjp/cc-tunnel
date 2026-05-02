include "root" {
  path   = find_in_parent_folders("root.hcl")
  expose = true
}

dependency "cc_tunnel" {
  config_path = "./../cc-tunnel"
}

terraform {
  source = "./../../../modules//cloudflare_dns"
}

inputs = {
  # Cloudflare Zone ID for pollenjp.com
  # Dashboard > pollenjp.com の Overview ページ右下から取得
  cloudflare_zone_id = get_env("CLOUDFLARE_ZONE_ID")

  record_name  = "cctunnel.pollenjp.com"
  record_type  = "A"
  record_value = "${dependency.cc_tunnel.outputs.lb_ip}"
  ttl          = 1
  proxied      = false
  comment      = "cc-tunnel LB (managed by terraform)"
}
