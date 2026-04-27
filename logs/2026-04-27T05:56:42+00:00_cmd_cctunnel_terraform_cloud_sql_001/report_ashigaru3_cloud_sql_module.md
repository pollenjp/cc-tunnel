# 変更ログ: subtask_terraform_cloud_sql_module_001

**日時**: 2026-04-27T05:56:42+00:00
**実行者**: ashigaru3
**親コマンド**: cmd_cctunnel_terraform_cloud_sql_001

## 実施内容

### 新規作成ファイル

#### `terraform/modules/cc-tunnel/cloudsql.tf`
- locals: `cs_instance_suffix`, `cs_instance_name`, `cs_db_name`, `cs_user_name`
- `random_password.cs_password` (24文字, special=true)
- `google_sql_database_instance.cs_instance` (POSTGRES_17, ZONAL, PD_SSD)
- `google_sql_database.cs_db`
- `google_sql_user.cs_user`
- `google_secret_manager_secret.cs_password_secret`
- `google_secret_manager_secret_version.cs_password_secret_version`
- `google_project_iam_member.cs_runtime_sql_client` (roles/cloudsql.client)
- `google_secret_manager_secret_iam_member.cs_runtime_secret_accessor` (roles/secretmanager.secretAccessor)

### 更新ファイル

#### `terraform/modules/cc-tunnel/main.tf`
- `google_cloud_run_v2_service.cloud_run` の template に追加:
  - `volumes` ブロック (cloudsql)
  - containers 内: `volume_mounts` (/cloudsql)
  - containers 内: `env.DB_PASSWORD` (Secret Manager 参照)
  - containers 内: `env.DATABASE_URL` (Unix socket 接続文字列)
- `depends_on` 追加: cs_runtime_sql_client, cs_runtime_secret_accessor, cs_password_secret_version

#### `terraform/modules/cc-tunnel/variables.tf`
追加変数 5件:
- `cloud_sql_region` (default: us-central1)
- `cloud_sql_version` (default: POSTGRES_17)
- `cloud_sql_tier` (default: db-custom-1-3840)
- `cloud_sql_db_name` (default: cctunnel)
- `cloud_sql_user` (default: cctunnel)

#### `terraform/modules/cc-tunnel/outputs.tf`
追加 output 3件:
- `cloud_sql_instance_connection_name`
- `cloud_sql_db_name`
- `cloud_sql_password_secret_id`

## 品質チェック結果

| 項目 | 結果 |
|------|------|
| cloudsql.tf 作成 | OK (cs_ プレフィックス全リソース) |
| google_sql_database_instance | OK |
| google_sql_database | OK |
| google_sql_user | OK |
| random_password + secret_manager | OK |
| cs_runtime_sql_client IAM | OK |
| cs_runtime_secret_accessor IAM | OK |
| Cloud Run volumes (cloudsql) | OK |
| Cloud Run volume_mounts | OK |
| DB_PASSWORD env (Secret Manager) | OK |
| DATABASE_URL env | OK |
| variables.tf 5変数 | OK |
| outputs.tf 3 outputs | OK |
| LF改行のみ | OK |
| git 操作ゼロ | OK |

## gcloud 確認

- `gcloud sql tiers list` コマンド: 出力なし（gcloud不可環境）
- Cloud SQL バージョン: POSTGRES_17 で安全側に固定
