# subtask_terraform_frontend_integrate_001 変更ログ

## 実行日時
2026-04-27T05:36:06+00:00

## 担当
ashigaru3

## タスク概要
殿の直命により frontend Terraform リソースを modules/cc-tunnel/ 内に統合。
別モジュール（modules/frontend/）は不使用、cc-tunnel module の一部として frontend の
Cloud Build trigger + Cloud Run v2 service を追加した。

## 変更ファイル

### 新規作成: terraform/modules/cc-tunnel/frontend.tf

以下のリソースを追加:

| リソース種別 | リソース名 | 説明 |
|---|---|---|
| locals | - | fe_ プレフィックスの frontend 専用ローカル変数 |
| google_service_account | fe_builder_sa | Cloud Build Builder SA |
| google_project_iam_member | fe_builder_sa_roles | logging.logWriter IAM |
| google_artifact_registry_repository_iam_member | fe_registry_writer | Artifact Registry writer権限 |
| google_cloudbuild_trigger | fe_trigger | frontend Cloud Build trigger（apps/frontend/**） |
| terraform_data | fe_run_trigger_once | 初回ビルド実行（cc-tunnelと同パターン） |
| google_service_account | fe_runtime_sa | Cloud Run Runtime SA |
| google_cloud_run_v2_service | fe_cloud_run | frontend Cloud Run v2サービス |
| google_cloud_run_v2_service_iam_member | fe_public_access | パブリックアクセス（count制御） |

### 設計ポイント
- `random_string.unique_id` は main.tf の既存リソースを使い回し（重複作成なし）
- `API_UPSTREAM` env = `google_cloud_run_v2_service.cloud_run.uri`（同モジュール内参照）
- fe_ プレフィックスで cc-tunnel リソースと区別
- `fe_dockerfile_dir = "apps/frontend"`

### 更新: terraform/modules/cc-tunnel/variables.tf

以下の変数を末尾に追加:
- `frontend_image_name` (string, default: "frontend")
- `frontend_container_port` (number, default: 8080)
- `frontend_enable_public_access` (bool, default: false)

### 更新: terraform/modules/cc-tunnel/outputs.tf

以下のoutputを追加:
- `frontend_url` = `google_cloud_run_v2_service.fe_cloud_run.uri`

## 品質確認

- [x] modules/cc-tunnel/frontend.tf が新規作成済み
- [x] fe_cloud_run に API_UPSTREAM env が `google_cloud_run_v2_service.cloud_run.uri` を参照
- [x] variables.tf に frontend_image_name / frontend_container_port / frontend_enable_public_access 追加済み
- [x] outputs.tf に frontend_url 追加済み
- [x] LF 改行のみ（CRLF 0件確認済み）
- [x] git 操作ゼロ
