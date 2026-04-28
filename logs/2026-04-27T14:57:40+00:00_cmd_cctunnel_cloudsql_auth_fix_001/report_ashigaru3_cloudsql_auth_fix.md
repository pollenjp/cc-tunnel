# 変更ログ: Cloud SQL SQLSTATE 28P01 修正

**task_id**: subtask_terraform_cloudsql_auth_fix_001  
**parent_cmd**: cmd_cctunnel_cloudsql_auth_fix_001  
**担当**: 足軽3号 (ashigaru3)  
**日時**: 2026-04-27T14:57:40+00:00  

## 根本原因

Cloud Run v2 の `env.value` フィールド内の `$(DB_PASSWORD)` 記法は
`value_source.secret_key_ref` 由来の env を参照できない。
起動時に動的解決されるため展開タイミングが異なり、リテラル文字列 `"$(DB_PASSWORD)"` が
Postgres に送信されて SQLSTATE 28P01（認証失敗）が発生していた。

## 修正内容

### terraform/modules/cc-tunnel/cloudsql.tf

**変更1: random_password.cs_password を URL safe に変更（案C）**
- `length`: 24 → 32
- `special`: true → false
- `override_special` 削除
- `upper = true`, `lower = true`, `numeric = true` 追加

**変更2: DATABASE_URL 格納 secret を新規追加（案D）**
- `google_secret_manager_secret.cs_database_url_secret` 追加
  - secret_id: `{cs_instance_name}-database-url`
- `google_secret_manager_secret_version.cs_database_url_secret_version` 追加
  - `format()` で DATABASE_URL 全体を組み立てて格納
- `google_secret_manager_secret_iam_member.cs_runtime_database_url_accessor` 追加
  - runtime_sa に secretAccessor 権限を付与

**変更3: 旧 password-only secret 削除**
- `google_secret_manager_secret.cs_password_secret` 削除
- `google_secret_manager_secret_version.cs_password_secret_version` 削除
- `google_secret_manager_secret_iam_member.cs_runtime_secret_accessor` 削除

### terraform/modules/cc-tunnel/main.tf

**変更4: Cloud Run env 修正**
- `DB_PASSWORD` env（secret_key_ref）削除
- `DATABASE_URL` env（value = "...$(DB_PASSWORD)..."）削除
- `DATABASE_URL` env を `secret_key_ref` で直接注入に変更

**変更5: depends_on 更新**
- `cs_runtime_secret_accessor` → `cs_runtime_database_url_accessor`
- `cs_password_secret_version` → `cs_database_url_secret_version`

### terraform/modules/cc-tunnel/outputs.tf

**変更6: output 更新**
- `cloud_sql_password_secret_id` 削除
- `cloud_sql_database_url_secret_id` 追加（sensitive=true）

### docs/terraform-setup.md

**変更7: Cloud Run v2 env 展開の落とし穴を追記**
- 新セクション「Cloud Run v2 env 展開の落とし穴」を追加

## 品質チェック

- [x] random_password.cs_password: special=false, length=32, upper/lower/numeric=true
- [x] cs_database_url_secret: secret_id に "-database-url" 含む
- [x] cs_database_url_secret_version: format() で DATABASE_URL 組み立て
- [x] cs_runtime_database_url_accessor: secret 単位で secretAccessor bind
- [x] 旧 cs_password_secret / cs_password_secret_version / cs_runtime_secret_accessor: 削除
- [x] main.tf Cloud Run env: DATABASE_URL のみ secret_key_ref（DB_PASSWORD env なし）
- [x] main.tf depends_on: cs_runtime_database_url_accessor + cs_database_url_secret_version
- [x] outputs.tf: cloud_sql_database_url_secret_id 追加 / cloud_sql_password_secret_id 削除
- [x] docs/terraform-setup.md: Cloud Run v2 env 展開の落とし穴追記
- [ ] LF 改行のみ確認（STEP 9 で実施）
- [x] git 操作ゼロ
