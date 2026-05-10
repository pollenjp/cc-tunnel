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
  # google_iap_brand / google_iap_client は deprecated (2025-01-22) で
  # 2026-03-19 に裏側 API が shutdown 済みのため、IAP の OAuth brand と
  # OAuth client は GCP Console で手動作成する必要がある。
  # 個人プロジェクト (組織なし) では User type=External を選択し、
  # Authorized redirect URIs に
  #   https://iap.googleapis.com/v1/oauth/clientIds/<CLIENT_ID>:handleRedirect
  # を設定する。詳細は modules/cc-tunnel-iap/main.tf のコメント参照。
  #
  # 作成後、Client ID / Client secret を以下の環境変数に設定して apply する:
  #   IAP_OAUTH_CLIENT_ID=<...>.apps.googleusercontent.com
  #   IAP_OAUTH_CLIENT_SECRET=<...>
  # 既定値 "" は terragrunt hcl validate (環境変数なし) を通すため。
  # 実値必須は modules/cc-tunnel-iap/variables.tf の validation で plan/apply
  # 時に強制する (ただし client_secret は空チェックなしで cc-tunnel 側に渡る)。
  oauth_client_id     = get_env("IAP_OAUTH_CLIENT_ID", "")
  oauth_client_secret = get_env("IAP_OAUTH_CLIENT_SECRET", "")
}
