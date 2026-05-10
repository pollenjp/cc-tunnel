# ADR: IAP for LB-fronted Cloud Run — adapt to OAuth Admin API shutdown

- Date: 2026-05-10
- Status: Accepted
- 関連 PR: (this branch) `claude/destroy-terraform-resources-m9Fle`
- 関連 Issue: #62 (defense-in-depth: drop allUsers Cloud Run invoker)
- 関連 docs: `docs/terraform-setup.md` (Phase 3: Identity-Aware Proxy 節)

## Context

Phase 2 で External Global HTTPS LB + serverless NEG 構成 (cctunnel.pollenjp.com) まで
実装し、cc-tunnel と frontend を単一ドメインに統合した。Phase 3 として LB の前段で
Google アカウント認証を強制したい — つまり Identity-Aware Proxy (IAP) の有効化。

ところが GCP 側で大きな前提変更があった:

- `google_iap_brand` resource は **2025-01-22 に deprecate**
- `google_iap_client` resource も同様に deprecate
- 裏側の **"IAP OAuth Admin APIs" は 2026-03-19 に permanently shut down**
- 新規プロジェクトでは API が使えず、既存プロジェクトでも resource が機能しない

これにより、当初想定していた「terraform で OAuth brand と OAuth client を declarative に
作成 → backend service に紐付け」というフローは取れなくなった。

加えて IAP が LB 経由で Cloud Run を invoke するには、IAP service agent
(`service-<PROJECT_NUMBER>@gcp-sa-iap.iam.gserviceaccount.com`) に
`roles/run.invoker` を付与する必要がある。これが無いとブラウザログイン後に
"The IAP service account is not provisioned" のエラーになる。

## Decision

1. **OAuth brand と OAuth client は GCP Console で手動作成する**
   - terraform からは作らない (作れない)
   - 個人プロジェクト (組織なし) では User type=External を選択
   - Authorized redirect URIs に `https://iap.googleapis.com/v1/oauth/clientIds/<CLIENT_ID>:handleRedirect` を設定

2. **`cc-tunnel-iap` モジュールを thin passthrough にする**
   - Console 作成済みの `client_id` / `client_secret` を入力として受け取り、そのまま output として再エクスポートする
   - 主要な変数:
     - `oauth_client_id` (validation: googleusercontent.com 形式)
     - `oauth_client_secret` (sensitive)
   - Live unit (`terraform/live/local/cc-tunnel-iap/terragrunt.hcl`) は
     `IAP_OAUTH_CLIENT_ID` / `IAP_OAUTH_CLIENT_SECRET` 環境変数から値を読む
   - cc-tunnel module 側の入力配線
     (`dependency.cc_tunnel_iap.outputs.oauth_client_id` 等) は変更不要

3. **IAP P4SA のプロビジョニングと Cloud Run invoker 付与は cc-tunnel module で行う**
   - `google_project_service_identity.iap` (google-beta) で P4SA を作成
   - `google_cloud_run_v2_service_iam_member` で両 Cloud Run service に
     `roles/run.invoker` を付与
   - すべて `var.iap_enabled` で gate し、有効化/無効化を 1 変数で制御

4. **IAP の認可は `google_iap_web_backend_service_iam_member` で backend service スコープに付与**
   - 許可ユーザは `var.iap_allowed_members` (例: `["user:foo@example.com"]`)
   - 必要ロール: `roles/iap.httpsResourceAccessor`
   - スコープを project レベル (`google_project_iam_member`) ではなく backend service にしたのは、将来同じ project で別の IAP リソース (App Engine など) を持つ可能性に備えて最小権限にするため

5. **Runner SA に `roles/iap.admin` を付与**
   - `prepare_terraform_sa` で管理
   - これが無いと IAP の IAM policy 操作で 403 になる

6. **Cloud Run の `allUsers` invoker は当面残す**
   - `ingress=INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER` で `.run.app` 直接アクセスは塞がっており、defense-in-depth 上は問題なし
   - 撤去 + IAP P4SA only にする作業は **Issue #62** で別途追跡 (順序を逆にすると全リクエスト 403 になるリスクがあるため)

## Alternatives considered

### A. `gcloud iap oauth-brands describe` で brand 存在を verify する

一度実装したが破棄。`gcloud iap oauth-brands` も同じ "IAP OAuth Admin APIs" を叩くため
2026-03-19 以降は失敗する。`hashicorp/external` provider 依存も増えるので削除した。

### B. Cloud Run 直接 IAP (LB を介さない)

Google ドキュメントの推奨パスは Cloud Run service レベルで IAP を直接有効化することだが、
本構成では single-domain 化のために LB 必須 (`/api/*` → cc-tunnel,  `/*` → frontend の path routing)。
LB をすでに持っているので、backend service レベルに IAP を被せる構成を採用した。

### C. project-scoped IAP IAM (`google_project_iam_member`)

`roles/iap.httpsResourceAccessor` を project 全体に付与すれば IAM 管理が単純化されるが、
将来 App Engine 等を IAP 配下に追加した際に巻き添えで認可されてしまうため不採用。
backend service スコープで個別管理する方が将来の拡張に対して安全。

### D. OAuth client_id / secret を Secret Manager 経由で読む

terraform state に secret 値がそのまま入る今の構成より一段安全になるが、
- 値の初期登録 (gcloud secrets versions add) を手動で行う追加手順が増える
- 個人プロジェクト規模では state の access control で十分
- 将来必要になれば cc-tunnel-iap モジュールに `data "google_secret_manager_secret_version"`
  を足すだけで切り替え可能

として今回は採用見送り。

## Consequences

### Positive

- API shutdown 後も IAP を terraform で運用できる (brand/client 以外は declarative)
- Console 手作業は OAuth brand と client の **初回作成 1 回のみ**。以降の `terragrunt apply` で
  iap_enabled / iap_allowed_members の切替は完結する
- IAP 有効化の一連の依存関係 (P4SA 作成 → Cloud Run invoker → backend service iap block →
  user accessor) が `var.iap_enabled` 一つで連動するので on/off が容易

### Negative / Trade-offs

- **Console 手作業が消えない**: brand と OAuth client は人間が作る必要がある。
  CI で apply を完結させたい場合に手動ステップが残る
- **OAuth Test users と `iap_allowed_members` の二重管理**: Publishing=Testing 状態の場合、
  Console の Test users と terraform の `iap_allowed_members` の両方にユーザを足す必要がある。
  2 箇所同期忘れがエラーの原因になりやすい (`docs/terraform-setup.md` のトラブルシュートに記載)
- **`oauth_client_secret` が terraform state に入る**: ローカルの GCS state 暗号化と
  IAM access control に依存する。team 規模が大きくなれば D 案 (Secret Manager) に移行する余地

### Follow-ups

- **Issue #62**: `enable_public_access=false` 化 + `allUsers` 撤去 (defense-in-depth)
- IAP P4SA に `roles/run.invoker` を付与済みなので、Issue #62 は invoker IAM の差し替えだけで完結する見込み
- `google_iap_client` も同じ deprecated API 群を使う。**resource 自体は本リポジトリではもう使っていない**ため影響なし、念のため記録
