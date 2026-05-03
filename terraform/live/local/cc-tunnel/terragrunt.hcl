include "root" {
  path   = find_in_parent_folders("root.hcl")
  expose = true
}

dependency "artifact_registry" {
  config_path = "./../artifact_registry/"
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

  enable_public_access          = true # LB 経由のみだが allUsers invoker 必須（ingress=INTERNAL_LOAD_BALANCER で .run.app 直接アクセスはブロック済）
  frontend_enable_public_access = true # LB 経由のみだが allUsers invoker 必須（ingress=INTERNAL_LOAD_BALANCER で .run.app 直接アクセスはブロック済）

  lb_fqdn = "cctunnel.pollenjp.com"

  vpc_connector_subnet_cidr = "10.8.0.0/28"

  # Cloudflare Zone ID for pollenjp.com
  # Dashboard > pollenjp.com の Overview ページ右下から取得
  # 既定値 "" は terragrunt hcl validate (環境変数なし) を通すため。
  # 実値必須は modules/cc-tunnel/variables.tf cloudflare_zone_id の validation で plan/apply 時に強制する。
  cloudflare_zone_id = get_env("CLOUDFLARE_ZONE_ID", "")
}
