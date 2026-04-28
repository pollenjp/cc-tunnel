# 変更ログ — subtask_terraform_lb_neg_docs_001 (Phase C)

- **cmd**: cmd_cctunnel_terraform_lb_neg_001
- **担当**: 足軽4号
- **日時**: 2026-04-28T04:27:14+00:00
- **対象**: docs/terraform-setup.md, docs/frontend.md

## 概要

Phase C: LB 構成（Global HTTPS LB + serverless NEG）への docs 更新。
候補X（OpenResty/Lua/ID token注入）の記述を Phase 2 LB 構成説明に完全置換。

## 変更サマリー

### docs/terraform-setup.md
- 「## Phase 1: frontend → cc-tunnel 認証戦略」セクション（候補X/OpenResty/Lua 記述）を削除
- 「## Phase 2: External Global HTTPS LB + serverless NEG 構成」セクションに置換
  - アーキテクチャ図（Browser → LB → Cloud Run）追記
  - Apply後手順（terragrunt apply → Cloudflare Aレコード → cert ACTIVE確認）追記
  - Terraform変数表（lb_fqdn）追記
  - 注意事項（cert待ち、ingress制限、ローカル開発への影響なし）追記
  - Phase 3 IAP移行時の差分（iapブロック追加のみ）追記

### docs/frontend.md
- 「### ID token 注入（候補X 採用）」サブセクション（OpenResty/Lua 記述）を削除
- 「### Phase 2: LB 経由配信」サブセクションに置換

## 品質確認

- 候補X / OpenResty / lua の記述: 残留なし ✓
- CRLF: 0行（LF only） ✓
- git操作: ゼロ ✓

## 変更 diff（参考）

```diff
diff --git a/docs/frontend.md b/docs/frontend.md
index 8fd0b88..e7a06f1 100644
--- a/docs/frontend.md
+++ b/docs/frontend.md
@@ -627,19 +627,13 @@ location /api/ {
 **CORS 不要の理由:** ブラウザから見ると frontend と API が同一オリジン（nginx が同一 Port で両方を提供）のため
 preflight が発生しない。
 
-### ID token 注入（候補X 採用）
+### Phase 2: LB 経由配信
 
-cc-tunnel が IAM invoker で保護されているため、frontend nginx は
-GCE metadata server から ID token を取得して Authorization header に注入する。
+frontend Cloud Run は Global HTTPS LB の serverless NEG 経由でアクセス。
+LB の default backend が frontend を配信するため、nginx の /api/ proxy_pass は使用されない。
+ローカル開発環境（compose.yaml）では引き続き nginx の /api/ proxy_pass が動作する。
 
-実装: OpenResty + Lua の `access_by_lua_block`
-- cache: `lua_shared_dict id_tokens`（1m、50分 TTL）
-- audience: env var `CC_TUNNEL_AUDIENCE` = cc-tunnel Cloud Run URI
-- cache miss 時に `http://metadata.google.internal` から取得
-
-トラブルシュート:
-- `502` → metadata server 到達不可（ローカル開発時は想定動作）
-- `401` from cc-tunnel → fe_runtime_sa に roles/run.invoker が付与されていない
-- SSE 接続が切断 → SSE location にも access_by_lua_block が必要（接続開始時に Authorization を注入）
+- ingress: `INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCING`（LB 経由のみ）
+- SPA ルーティング: nginx の `try_files $uri $uri/ /index.html` で対応（変更なし）
 
 
diff --git a/docs/terraform-setup.md b/docs/terraform-setup.md
index 53b8e4f..7cc1fb5 100644
--- a/docs/terraform-setup.md
+++ b/docs/terraform-setup.md
@@ -259,39 +259,46 @@ Artifact Registry push を管理する。
 
 `cc_remote_agent_image` output に cc-remote-agent の Artifact Registry イメージ URL が出力される。
 
-## Phase 1: frontend → cc-tunnel 認証戦略
+## Phase 2: External Global HTTPS LB + serverless NEG 構成
 
-Phase 1 では候補X（cc-tunnel 非公開 + IAM invoker + nginx ID token 注入）を採用:
-- cc-tunnel: enable_public_access=false、fe_runtime_sa のみ invoke 可（GCP IAM で遮断）
-- frontend: frontend_enable_public_access=true（ブラウザ到達用、AppAuth で UI 保護）
-- frontend nginx: OpenResty + Lua で metadata server から ID token 取得 → Authorization header に注入
-- 多層防御: GCP IAM invoker（ネットワーク層の侵入遮断）+ AppAuth（user-level セッション保護）
+Phase 2 では External Global HTTPS LB を採用し、cc-tunnel と frontend を
+単一ドメイン（cctunnel.pollenjp.com）に統合する:
+- LB の path matcher: /api/* → cc-tunnel backend（url_rewrite で /api prefix を削除）
+- LB の default: frontend backend
+- cc-tunnel/frontend ともに ingress=INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCING（LB 経由のみ）
+- SSL: Google-managed SSL certificate（DNS 認証、cctunnel.pollenjp.com）
 
-### ID token 注入フロー
+### アーキテクチャ
 
-1. ブラウザ → frontend Cloud Run（公開、AppAuth でログイン）
-2. ブラウザ → /api/* リクエスト
-3. frontend nginx の Lua が:
-   a. lua_shared_dict cache に ID token があれば再利用（TTL 50分）
-   b. なければ GCE metadata server から取得（audience = CC_TUNNEL_AUDIENCE）
-   c. Authorization: Bearer <token> を付与して cc-tunnel に proxy
-4. cc-tunnel は Cloud Run の IAM invoker チェック → AppAuth セッション検証
+```
+Browser → DNS (cctunnel.pollenjp.com → LB IP) → Global HTTPS LB
+          ├── /api/* → cc-tunnel Cloud Run (path rewrite: /api を削除、timeout=3600s for SSE)
+          └── /*    → frontend Cloud Run (SPA + nginx try_files)
+```
 
-### cache 戦略
+### Apply 後の手順（殿の作業）
 
-- `lua_shared_dict id_tokens 1m`
-- TTL 50分（token 有効期限 60分の安全マージン）
-- cache miss 時のみ metadata server を叩く（再startup・cron 不要）
+1. `terragrunt apply` で Global LB / managed cert を一括作成
+   （cert は PROVISIONING 状態で apply 完了）
+2. outputs の `lb_ip` を確認し、Cloudflare で A レコード設定:
+   - Record: `cctunnel.pollenjp.com` → `<lb_ip>`
+   - Proxy: DNS only（灰色雲、Proxy OFF）
+3. DNS 伝搬後、Google が cert を発行（数十分〜数時間）
+4. `lb_managed_cert_status = ACTIVE` を確認後、`https://cctunnel.pollenjp.com/` でアクセス可能
 
-### Phase 3 IAP 移行時の差分
+### Terraform 変数
 
-- HTTPS LB + serverless NEG 構成で frontend/cc-tunnel を統合
-- LB に IAP を有効化、cc-tunnel の ingress を INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCING に変更
-- nginx の Lua ブロックは IAP が token 検証を引き受けるため削除可（削除しなくても harm は少ない）
-- AppAuth との責務整理は Phase 3 cmd で別途決定
+| 変数 | 値 | 説明 |
+|------|----|------|
+| `lb_fqdn` | `cctunnel.pollenjp.com` | SSL cert のドメイン |
 
-### セキュリティ前提
+### 注意事項
+
+- cert が ACTIVE になるまでは HTTPS アクセス不可（ERR_SSL_PROTOCOL_ERROR）
+- ingress=INTERNAL_LOAD_BALANCING 切替後、Cloud Run の `.run.app` URL への直接アクセスは拒否される
+- ローカル開発（compose.yaml）は引き続き nginx の /api/ proxy_pass で動作（Cloud Run 側の変更は影響なし）
+
+### Phase 3 IAP 移行時の差分
 
-- cc-tunnel URL は GCP Cloud Run revision suffix で推測困難（allUsers invoker も削除済み）
-- ローカル開発環境では metadata server に到達できないため /api/ が 502 を返す（想定動作）
-- Cloud Armor によるレート制限は Phase 2 で追加予定
+- backend service に `iap { }` ブロックを追加するだけで IAP 有効化可能（LB 構成の変更最小）
+- AppAuth との責務整理（IAP 一本化 or 並走）は Phase 3 cmd で別途決定
```
