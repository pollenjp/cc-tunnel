# subtask_terraform_lb_neg_apply_fix_001 変更ログ

実行日時: 2026-04-28T07:48:13+00:00
担当: ashigaru3

## Fix 1: ingress enum 修正 4箇所

誤: `INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCING`
正: `INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER`

### terraform/modules/cc-tunnel/main.tf (L161)
```diff
-  ingress             = "INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCING"
+  ingress             = "INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER"
```

### terraform/modules/cc-tunnel/frontend.tf (L139)
```diff
-  ingress             = "INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCING"
+  ingress             = "INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER"
```

### docs/terraform-setup.md (L268)
```diff
-- cc-tunnel/frontend ともに ingress=INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCING（LB 経由のみ）
+- cc-tunnel/frontend ともに ingress=INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER（LB 経由のみ）
```

### docs/frontend.md (L636)
```diff
-- ingress: `INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCING`（LB 経由のみ）
+- ingress: `INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER`（LB 経由のみ）
```

## Fix 2: lb_managed_cert_status output 削除

`terraform/modules/cc-tunnel/outputs.tf` から以下ブロックを完全削除:
```hcl
output "lb_managed_cert_status" {
  value       = google_compute_managed_ssl_certificate.lb_cert.managed[0].status
  description = "Managed SSL cert provisioning status (ACTIVE になればアクセス可)"
}
```
理由: `managed[0].status` は Terraform で参照できない属性。

## Fix 3: docs/terraform-setup.md に gcloud cert 確認手順追記

`### Apply 後の手順（殿の作業）` の手順4を以下に変更:
```markdown
4. cert の ACTIVE を確認:
   ```bash
   gcloud compute ssl-certificates describe <cert-name> \
     --global --format="value(managed.status)"
   ```
   cert-name は `${deploy_env}-${random_id}-lb-cert` 形式。apply 後 `terragrunt output` でリソース名を確認すること。
   `ACTIVE` になれば `https://cctunnel.pollenjp.com/` でアクセス可能
```

## 品質チェック結果

| 項目 | 結果 |
|------|------|
| INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCING 残留 | ゼロ ✓ |
| INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER 統一 | 4箇所確認 ✓ |
| lb_managed_cert_status 残留 | ゼロ ✓ |
| gcloud cert 確認手順追記 | 完了 ✓ |
| terraform fmt | terraform コマンド未インストール（手動確認: インデント正常） |
| CRLF | 全ファイル LF only ✓ |
| git 操作 | ゼロ ✓ |
