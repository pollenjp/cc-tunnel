include "root" {
  path   = find_in_parent_folders("root.hcl")
  expose = true
}

dependency "init" {
  config_path = "./../init"
}

terraform {
  source = "./../../../modules//cc-tunnel-iap"
}

inputs = {
  project_id = "${include.root.locals.gcp_project_id}"
  deploy_env = "${include.root.locals.env}"

  # OAuth ブランドはプロジェクトに 1 個まで。新規プロジェクトなら create_brand=true、
  # 既にコンソール等で作成済みなら create_brand=false にして existing_brand_name に
  # "projects/<project_number>/brands/<brand_id>" を渡す。
  create_brand        = false
  existing_brand_name = ""

  # create_brand=true のときに使う OAuth 同意画面の表示
  application_title = "cc-tunnel"
  support_email     = ""
}
