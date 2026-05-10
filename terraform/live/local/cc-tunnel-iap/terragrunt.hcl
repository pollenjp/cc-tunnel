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
  # この unit は plan / apply / hcl validate のいずれでも両 env var が必須。
  # 未設定だと modules/cc-tunnel-iap/variables.tf の validation で
  # 即時エラーになる (空文字禁止 + 形式チェック)。
  oauth_client_id     = get_env("IAP_OAUTH_CLIENT_ID", "")
  oauth_client_secret = get_env("IAP_OAUTH_CLIENT_SECRET", "")
}
