# cmd_cctunnel_terraform_execution_provider_001 変更ログ

## 概要
cc-tunnel Cloud Run が EXECUTION_PROVIDER=docker_gce で稼働できるよう Terraform を整備。
軍師による設計レビュー（DD001〜DD005）を経て足軽3号が実装。Go コード変更なし。

## 軍師設計レビュー（gunshi_execution_provider_design）判定: GO_WITH_CHANGES

### DD001: cc-remote-agent ビルド・push 方式
**推奨案: 候補A — google_cloudbuild_trigger（GitHub push 自動ビルド）**
- 既存 cc-tunnel API / frontend と同じ Cloud Build trigger パターンを流用
- included_files = ["apps/cc-remote-agent/**"] で trigger 発火
- terraform_data.cra_run_trigger_once で初回 build を実施（cc-tunnel/frontend と同パターン）
- GitHub App connection は既存 trigger で設定済みのため再設定不要

### DD002: イメージタグ戦略
**推奨案: 候補A — latest tag**
- dev 環境かつ既存 cc-tunnel / frontend も latest 採用
- GCE VM startup_script は `docker pull` するため VM 起動毎に最新版を取得
- git-sha は Cloud Run env 毎回更新が必要で複雑化するため不採用

### DD003: GCE VM Docker 環境
**確定済み: COS（Container-Optimized OS）**
- sdk_client.go 内で `cosImage = "projects/cos-cloud/global/images/family/cos-stable"` ハードコード
- Terraform 側での設定不要。startup_script で Docker daemon + cc-remote-agent docker run が完結

### DD004: Artifact Registry 認証
**推奨案: 候補A — GCE VM の default compute SA に roles/artifactregistry.reader 付与**
- sdk_client.go の CreateInstance は ServiceAccount を指定しない設計
- GCE VM はデフォルト compute SA（<PROJECT_NUMBER>-compute@developer.gserviceaccount.com）で起動
- Terraform で AR repo 単位の IAM bind を付与することで Go コード変更なしで解決
- 将来的には専用 SA への切替を Phase 2 cmd で実施推奨（C001）

### DD005: Terraform / Go コードの境界線
**境界明確**
- Terraform 管理: Cloud Run env（EXECUTION_PROVIDER/GCE_PROJECT_ID/GCE_ZONE/GCE_MACHINE_TYPE/CC_REMOTE_AGENT_IMAGE）+ IAM 権限
- Go コード管理: VM image（COS）/Docker daemon設定/cc-remote-agent コンテナ起動/VM ライフサイクル
- 侵食なし。Go コード変更は本タスクで発生しない

## API/Role Gap 分析

### API gap: 追加 0 件
- compute.googleapis.com, cloudbuild.googleapis.com, artifactregistry.googleapis.com,
  iam.googleapis.com はすべて既追加済み

### Role gap: CRITICAL 3 件（本実装で解消）
| SA | Role | 付与先 |
|---|---|---|
| Cloud Run runtime SA | roles/compute.instanceAdmin.v1 | project 単位 |
| Cloud Run runtime SA | roles/iam.serviceAccountUser | default compute SA 単位 |
| GCE default compute SA | roles/artifactregistry.reader | AR repo 単位 |

## 実装内容（足軽3号: subtask_terraform_execution_provider_impl_001）

### 新規ファイル
- `terraform/modules/cc-tunnel/cc-remote-agent.tf`
  - `google_service_account.cra_builder_sa` — Cloud Build 用 Builder SA
  - `google_project_iam_member.cra_builder_sa_roles` — roles/logging.logWriter
  - `google_artifact_registry_repository_iam_member.cra_registry_writer` — roles/artifactregistry.writer
  - `google_cloudbuild_trigger.cra_trigger` — cc-remote-agent 用 Cloud Build trigger
  - `terraform_data.cra_run_trigger_once` — 初回ビルド実行（local-exec）
  - `data.google_project.current` — default compute SA email 取得用
  - `google_artifact_registry_repository_iam_member.cra_default_compute_sa_reader` — roles/artifactregistry.reader

### 更新ファイル
- `terraform/modules/cc-tunnel/main.tf`
  - Cloud Run env 5 件追加:
    - `EXECUTION_PROVIDER = "docker_gce"`
    - `GCE_PROJECT_ID = var.project_id`
    - `GCE_ZONE = var.gce_zone`
    - `GCE_MACHINE_TYPE = var.gce_machine_type`
    - `CC_REMOTE_AGENT_IMAGE = local.cra_fqim`
  - IAM 2 件追加:
    - `google_project_iam_member.cr_runtime_compute_admin` (roles/compute.instanceAdmin.v1)
    - `google_service_account_iam_member.cr_runtime_default_compute_sa_user` (roles/iam.serviceAccountUser)
  - `depends_on` 4 件追加（上記 IAM 2件 + cra_default_compute_sa_reader + cra_run_trigger_once）
- `terraform/modules/cc-tunnel/variables.tf` — gce_zone / gce_machine_type / cc_remote_agent_image_name 追加
- `terraform/modules/cc-tunnel/outputs.tf` — cc_remote_agent_image 追加
- `docs/terraform-setup.md` — cc-remote-agent 統合セクション追記

## Go コード変更: なし
- main.go の newProviderFromEnv は CC_REMOTE_AGENT_IMAGE / GCE_PROJECT_ID 等の env を読み込む実装が完成済み
- Terraform から正しい値を流すだけで動作する
- sdk_client.go の ServiceAccount 未指定（default compute SA 利用）も AR pull 権限を Terraform で付与することで補正

## 殿の apply 手順

### 前提
- bundle commit（🚨要対応参照）が完了していること
- Cloud Build GitHub App connection が設定済みであること（手動、GCP Console > Cloud Build > Triggers）

### apply 順序
```bash
# ステップ1: SA 権限更新
cd ~/ghq/github.com/pollenjp/cc-tunnel/terraform/prepare/local/terraform_sa
terragrunt plan && terragrunt apply

# ステップ2: Artifact Registry
cd ~/ghq/github.com/pollenjp/cc-tunnel/terraform/live/local/artifact_registry
terragrunt apply

# ステップ3: cc-tunnel モジュール（cc-remote-agent.tf 含む全リソース同時 apply）
cd ~/ghq/github.com/pollenjp/cc-tunnel/terraform/live/local/cc-tunnel
terragrunt plan  # C002（self-impersonation）の問題が出ないか確認
terragrunt apply
# → cra_run_trigger_once が cc-remote-agent image を AR に push（数分かかる）
# → Cloud Run revision 切替で EXECUTION_PROVIDER=docker_gce 有効化
```

### 検証手順
1. AR に cc-remote-agent イメージが push されていることを確認:
   `gcloud artifacts docker images list <AR_LOCATION>-docker.pkg.dev/<PROJECT>/<REPO>`
2. cc-tunnel API への POST /conversations でリクエストを送信し GCE VM 作成を確認:
   `gcloud compute instances list --project=<PROJECT>`
3. VM 内 docker logs で cc-remote-agent 起動を確認

## 品質確認
- CRLF: 0件
- git 操作: 0件
- 実装仕様（implementation_spec A〜F）と完全整合
