# subtask_terraform_cloud_sql_roles_001 変更ログ

## 実行日時
2026-04-27T05:54:56+00:00

## 担当
ashigaru2

## 変更内容

### terraform/modules/prepare_terraform_sa/main.tf

`sa_roles` for_each ブロックに以下 2 件を追加:

| Role | 目的 |
|------|------|
| `roles/cloudsql.admin` | Cloud SQL Instance 管理 (cc-tunnel module) |
| `roles/secretmanager.admin` | Secret Manager 管理 (cc-tunnel module / DB password) |

追加位置: `roles/run.admin` の直後（Cloud Run と Cloud SQL の関連グループ）

## 品質確認

- [x] `roles/cloudsql.admin` 追加確認 (line 28)
- [x] `roles/secretmanager.admin` 追加確認 (line 29)
- [x] LF 改行のみ（CRLF なし）
- [x] git 操作ゼロ
