# 変更ログ: subtask_terraform_live_cc_tunnel_unit_001

## 作業概要

- **タスクID**: subtask_terraform_live_cc_tunnel_unit_001
- **担当**: 足軽3号 (ashigaru3)
- **日時**: 2026-04-26T16:01:33+00:00

## 対象ファイル

`terraform/live/local/cc-tunnel/terragrunt.hcl`

## 実施内容

ファイルは既に存在していたが `deploy_env` (必須変数) が未設定だった (VAR_GAP_001)。
`inputs` ブロックに `deploy_env = "${include.root.locals.env}"` を追加した。

### 変更前

```hcl
inputs = {
  project_id                            = "${include.root.locals.gcp_project_id}"
  artifact_registry_repository_location = "${dependency.artifact_registry.outputs.artifact_registry_repository_location}"
  artifact_registry_repository_name     = "${dependency.artifact_registry.outputs.artifact_registry_repository_name}"
  terraform_runner_sa_email             = "${include.root.locals.terraform_runner_sa_email}"
}
```

### 変更後

```hcl
inputs = {
  project_id                            = "${include.root.locals.gcp_project_id}"
  artifact_registry_repository_location = "${dependency.artifact_registry.outputs.artifact_registry_repository_location}"
  artifact_registry_repository_name     = "${dependency.artifact_registry.outputs.artifact_registry_repository_name}"
  terraform_runner_sa_email             = "${include.root.locals.terraform_runner_sa_email}"
  deploy_env                            = "${include.root.locals.env}"
}
```

## 検証

- `project_id` → `include.root.locals.gcp_project_id` (root.hcl 定義に準拠) ✓
- `artifact_registry_repository_location` → modules の outputs 名と一致 ✓
- `artifact_registry_repository_name` → modules の outputs 名と一致 ✓
- `terraform_runner_sa_email` → `include.root.locals.terraform_runner_sa_email` ✓
- `deploy_env` → `include.root.locals.env` (VAR_GAP_001 解消) ✓
- LF 改行のみ (CRLF なし) ✓
- git 操作ゼロ ✓

## modules/cc-tunnel/variables.tf 必須変数 対応状況

| 変数名 | デフォルト | inputs 設定 |
|--------|-----------|------------|
| `project_id` | なし (必須) | ✓ |
| `deploy_env` | なし (必須) | ✓ (今回追加) |
| `artifact_registry_repository_location` | なし (必須) | ✓ |
| `artifact_registry_repository_name` | なし (必須) | ✓ |
| `terraform_runner_sa_email` | なし (必須) | ✓ |
| `image_name` | "cc-tunnel" (任意) | — |
| `enable_public_access` | false (任意) | — |
| `container_port` | 5173 (任意) | — |
