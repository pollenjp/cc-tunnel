include "root" {
  path   = find_in_parent_folders("root.hcl")
  expose = true
}

dependency "artifact_registry" {
  config_path = "./../artifact_registry/"
}

dependency "cc_tunnel_iap" {
  config_path = "./../cc-tunnel-iap/"

  mock_outputs = {
    oauth_client_id     = ""
    oauth_client_secret = ""
  }
  mock_outputs_allowed_terraform_commands = ["validate", "init", "plan"]
}

terraform {
  source = "./../../../modules//cc-tunnel"
}

inputs = {
  project_id                            = "${include.root.locals.gcp_project_id}"
  artifact_registry_repository_location = "${dependency.artifact_registry.outputs.artifact_registry_repository_location}"
  artifact_registry_repository_name     = "${dependency.artifact_registry.outputs.artifact_registry_repository_name}"
  terraform_runner_sa_email             = "${include.root.locals.terraform_runner_sa_email}"
  deploy_env                            = "${include.root.locals.env}"

  frontend_image_name     = "frontend"
  frontend_container_port = 8080

  # IAP P4SA に roles/run.invoker を bind 済み (modules/cc-tunnel/iap.tf) なので allUsers 撤去。
  # ingress=INTERNAL_LOAD_BALANCER と合わせて defense-in-depth で .run.app への直接アクセスを封じる。
  # 前提: iap_enabled = true で運用すること (false のままだと LB → Cloud Run の invoker 権限が無くなり 403)。
  enable_public_access          = false
  frontend_enable_public_access = false

  lb_fqdn = "cctunnel.pollenjp.com"

  vpc_connector_subnet_cidr = "10.8.0.0/26"

  # Cloudflare Zone ID for pollenjp.com
  # Dashboard > pollenjp.com の Overview ページ右下から取得
  # 既定値 "" は terragrunt hcl validate (環境変数なし) を通すため。
  # 実値必須は modules/cc-tunnel/variables.tf cloudflare_zone_id の validation で plan/apply 時に強制する。
  cloudflare_zone_id = get_env("CLOUDFLARE_ZONE_ID", "")

  # IAP: 有効化するには iap_enabled=true にして iap_allowed_members を埋める。
  # OAuth client_id/secret は cc-tunnel-iap unit から自動で渡る。
  iap_enabled             = false
  iap_oauth_client_id     = "${dependency.cc_tunnel_iap.outputs.oauth_client_id}"
  iap_oauth_client_secret = "${dependency.cc_tunnel_iap.outputs.oauth_client_secret}"
  iap_allowed_members     = []
}
