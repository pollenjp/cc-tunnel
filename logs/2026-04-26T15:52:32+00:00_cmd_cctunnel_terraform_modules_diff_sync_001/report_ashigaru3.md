# subtask_terraform_modules_diff_verify_001 — 足軽3号 報告書

日時: 2026-04-26T15:52 UTC  
担当: ashigaru3  
親タスク: cmd_cctunnel_terraform_modules_diff_sync_001

---

## 1. 現状の git ステータス（実測値）

### 状態の乖離

タスクYAMLの前提（「working tree のみ、staging なし」）と実際の状態は異なる:

| 項目 | タスクYAML前提 | 実際の状態 |
|------|--------------|-----------|
| modules/cc-tunnel/ | working tree に新規ファイル | HEAD コミット(5bc2073)済み + staging に追加変更あり |
| prepare_terraform_sa/ | working tree のみ | HEAD コミット(5bc2073)済み、working tree clean |
| staging の有無 | なし | あり（cc-tunnel + init_project） |

### git status 実測

```
# cc-tunnel (staged changes)
modified:   terraform/modules/cc-tunnel/main.tf     (staged)
modified:   terraform/modules/cc-tunnel/variables.tf (staged)

# prepare_terraform_sa (committed, clean)
nothing to commit, working tree clean
```

### HEAD commit (5bc2073) の内容

コミット者: pollenjp  
メッセージ: "feat: Cloud Build による cc-tunnel イメージビルドモジュールと trigger 設定を追加"  
含まれる変更:
- `terraform/modules/cc-tunnel/main.tf` (新規 137行 — Cloud Build のみ)
- `terraform/modules/cc-tunnel/variables.tf` (新規 22行)
- `terraform/modules/cc-tunnel/outputs.tf` (空)
- `terraform/live/local/cc-tunnel/.terraform.lock.hcl`
- `terraform/live/local/cc-tunnel/terragrunt.hcl`
- `terraform/modules/prepare_terraform_sa/main.tf` (line28 コメント解除)
- `docs/terraform-setup.md` (cc-tunnel セクション追記)

---

## 2. staged diff の resource 一覧

### terraform/modules/cc-tunnel/main.tf (staged 変更)

**名称変更 (breaking なし):**
- `local.builder_postfix` → `local.builder_suffix`
- `local.trigger_postfix` → `local.trigger_suffix`

**新規 locals:**
- `cloud_run_location`
- `cloud_run_name_suffix`, `cloud_run_name`
- `cr_runtime_sa_suffix`, `cr_runtime_sa_name`

**新規 resource:**
- `google_service_account.runtime_sa` — Cloud Run ランタイム SA
- `google_cloud_run_v2_service.cloud_run` — Cloud Run v2 サービス
- `google_cloud_run_v2_service_iam_member.public_access` — 全公開 IAM (conditional: `var.enable_public_access`)

### terraform/modules/cc-tunnel/variables.tf (staged 変更)

| 変数 | 型 | デフォルト | 状態 |
|------|-----|-----------|------|
| `deploy_env` | string | なし（必須） | NEW |
| `enable_public_access` | bool | false | NEW |
| `container_port` | number | 5173 | NEW |

### terraform/modules/init_project/main.tf (staged 変更)

**新規 resource:**
- `google_project_iam_audit_config.artifactregistry` — AR の DATA_WRITE 監査ログ有効化

---

## 3. 前 cmd との差異分析

前 cmd の軍師分析は Cloud Build のみを対象としていた。  
staged 変更は **Cloud Run** 追加フェーズの作業であり、前 cmd 分析の範囲外。

| 項目 | 前 cmd 分析 | 今回確認 |
|------|-----------|---------|
| API gap (Cloud Build のみ) | 0件 ✓ | Cloud Build 分は合致 |
| API gap (Cloud Run 追加後) | 未分析 | **`run.googleapis.com` 不足** |
| Role gap | 1件（cloudbuild.builds.editor） | cloudbuild.builds.editor 解除済み ✓ |
| Role gap (Cloud Run 追加後) | 未分析 | **`roles/run.admin` コメントアウト中** |

---

## 4. init_project との整合性確認

### 現在の activate_apis

```hcl
activate_apis = [
  "artifactregistry.googleapis.com",  # ✓
  "cloudbuild.googleapis.com",         # ✓
  "compute.googleapis.com",            # ✓
  "iam.googleapis.com",                # ✓
]
```

### staged 変更が追加する resource に必要な API

| resource | 必要 API | 状態 |
|----------|---------|------|
| google_cloud_run_v2_service.cloud_run | `run.googleapis.com` | **MISSING** |
| google_cloud_run_v2_service_iam_member | `run.googleapis.com` | **MISSING** |
| google_service_account.runtime_sa | `iam.googleapis.com` | ✓ |
| google_project_iam_audit_config.artifactregistry | `artifactregistry.googleapis.com` | ✓ |

**判定: 変更必要**  
`run.googleapis.com` を `terraform/modules/init_project/main.tf` の `activate_apis` に追加すること。

---

## 5. terraform_sa との整合性確認

### 現在の sa_roles (prepare_terraform_sa/main.tf)

```hcl
"roles/storage.admin",                 # ✓
# "roles/run.admin",                   # ← コメントアウト中！
"roles/cloudbuild.builds.editor",      # ✓ (HEAD commit で解除済み)
"roles/artifactregistry.admin",        # ✓
"roles/serviceusage.serviceUsageAdmin", # ✓
"roles/resourcemanager.projectIamAdmin", # ✓
"roles/iam.serviceAccountAdmin",       # ✓
"roles/iam.serviceAccountUser",        # ✓
"roles/compute.admin",                 # ✓
"roles/compute.osLogin",               # ✓
```

### Cloud Run 追加後に必要なロール

| ロール | 用途 | 状態 |
|--------|------|------|
| `roles/run.admin` | Cloud Run サービス作成・更新・削除 | **COMMENTED OUT** |
| `roles/iam.serviceAccountAdmin` | runtime SA 作成 | ✓ |
| `roles/resourcemanager.projectIamAdmin` | Cloud Run IAM binding 設定 | ✓ |

**判定: 変更必要**  
`roles/run.admin` のコメントアウトを解除すること。

---

## 6. terraform validate/plan 結果

terraform / terragrunt バイナリが利用不可のため、コード目視確認で代替。

### 判定: **FAIL（3件の問題）**

#### 問題1: `deploy_env` 変数が terragrunt.hcl に未設定

`terraform/modules/cc-tunnel/variables.tf` に `deploy_env` が追加されたが、デフォルト値なし（必須）。  
`terraform/live/local/cc-tunnel/terragrunt.hcl` の inputs に `deploy_env` が含まれていない。  
→ `terragrunt plan` 時に "Required variable not set: deploy_env" エラーが発生する。

**修正要**: `live/local/cc-tunnel/terragrunt.hcl` に `deploy_env = "${include.root.locals.env}"` を追加。

#### 問題2: `run.googleapis.com` API 未有効化

`google_cloud_run_v2_service` リソースが Cloud Run API を必要とするが、  
`init_project` module の `activate_apis` に `run.googleapis.com` がない。  
→ apply 時に API エラー。

**修正要**: `terraform/modules/init_project/main.tf` に `run.googleapis.com` を追加。

#### 問題3: `roles/run.admin` 未付与

terraform SA に Cloud Run サービス CRUD 権限がない。  
→ apply 時に IAM_PERMISSION_DENIED。

**修正要**: `terraform/modules/prepare_terraform_sa/main.tf` の `roles/run.admin` コメントアウト解除。

### 目視確認で問題なし

- HCL 構文: 問題なし
- resource 間の依存関係: `runtime_sa` → `cloud_run` → `public_access` の順で正しく依存
- `ignore_changes` の `deploy-timestamp` annotation: Cloud Run の自動デプロイとの競合防止として適切
- `deletion_protection = false`: 開発環境として適切

---

## 7. 殿が apply する際の手順

以下の修正を先に実施してから apply すること:

### 事前修正 (3件)

#### 修正1: `run.googleapis.com` を init_project に追加

`terraform/modules/init_project/main.tf` の `activate_apis` に追加:
```hcl
"run.googleapis.com",  # Cloud Run API
```

#### 修正2: `roles/run.admin` を prepare_terraform_sa で解除

`terraform/modules/prepare_terraform_sa/main.tf`:
```hcl
# 変更前
# "roles/run.admin",
# 変更後
"roles/run.admin",  # Cloud Run サービスの作成・更新・削除
```

#### 修正3: `deploy_env` を cc-tunnel terragrunt.hcl に追加

`terraform/live/local/cc-tunnel/terragrunt.hcl` の inputs に:
```hcl
deploy_env = "${include.root.locals.env}"
```

### Apply 順序

```bash
# 1. prepare_terraform_sa (ADC 直接)
cd terraform/prepare/local/terraform_sa
terragrunt plan  # roles/run.admin が追加されることを確認
terragrunt apply

# 2. init_project (SA impersonation)
cd terraform/live/local/init
terragrunt plan  # run.googleapis.com が追加されることを確認
terragrunt apply

# 3. cc-tunnel (SA impersonation)
cd terraform/live/local/cc-tunnel
terragrunt plan  # Cloud Run + SA が表示されることを確認
terragrunt apply
```

---

## 8. 変更ログ作成: 完了

`logs/2026-04-26T15:52:32+00:00_cmd_cctunnel_terraform_modules_diff_sync_001/report_ashigaru3.md`

---

## まとめ

| 確認項目 | 結果 |
|---------|------|
| staged diff 取得・resource 列挙 | 完了 |
| 前 cmd 分析との差異確認 | staged 変更は Cloud Run 追加フェーズ（前 cmd 分析範囲外）|
| init_project 照合 | **変更必要: `run.googleapis.com` 追加** |
| terraform_sa 照合 | **変更必要: `roles/run.admin` 解除** |
| validate/plan (目視) | **FAIL: 3件の問題 (`deploy_env` + API + role)** |
| git 操作 | 0件（read-only のみ）|
