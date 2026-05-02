# 変更ログ: subtask_terraform_frontend_appauth_phase1_001

## 実施日時
2026-04-28T02:04:07+00:00

## 変更内容

### terraform/live/local/cc-tunnel/terragrunt.hcl の diff

```diff
@@ -20,5 +20,7 @@ inputs = {
 
   frontend_image_name           = "frontend"
   frontend_container_port       = 8080
-  frontend_enable_public_access = false
+
+  enable_public_access          = true  # Phase 1: cc-tunnel を allUsers 公開（AppAuth で保護）
+  frontend_enable_public_access = true  # Phase 1: ブラウザから直接アクセスを許可
 }
```

### docs/terraform-setup.md 追記箇所

ファイル末尾に以下のセクションを追記:

```markdown
## Phase 1: frontend → cc-tunnel 認証戦略

Phase 1 では候補Y（公開 + AppAuth）を採用:
- cc-tunnel/frontend ともに `enable_public_access = true` で allUsers 公開
- cc-tunnel の保護は AppAuth（token-based、apps/cc-tunnel/internal/api/app_auth.go）で実施
- nginx は単純な reverse proxy（/api/* → cc-tunnel URI）、ID token 不要
- Phase 3 で IAP 化（HTTPS LB + serverless NEG + IAP）する際に再設計予定

### Phase 1 のセキュリティ前提
- cc-tunnel URL は GCP Cloud Run revision suffix で推測困難
- AppAuth のレート制限（login endpoint）を推奨（本番化前）
- Phase 2: Cloud Armor 導入で URL 漏洩リスクを軽減予定

### 将来移行パス
- Phase 2: HTTPS LB + serverless NEG（同一ドメイン化）
- Phase 3: LB に IAP 有効化 → AppAuth との責務整理
```

## 候補Y 採用の経緯

軍師レポート（gunshi_frontend_cctunnel_auth_design）で GO_WITH_CHANGES 判定。
候補Y（公開 + AppAuth）を採用:
- IAP（候補X）と比較してシンプルで Phase 1 に適している
- cc-tunnel 既存の AppAuth 機能を最大活用
- Phase 3 で IAP 化する余地を残す設計

## 殿の apply + 動作確認手順

```bash
cd ~/ghq/github.com/pollenjp/cc-tunnel/terraform/live/local/cc-tunnel
terragrunt plan   # enable_public_access / frontend_enable_public_access の差分確認
terragrunt apply
```

apply 後:
- cc-tunnel Cloud Run の allUsers IAM binding が追加される
- frontend Cloud Run の allUsers IAM binding が追加される
- frontend URL にブラウザから直接アクセスできることを確認

## 品質チェック結果

- [x] terragrunt.hcl: enable_public_access=true 追加済み
- [x] terragrunt.hcl: frontend_enable_public_access=true 変更済み
- [x] docs/terraform-setup.md: Phase 1 認証戦略セクション追記済み
- [x] LF 改行のみ（CRLF なし確認済み）
- [x] git 操作ゼロ
- [x] logs/ 変更ログ作成済み
