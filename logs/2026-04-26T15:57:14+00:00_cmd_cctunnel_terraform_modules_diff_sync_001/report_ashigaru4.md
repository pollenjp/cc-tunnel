# 変更ログ: subtask_terraform_cloud_run_gap_fix_001

- **担当**: 足軽4号 (ashigaru4)
- **親コマンド**: cmd_cctunnel_terraform_modules_diff_sync_001
- **実行日時**: 2026-04-26T15:57:14+00:00

## 実施内容

### API_GAP_001 修正
- **ファイル**: `terraform/modules/init_project/main.tf`
- **変更**: `activate_apis` リストに `"run.googleapis.com"` を追加
- **コメント**: `# Cloud Run v2`

### ROLE_GAP_001 修正
- **ファイル**: `terraform/modules/prepare_terraform_sa/main.tf`
- **変更**: `# "roles/run.admin"` のコメントを解除
- **コメント更新**: `# Cloud Run v2 サービス管理 (cc-tunnel module)`

## 品質確認

| 項目 | 結果 |
|------|------|
| `run.googleapis.com` 追加 | OK (line 7) |
| `roles/run.admin` コメント解除 | OK (line 27) |
| LF改行のみ (init_project/main.tf) | OK (CRLF=0) |
| LF改行のみ (prepare_terraform_sa/main.tf) | OK (CRLF=0) |
| git 操作 | ゼロ (working tree のみ) |

## スコープ外

- VAR_GAP_001 (deploy_env 未設定): live unit 未作成のため別コマンドで対処。本タスクのスコープ外。
