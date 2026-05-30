# terraform/ → terraform2/ (Terragrunt Stacks) 移行プラン

`terraform/`（ディレクトリ毎に `terragrunt.hcl` を置くクラシック構成）を
**Terragrunt Stacks**（`terragrunt.stack.hcl` で unit を宣言生成する構成）へ
書き換えた新ディレクトリが `terraform2/` である。

既に GCP 上にリソースは存在するため、本移行の最優先事項は
**「リソースを一切再作成しない（`terragrunt plan` が無変更になる）」**こと。

---

## 1. 設計の核心 — なぜ state 移動ゼロで移行できるのか

Stacks では unit が `.terragrunt-stack/<path>/` に生成され、
`path_relative_to_include()` はその**生成パス**を返す。素朴に使うと backend の
state prefix が変わり（例: `live/local/.terragrunt-stack/cc-tunnel`）、Terraform は
全リソースを「新規」とみなして再作成しようとする。

`terraform2/root.hcl` は生成パスから `.terragrunt-stack` セグメント（および
Terragrunt が前後に挿入する空セグメント）を取り除いた**論理パス**を計算し、
それを state prefix と impersonation 例外判定に使う。

```
live/local//.terragrunt-stack//cc-tunnel    -> live/local/cc-tunnel
prepare/local//.terragrunt-stack//terraform_sa -> prepare/local/terraform_sa
```

結果、`terraform2/` の各 unit は **classic と完全に同一の** GCS バケット
（`local-gsaq-tfstate`）・同一 prefix を指すため、**同じ state オブジェクトを共有**する。
モジュール・inputs も同一なので `plan` は無変更になる。

> 採用しなかった代替案: 新しい state キーへ `gsutil cp` / `terraform state push`
> で移行する方法。キーは綺麗になるが unit 毎の移行手順とミスのリスクが増える。
> 本移行では「再作成しないこと」を最優先し、prefix 共有方式を採用した。

### 検証済みの事実（terragrunt v1.0.2 でローカル確認）

`terragrunt render --json` で classic と terraform2 の生成結果を比較した結果:

| 項目 | 結果 |
|---|---|
| backend (`bucket` / `prefix`) + provider (`impersonate_service_account`) | 全 unit で**バイト単位一致** |
| `inputs` | 一致 |
| `terraform.source` の解決先モジュールディレクトリ | 同一 |
| `prepare/local/terraform_sa` の impersonation | classic 同様 **無効**（ADC 直接） |
| `terragrunt hcl validate` / `terragrunt hcl fmt --check` | パス |

---

## 2. 新ディレクトリ構成

```
terraform2/
├── root.hcl                      # backend/provider/outputs を generate（論理パス方式）
├── mise.toml                     # stack 向け task（stack generate / stack run）
├── .gitignore                    # .terragrunt-stack / .terragrunt-cache を無視
├── MIGRATION.md                  # 本ファイル
├── catalog/
│   └── units/                    # 再利用可能な unit 定義（classic の各 terragrunt.hcl 相当）
│       ├── init/terragrunt.hcl
│       ├── artifact_registry/terragrunt.hcl
│       ├── cc-tunnel-iap/terragrunt.hcl
│       ├── cc-tunnel/terragrunt.hcl
│       ├── vm_image_cleaner/terragrunt.hcl
│       ├── prepare_terraform_sa/terragrunt.hcl
│       └── prepare_tfstate_bucket/terragrunt.hcl
├── live/
│   └── local/
│       └── terragrunt.stack.hcl  # init / artifact_registry / cc-tunnel-iap / vm_image_cleaner / cc-tunnel
└── prepare/
    └── local/
        └── terragrunt.stack.hcl  # terraform_sa / tfstate_bucket
```

- **modules は当面 `terraform/modules/` を参照**する（root.hcl の
  `modules_base_dir`）。単一ソースで drift を避け、移行完了時の最終ステップで
  `terraform2/modules/` へ移動する（→ 4章 STEP 5）。
- `terragrunt.stack.hcl` の `path` は classic のディレクトリ名と一致させてある
  （`cc-tunnel`, `terraform_sa` など）。**この一致が state prefix 一致の前提**なので変更しないこと。
- 依存関係は各 catalog unit の `dependency` ブロックで宣言（生成後は
  `.terragrunt-stack/` 内で兄弟配置になり `../init` 等の相対パスが解決される）。

依存グラフ（classic と同一）:

```
prepare:  terraform_sa,  tfstate_bucket            (bootstrap, 独立)
live:     init ─┬─ artifact_registry ─┐
                ├─ cc-tunnel-iap ──────┼─► cc-tunnel
                └─ vm_image_cleaner
```

---

## 3. 前提条件

- `mise install`（`terraform2/mise.toml`: terragrunt 1.0.2 / terraform 1.14.9 / gcloud / op_run）
- GCP ADC（`gcloud auth application-default login`）と、Terraform Runner SA を
  impersonate できる権限（`roles/iam.serviceAccountTokenCreator`）
- 1Password の env template（`terraform2/.env.template` をローカル作成。classic の
  `terraform/.env.template` と同等。`CLOUDFLARE_ZONE_ID` / `IAP_OAUTH_CLIENT_ID` /
  `IAP_OAUTH_CLIENT_SECRET` 等を含む）
- **重要**: 移行期間中、classic `terraform/` と `terraform2/` は**同じ state を共有**する。
  両方から同時に `apply` しない（state ロックが競合する）。

---

## 4. 移行ランブック

### STEP 0. ブランチ作成・レビュー
`terraform2/` を追加する PR をレビューする（リソース変更は伴わない＝追加のみ）。

### STEP 1. stack 生成
```bash
cd terraform2/live/local    && terragrunt stack generate
cd terraform2/prepare/local && terragrunt stack generate
```
`.terragrunt-stack/` が生成される（gitignore 済み）。
`terragrunt stack run` を使う場合はこの手順は自動実行される。

### STEP 2. ★ no-diff ゲート（最重要）
**全 unit で `plan` が「No changes」になることを確認する。** これが移行可否の判断基準。
```bash
# live
cd terraform2/live/local
op_run -e .env.template -- terragrunt stack run -- plan      # or: mise run tf:plan:all

# prepare（bootstrap。通常は無変更のはず）
cd terraform2/prepare/local
op_run -e .env.template -- terragrunt stack run -- plan
```
- いずれかの unit に差分が出たら **apply せず原因を調査**する
  （prefix 不一致 / inputs 差異 / モジュール差異のいずれか）。
- `cc-tunnel` unit は `artifact_registry` / `cc-tunnel-iap` の output に依存するため、
  単体 plan 時は依存 unit が apply 済み（= 既存 state）であることが前提。

### STEP 3. 運用を terraform2/ へ切替
no-diff を確認できたら、以降の apply は **terraform2/ から実行**する。
classic `terraform/` の使用を停止する（削除はまだしない＝ロールバック余地を残す）。

```bash
cd terraform2/live/local
op_run -e .env.template -- terragrunt stack run -- apply     # or: mise run tf:apply:all
```

apply 順序は dependency ブロックにより自動解決される（classic の
`docs/terraform-setup.md` の順序と同じ）。

### STEP 4. しばらく terraform2/ で運用し安定を確認
通常運用（plan/apply）を terraform2/ で回し、問題がないことを確認する。

### STEP 5. modules を terraform2/ へ移動（カットオーバー）
```bash
git mv terraform/modules terraform2/modules
```
`terraform2/root.hcl` の 1 行だけ変更:
```hcl
# before
modules_base_dir = "${get_repo_root()}/terraform/modules"
# after
modules_base_dir = "${get_repo_root()}/terraform2/modules"
```
再度 STEP 2 の no-diff ゲートを実行（モジュール内容は不変なので無変更のはず）。

### STEP 6. classic terraform/ を削除
```bash
git rm -r terraform/
```
- `.github/workflows/ci.yml` の `validate-terraform` ジョブの
  `working-directory` を `terraform2` に変更（または旧ジョブを削除し
  `validate-terraform2` に一本化）。
- `docs/terraform-setup.md` / `docs/directory-structure.md` のパスを更新。
- 最終的に `terraform2/` を `terraform/` にリネームするかは任意
  （リネームしても state prefix は `live/local/...` のままなので影響なし。
  ただし `modules_base_dir` と CI / docs のパスを合わせて更新すること）。

---

## 5. ロールバック

`terraform2/` は既存 state を**共有**するだけで、リソースの移動・破棄を行わない。

- STEP 1〜4 の段階: 何もせず classic `terraform/`（無傷で残存）から運用に戻すだけ。
- STEP 5（modules 移動）後: `modules_base_dir` を戻し `git mv` を revert。
- STEP 6（terraform/ 削除）後: revert で復元可能（state は終始無変更）。

破壊的操作は STEP 5/6 のみ。**確信を得てから最後に実施**すること。

---

## 6. 既知の注意点・フォローアップ

- **`.terraform.lock.hcl`**: classic は unit 毎に commit していたが、Stacks では
  生成先（`.terragrunt-stack/`、gitignore 済み）に置かれるため commit されない。
  provider バージョンは `root.hcl` の `required_providers` で固定済みのため実害は小さい。
  厳密に固定したい場合は catalog unit 配下に `.terraform.lock.hcl` を置けば
  生成時にコピーされる（フォローアップ）。
- **同時 apply 禁止**: 移行期間は classic と terraform2 が同一 state を共有する。
  両方から同時に apply しないこと。
- **env-specific inputs の values 化（将来）**: 現状 catalog unit は classic と
  完全同一にするため env 固有値（`lb_fqdn`, `iap_allowed_members` 等）をインラインで
  保持している。`live/dev/terragrunt.stack.hcl` のような multi-env 展開を行う際は、
  これらを `terragrunt.stack.hcl` の `values` 経由で渡す形に切り出すと DRY になる
  （`root.hcl` の env 検出機構はそのまま流用可能）。
- **CI**: `terraform2/` 用に `terragrunt hcl fmt --check` / `terragrunt hcl validate`
  を回すジョブを追加済み（`.github/workflows/ci.yml` の `validate-terraform2`）。
