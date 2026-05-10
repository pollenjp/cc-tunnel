# Project-scoped IAP resources for the cc-tunnel External HTTPS LB.
#
# 重要な前提:
#   google_iap_brand / google_iap_client はいずれも 2025-01-22 に deprecate され、
#   裏側の "IAP OAuth Admin APIs" は 2026-03-19 に shutdown 済み。
#   そのため OAuth brand と OAuth client はいずれも GCP Console で手動作成する。
#
#   Console 手順:
#     1. APIs & Services > OAuth consent screen を開き User type を選択。
#        (組織配下なら Internal、個人プロジェクトなど組織なしの場合は External)
#        Application title / Support email を設定して作成。
#        External の場合は Test users にアクセス許可するアカウントを追加するか、
#        Publishing status を In production に切り替える必要がある点に注意。
#     2. APIs & Services > Credentials > Create credentials > OAuth client ID
#        を選び、Application type=Web application で作成。
#        Authorized redirect URIs に IAP の callback URL を追加:
#          https://iap.googleapis.com/v1/oauth/clientIds/<CLIENT_ID>:handleRedirect
#        (CLIENT_ID は作成直後にダイアログ表示されるので、控えてからこの URI を
#         追記する 2 段階の流れになる)
#     3. 作成された Client ID / Client secret を控え、IAP_OAUTH_CLIENT_ID と
#        IAP_OAUTH_CLIENT_SECRET 環境変数に設定して terragrunt apply する。
#
# このモジュールは Console 作成済みの OAuth credentials を入力として受け取り、
# 他モジュール (cc-tunnel) が参照しやすいよう output として再エクスポートするだけ。
# 資格情報は terraform state に格納される (sensitive output 扱い)。
