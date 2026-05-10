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
  project_id                = "${include.root.locals.gcp_project_id}"
  deploy_env                = "${include.root.locals.env}"
  terraform_runner_sa_email = "${include.root.locals.terraform_runner_sa_email}"

  # google_iap_brand は deprecated (2025-01-22) で 2026-03-19 に裏側 API が
  # shutdown 済みのため、IAP の OAuth brand は GCP Console (APIs & Services >
  # OAuth consent screen) で手動作成する必要がある。
  # 個人プロジェクト (組織なし) では User type=External を選択する
  # (Internal は Google Workspace 組織配下でないと選べない)。
  # 作成後、以下のコマンドで name (projects/<project_number>/brands/<brand_id>)
  # を取得し、IAP_BRAND_NAME 環境変数に設定:
  #   gcloud iap oauth-brands list --project=<PROJECT> --format='value(name)'
  # 既定値 "" は terragrunt hcl validate (環境変数なし) を通すため。
  # 実値必須は modules/cc-tunnel-iap/variables.tf brand_name の validation で
  # plan/apply 時に強制する。
  brand_name = get_env("IAP_BRAND_NAME", "")
}
