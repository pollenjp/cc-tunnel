# 変更ログ: subtask_terraform_cloudbuild_role_001

**実行者**: ashigaru2  
**日時**: 2026-04-26T14:55:18+00:00  
**親コマンド**: cmd_cctunnel_terraform_modules_sync_001

## 変更内容

### 1. terraform/modules/prepare_terraform_sa/main.tf (line 28)

**変更前**:
```hcl
    # "roles/cloudbuild.builds.editor",        # Cloud Build トリガーの作成・更新
```

**変更後**:
```hcl
    "roles/cloudbuild.builds.editor",        # Cloud Build trigger 作成・更新 + run/describe (cc-tunnel module)
```

**理由**: `terraform/modules/cc-tunnel/` が Cloud Build trigger を作成・実行するため、Terraform Runner SA に `roles/cloudbuild.builds.editor` が必須。軍師分析 `gunshi_terraform_modules_sync_analysis` にて必要性確定済み。

### 2. docs/terraform-setup.md

「既知の問題と対処」セクションの後に「modules/cc-tunnel について」セクションを追記:
- cc-tunnel モジュールが管理するリソース一覧
- Cloud Build GitHub App connection の手動操作手順
- Terraform Runner SA に必要な追加ロール（roles/cloudbuild.builds.editor）

## 品質確認

| 確認項目 | 結果 |
|--------|------|
| main.tf line 28 コメント解除 | OK |
| LF 改行のみ（CRLF なし） | OK（grep -Pc "\r" = 0） |
| terraform fmt -check | SKIP（terraform バイナリ未インストール） |
| docs/terraform-setup.md 追記 | OK |
| git 操作 | ゼロ（working tree のみ変更） |

## 殿の apply 手順

```bash
cd ~/ghq/github.com/pollenjp/cc-tunnel

# 1. Terraform Runner SA に cloudbuild.builds.editor を付与
cd terraform/prepare/local/terraform_sa
terragrunt plan   # roles/cloudbuild.builds.editor が追加されることを確認
terragrunt apply

# 2. cc-tunnel モジュールを apply（Cloud Build GitHub App connection が完了していること前提）
cd terraform/live/local/cc-tunnel
terragrunt plan
terragrunt apply
```

### 前提条件（apply 前に確認）

- [ ] Cloud Build GitHub App が cc-tunnel リポジトリに接続済み
  - GCP Console > Cloud Build > Triggers > Manage repositories で確認

## 次のステップ

- `live/local/cc-tunnel/` の Terragrunt unit はまだ未作成（別タスク予定）
- cc-tunnel モジュール本体（`terraform/modules/cc-tunnel/`）の設計・実装は別タスク
