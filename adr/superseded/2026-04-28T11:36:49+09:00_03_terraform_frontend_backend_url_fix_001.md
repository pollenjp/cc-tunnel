# subtask_terraform_frontend_invoker_001 変更ログ

## 変更内容

### 1. terraform/live/local/cc-tunnel/terragrunt.hcl

```diff
- enable_public_access          = true  # Phase 1: cc-tunnel を allUsers 公開（AppAuth で保護）
+ enable_public_access          = false  # 候補X: cc-tunnel 非公開、fe_runtime_sa 経由のみ invoke 許可
```

frontend_enable_public_access = true は維持（ブラウザから frontend に到達するため）。

### 2. terraform/modules/cc-tunnel/frontend.tf

① IAM member リソース追加（fe_public_access の後）:
```hcl
# fe_runtime_sa が cc-tunnel API を invoke できるように
resource "google_cloud_run_v2_service_iam_member" "fe_to_cc_tunnel_invoker" {
  location = google_cloud_run_v2_service.cloud_run.location
  name     = google_cloud_run_v2_service.cloud_run.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.fe_runtime_sa.email}"
}
```

② fe_cloud_run の containers.env に CC_TUNNEL_AUDIENCE を追加:
```hcl
env {
  name  = "CC_TUNNEL_AUDIENCE"
  value = google_cloud_run_v2_service.cloud_run.uri
}
```

③ fe_cloud_run の depends_on 追加:
```hcl
depends_on = [
  google_cloud_run_v2_service_iam_member.fe_to_cc_tunnel_invoker,
]
```

## CC_TUNNEL_AUDIENCE の設定方針

- audience は cc-tunnel Cloud Run の URI（`google_cloud_run_v2_service.cloud_run.uri`）
- frontend のコンテナが GCE metadata server から ID token を取得する際に使用
- token のaudienceを cc-tunnel URI に設定することで、cc-tunnel への認証リクエストが有効になる

## terraform validate 結果

`terraform fmt` → PASS（frontend.tf フォーマット済み）

`terraform validate` → SKIP（providers 未インストール: `terraform init` 未実行のため）
- HCL 構文は terraform fmt が通っており有効
- エラーは provider binary 不足のみ（コード上の問題なし）

## CRLF 確認

- terragrunt.hcl: LF only OK
- frontend.tf: LF only OK

## 殿の apply + 動作確認手順

```bash
cd ~/ghq/github.com/pollenjp/cc-tunnel/terraform/live/local/cc-tunnel

# 1. plan で差分確認
terragrunt plan

# 2. 問題なければ apply
terragrunt apply

# 3. 動作確認
# - cc-tunnel が非公開になっていること（allUsers invoke 不可）
# - frontend から cc-tunnel への API 呼び出しが成功すること（fe_runtime_sa 経由）
# - frontend の CC_TUNNEL_AUDIENCE env が設定されていること
```
