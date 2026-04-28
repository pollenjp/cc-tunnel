# HCL Syntax Fix Report

**Task**: subtask_terraform_hcl_syntax_fix_001
**Date**: 2026-04-27T23:59:19+00:00
**Agent**: ashigaru2

## 修正内容

### 対象ファイル
`terraform/modules/cc-tunnel/main.tf`

### 問題
HCL は `;` 区切りで複数引数を1行に詰める構文を許可しない。
lines 191–195 の env block に `;` が使用されていた。

### diff

```diff
-      env { name = "EXECUTION_PROVIDER";    value = "docker_gce" }
-      env { name = "GCE_PROJECT_ID";        value = var.project_id }
-      env { name = "GCE_ZONE";              value = var.gce_zone }
-      env { name = "GCE_MACHINE_TYPE";      value = var.gce_machine_type }
-      env { name = "CC_REMOTE_AGENT_IMAGE"; value = local.cra_fqim }
+      env {
+        name  = "EXECUTION_PROVIDER"
+        value = "docker_gce"
+      }
+      env {
+        name  = "GCE_PROJECT_ID"
+        value = var.project_id
+      }
+      env {
+        name  = "GCE_ZONE"
+        value = var.gce_zone
+      }
+      env {
+        name  = "GCE_MACHINE_TYPE"
+        value = var.gce_machine_type
+      }
+      env {
+        name  = "CC_REMOTE_AGENT_IMAGE"
+        value = local.cra_fqim
+      }
```

### 他ファイルの `;` 確認
cc-remote-agent.tf / frontend.tf の `;` はシェルスクリプトのヒアドキュメント内（文字列リテラル）のため HCL 構文エラーではない。修正不要。

## 実行結果

| ステップ | 結果 |
|---------|------|
| `terraform fmt main.tf` | OK |
| `terraform fmt modules/cc-tunnel/` | OK (他ファイル変更なし) |
| `terraform validate` (モジュール単体) | **Success! The configuration is valid.** |
| LF 確認 (`grep -Pc "\r" main.tf`) | 0 — LF only OK |
| git 操作 | ゼロ（禁止遵守） |

## terragrunt validate について

`terragrunt validate` は以下の理由で失敗（HCL 構文修正とは無関係）:
1. GCS backend 403 エラー: SA `dev-ai-agent-vm-sa-jsmywsey@my-project-3-487404.iam.gserviceaccount.com` に `storage.objects.list` 権限なし
2. `terragrunt.hcl` line 16-17 の `dependency` 変数未解決（依存 stack が未初期化）

これらはインフラ設定の問題であり、今回の HCL 構文修正とは独立した問題。
`terraform validate` は PASS しており、HCL 構文修正は完了。

## 殿が apply する際の手順

```bash
cd ~/ghq/github.com/pollenjp/cc-tunnel/terraform/live/local/cc-tunnel

# 1. terragrunt plan で差分確認
terragrunt plan

# 2. 問題なければ apply（殿の判断）
terragrunt apply
```

## 品質チェックリスト

- [x] main.tf の `;` 区切り env block が改行区切りに修正
- [x] 他 `.tf` ファイルの `;` 誤記述なし（シェルスクリプト内のみ）
- [x] terraform fmt 整形済み
- [x] terraform validate PASS
- [x] LF 改行のみ（CRLF なし）
- [x] git 操作ゼロ
- [x] 変更ログ作成済み
