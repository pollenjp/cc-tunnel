# 変更ログ: subtask_terraform_frontend_docs_redo_001

## 実行日時
2026-04-27T05:36:20+00:00

## 担当
足軽4号 (ashigaru4)

## タスクID
subtask_terraform_frontend_docs_redo_001

## 変更ファイル
`docs/terraform-setup.md`

## 変更内容

### (A) 削除: 「5. live/local/frontend」Apply 順序セクション
- 誤った別モジュール前提の Apply 手順セクションを削除
- frontend は cc-tunnel と同時 apply されるため独立した step 5 は不要

### (B) 削除: 「modules/frontend/」独立セクション
- 誤った別モジュール前提の modules/frontend/ セクションを削除

### (C) 追記: modules/cc-tunnel セクションに frontend 統合の説明
- 「modules/cc-tunnel/ に frontend（nginx + React SPA）の Cloud Build trigger と
  Cloud Run サービスも統合されており、cc-tunnel API と同時に apply される。」

### (D) 更新: Apply 順序 step 4（cc-tunnel）の説明
- 「cc-tunnel API + frontend（nginx + SPA）の Cloud Build trigger と Cloud Run サービスを作成する。
  Cloud Build GitHub App connection が必要（手動設定、C003）。
  apply 後に frontend URL と cc-tunnel API URL が outputs に表示される。」

### ディレクトリ構成の修正
- modules/ から `frontend/` 行を削除し、`cc-tunnel/` のコメントを更新
- live/local/ から `frontend/` ディレクトリ行を削除し、`cc-tunnel/` のコメントを更新

## 品質確認
- [x] 「5. live/local/frontend」Apply 順序が存在しない
- [x] 「modules/frontend/」独立セクションが存在しない
- [x] cc-tunnel セクションに frontend 統合の説明が追記されている
- [x] LF 改行のみ（CRLF なし: grep -Pc "\r" → 0）
- [x] git 操作ゼロ
