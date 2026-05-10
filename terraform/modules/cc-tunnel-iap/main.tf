# Project-scoped IAP resources for the cc-tunnel External HTTPS LB.
#
# 重要な前提:
#   - google_iap_brand は 2025-01-22 deprecate / 2026-03-19 に裏側の
#     "IAP OAuth Admin APIs" が shutdown 済み。本モジュールは brand を
#     terraform からは作らず、GCP Console で作成済みの brand を入力として受け取る。
#     Console 手順:
#       APIs & Services > OAuth consent screen で User type=Internal を選択し
#       Application title / Support email を設定して作成。
#       作成後 `gcloud iap oauth-brands list --project=<PROJECT>` で
#       "projects/<PROJECT_NUMBER>/brands/<BRAND_ID>" 形式の name を取得。
#   - google_iap_client は同じ deprecated API 群を使うため将来停止する可能性が
#     ある。停止が確認されたら Console 手動で OAuth client を作り、client_id /
#     client_secret を直接入力 (e.g. Secret Manager 経由) する構成に切り替える。

# brand の存在確認。Console で作成し忘れ / brand_name 入力ミスを plan 段階で検出する。
# `gcloud iap oauth-brands describe` は IAP OAuth Admin API を叩くため API shutdown
# 後は失敗する。そのときは error.fail_open=true 相当の運用 (verification skip) に
# 切り替える必要があるが、現状は明示エラーで気付ける方を優先。
data "external" "iap_brand" {
  program = [
    "bash", "-c",
    <<-EOT
      set -euo pipefail
      brand_name='${var.brand_name}'
      project_id='${var.project_id}'
      if [[ -z "$${brand_name}" ]]; then
        echo "brand_name is empty. Set IAP_BRAND_NAME to projects/<project_number>/brands/<brand_id> after creating the OAuth brand in GCP Console." >&2
        exit 1
      fi
      impersonate_flag=""
      runner_sa='${var.terraform_runner_sa_email}'
      if [[ -n "$${runner_sa}" ]]; then
        impersonate_flag="--impersonate-service-account=$${runner_sa}"
      fi
      if ! out=$(gcloud iap oauth-brands describe "$${brand_name}" \
            --project="$${project_id}" \
            $impersonate_flag \
            --format=json 2>&1); then
        echo "Failed to describe IAP brand '$${brand_name}' in project '$${project_id}'." >&2
        echo "Make sure the OAuth brand has been created in GCP Console (APIs & Services > OAuth consent screen)" >&2
        echo "and that brand_name is in the form projects/<project_number>/brands/<brand_id>." >&2
        echo "$${out}" >&2
        exit 1
      fi
      echo "$${out}" | jq -c '{
        name: (.name // ""),
        application_title: (.applicationTitle // ""),
        support_email: (.supportEmail // ""),
        org_internal_only: ((.orgInternalOnly // false) | tostring)
      }'
    EOT
  ]
  query = {
    # query を変えると外部スクリプトが再評価される。
    # brand_name 変更時に都度確認するためここに含める。
    brand_name = var.brand_name
    project_id = var.project_id
  }
}

resource "google_iap_client" "client" {
  display_name = "${var.deploy_env}-iap-client"
  brand        = data.external.iap_brand.result.name
}
