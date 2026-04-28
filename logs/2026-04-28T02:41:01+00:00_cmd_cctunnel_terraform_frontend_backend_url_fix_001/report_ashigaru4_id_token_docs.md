# 変更ログ: subtask_terraform_frontend_id_token_docs_001

## 変更内容

### docs/terraform-setup.md
- `## Phase 1: frontend → cc-tunnel 認証戦略` セクションを候補Y → 候補X に完全置換
- 候補Y（allUsers 公開 + AppAuth 単独）の記述を削除
- 候補X（cc-tunnel 非公開 + IAM invoker + nginx ID token 注入）の記述を追加
  - ID token 注入フロー（4ステップ）
  - cache 戦略（lua_shared_dict id_tokens、TTL 50分）
  - Phase 3 IAP 移行時の差分
  - セキュリティ前提

### docs/frontend.md
- `### nginx reverse proxy（/api/ → cc-tunnel API）` セクション直下に `### ID token 注入（候補X 採用）` サブセクション追加
  - 実装概要（OpenResty + Lua の access_by_lua_block）
  - cache 設定（lua_shared_dict id_tokens、1m、50分 TTL）
  - audience env var（CC_TUNNEL_AUDIENCE）
  - トラブルシュート手順

## 候補Y → 候補X への方針変更の経緯

2026-04-28 殿の直命により、Phase 1 認証戦略が候補Y（公開 + AppAuth 単独）から
候補X（cc-tunnel 非公開 + IAM invoker + nginx ID token 注入）へ転換。
軍師（gunshi）の設計ドキュメント DD-AUTH001（gunshi_auth_token_injection_design）に基づく実装。

## 品質確認

- CRLF チェック: terraform-setup.md = 0件 (LF only OK), frontend.md = 0件 (LF only OK)
- 候補Y の記述（allUsers 公開 + AppAuth 単独）: 削除済み ✓
- git 操作: ゼロ ✓
