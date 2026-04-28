# Phase 3 Report: live/local/frontend/terragrunt.hcl 作成

## タスク
subtask_terraform_frontend_live_unit_001

## 実施内容
`terraform/live/local/frontend/terragrunt.hcl` を新規作成した。

## 確認した既存パターン

### root.hcl のキー名
- `include.root.locals.gcp_project_id`
- `include.root.locals.env`
- `include.root.locals.terraform_runner_sa_email`

### artifact_registry/outputs.tf の output 名
- `artifact_registry_repository_location`
- `artifact_registry_repository_name`

### cc-tunnel/outputs.tf の output 名
- `cc_tunnel_url` (Phase 2 追加済み確認)

## 作成ファイル
- `terraform/live/local/frontend/terragrunt.hcl`
  - dependency: artifact_registry, cc_tunnel
  - source: `./../../../modules//frontend`
  - api_upstream = dependency.cc_tunnel.outputs.cc_tunnel_url

## 品質チェック
- LF only: OK (CRLF 0件)
- git 操作: ゼロ
