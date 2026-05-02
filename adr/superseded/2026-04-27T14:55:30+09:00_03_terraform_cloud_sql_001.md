# subtask_terraform_cloud_sql_docs_001 変更ログ

## 実施内容

対象ファイル: `docs/terraform-setup.md`

### (A) modules/cc-tunnel/ 説明の更新（行 117-120）

Cloud SQL for PostgreSQL インスタンスの統合を追記。
DB パスワードは Secret Manager で管理、Cloud SQL Auth Proxy（Unix socket）接続の説明を追加。

### (B) Apply 順序 Step 4 の説明更新（行 62-74）

- タイトルに「Cloud SQL インスタンス（PostgreSQL）」を追加
- outputs に `cloud_sql_instance_connection_name` を追加
- goose による自動 migrate の説明を追加

### (C) Cloud SQL 接続方式セクション追加（新規）

- 接続方式: Unix socket `/cloudsql/INSTANCE_CONNECTION_NAME`
- DATABASE_URL フォーマット
- sslmode=disable の理由（Cloud SQL Auth Proxy が TLS 担当）
- パスワード管理（Secret Manager + random_password）

### (D) マイグレーション適用方式セクション追加（新規）

- goose embed による自動適用
- advisory lock による競合安全の説明

### (E) Cloud SQL に関する注意事項セクション追加（新規）

- C001: POSTGRES_17 使用（compose/CI との差異）
- C002: max_connections（デフォルト 400）への注意
- C003: deletion_protection=false（dev 用）

## 品質確認

- LF 改行のみ: OK（CRLF なし）
- Cloud SQL キーワード確認: 15 箇所
- git 操作: ゼロ
