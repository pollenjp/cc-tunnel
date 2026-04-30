# Terraform セットアップガイド

## 概要

cc-tunnel の GCP インフラは Terraform (Terragrunt) で管理されている。
本ドキュメントは初回セットアップ手順と既知の注意点を説明する。

## ディレクトリ構成

```
terraform/
├── root.hcl              # 共通設定（impersonation、provider 生成）
├── modules/
│   ├── prepare_terraform_sa/  # Terraform Runner SA とその IAM を管理
│   ├── artifact_registry/     # Artifact Registry リポジトリを管理
│   └── cc-tunnel/             # Cloud Build trigger + Cloud Run（cc-tunnel API + frontend）
├── prepare/
│   └── local/
│       └── terraform_sa/      # SA の apply（ADC 直接、impersonation なし）
└── live/
    └── local/
        ├── init/              # GCP API 有効化
        ├── artifact_registry/ # Artifact Registry リポジトリ作成
        └── cc-tunnel/         # cc-tunnel API + frontend デプロイ
```

## Apply 順序

必ず以下の順序で apply すること:

### 1. prepare/local/terraform_sa（ADC 直接で実行）

Terraform Runner SA を作成し、必要な IAM ロールを付与する。
この unit は impersonation を使わず殿の ADC 直接で実行される（root.hcl の例外条件）。

```bash
cd terraform/prepare/local/terraform_sa
terragrunt plan   # 差分確認（必須）
terragrunt apply
```

### 2. live/local/init（SA impersonation で実行）

GCP API（Artifact Registry、Compute Engine 等）を有効化する。

```bash
cd terraform/live/local/init
terragrunt apply
```

### 3. live/local/artifact_registry（SA impersonation で実行）

Artifact Registry リポジトリを作成する。

```bash
cd terraform/live/local/artifact_registry
terragrunt plan
terragrunt apply
```

### 4. live/local/cc-tunnel（SA impersonation で実行）

cc-tunnel API + frontend + Cloud SQL インスタンス（PostgreSQL）の Cloud Build trigger と
Cloud Run サービスを作成する。Cloud Build GitHub App connection が必要（手動設定）。
apply 後に `frontend_url` / `cc_tunnel_url` / `cloud_sql_instance_connection_name` が outputs に表示される。

```bash
cd terraform/live/local/cc-tunnel
terragrunt plan   # 差分確認（必須）
terragrunt apply
```

初回 apply 時に Cloud Build trigger が発火して Docker image がビルドされる。
Cloud Run サービスは image が push された後に起動可能となる。
初回 apply 後に cc-tunnel が起動すると goose によってスキーマが自動 migrate される。

## 前提条件と必要な権限

### 殿の ADC ユーザー（polleninjp@gmail.com）に必要なロール

| ロール | 用途 |
|--------|------|
| roles/iam.serviceAccountTokenCreator | SA の impersonation |
| roles/resourcemanager.projectIamAdmin | SA への IAM 付与（prepare 再 apply 時） |

### Terraform Runner SA に付与されるロール

`terraform/modules/prepare_terraform_sa/main.tf` の `sa_roles` で管理:

| ロール | 用途 |
|--------|------|
| roles/compute.admin | GCE VM 管理 |
| roles/compute.osLogin | GCE VM SSH |
| roles/iam.serviceAccountUser | VM へのSA アタッチ |
| roles/artifactregistry.admin | Artifact Registry 管理 |

## 既知の問題と対処

### IAM_PERMISSION_DENIED: artifactregistry.repositories.create

**原因**: Terraform Runner SA に `roles/artifactregistry.admin` が付与されていない。
`terraform/modules/prepare_terraform_sa/main.tf` の `sa_roles` にコメントアウトされた行がないか確認する。

**解決手順**:
1. `terraform/modules/prepare_terraform_sa/main.tf` で `roles/artifactregistry.admin` がコメントアウトされていれば解除
2. `prepare/local/terraform_sa` を再 apply（SA への権限追加）
3. `live/local/artifact_registry` を再 apply

### SA 名の変更を防ぐ

`random_string.unique_id` は terraform state で管理される。
state が正常であれば再 apply しても SA 名は変わらない。
`terragrunt plan` で差分を必ず確認すること。

## modules/cc-tunnel について

`terraform/modules/cc-tunnel/` は以下のリソースを管理する。
modules/cc-tunnel/ には cc-tunnel API、frontend（nginx + React SPA）、
Cloud SQL for PostgreSQL インスタンスが統合されており、cc-tunnel と同時に apply される。
DB パスワードは Secret Manager で管理され、Cloud Run には Cloud SQL Auth Proxy（Unix socket）で接続する。

**Cloud Build リソース:**
- Cloud Build BuilderSA (google_service_account)
- BuilderSA に logging.logWriter / AR writer 権限付与
- GitHub push trigger (google_cloudbuild_trigger, 1st gen)
- apply 後の初回ビルド実行 (terraform_data + local-exec gcloud)

**Cloud Run リソース (staged 追加):**
- Cloud Run ランタイム SA (google_service_account.runtime_sa)
- Cloud Run v2 サービス (google_cloud_run_v2_service.cloud_run)
- 全公開 IAM binding (google_cloud_run_v2_service_iam_member.public_access, `enable_public_access=true` 時のみ)

### 前提: Cloud Build GitHub App connection（手動操作必須）

1st gen の GitHub trigger を使う場合、事前に Cloud Build GitHub App のインストールが必要:
1. GCP Console > Cloud Build > Triggers > Manage repositories を開く
2. Cloud Build GitHub App をインストールし、cc-tunnel リポジトリを接続する

この接続は Terraform では自動化されていない。apply 前に手動で完了させること。

### Terraform Runner SA に必要な追加ロール

| ロール | 用途 |
|--------|------|
| roles/cloudbuild.builds.editor | Cloud Build trigger 作成・更新 + run/describe |
| roles/run.admin | Cloud Run サービスの作成・更新・削除 |

これらのロールは `terraform/modules/prepare_terraform_sa/main.tf` で管理される。

### modules/cc-tunnel に必要な GCP API

| API | 用途 |
|-----|------|
| `cloudbuild.googleapis.com` | Cloud Build trigger 管理 |
| `run.googleapis.com` | Cloud Run v2 サービス管理 |
| `iam.googleapis.com` | SA 作成 |
| `artifactregistry.googleapis.com` | イメージ pull |

`run.googleapis.com` は `terraform/modules/init_project/main.tf` の `activate_apis` で有効化が必要。

### modules/cc-tunnel の変数（live/local/cc-tunnel/terragrunt.hcl に追加が必要）

| 変数 | 用途 | terragrunt.hcl での指定例 |
|------|------|--------------------------|
| `deploy_env` | Cloud Run 名生成に使用（必須） | `include.root.locals.env` |
| `enable_public_access` | 全公開 IAM binding (bool, default: false) | 省略可 |
| `container_port` | コンテナ待受ポート (number, default: 5173) | 省略可 |

## frontend nginx reverse proxy の env 注入

`frontend.tf` の `fe_cloud_run` に以下の env が注入される:

| env 名 | 値 | 用途 |
|--------|-----|------|
| `API_UPSTREAM` | `google_cloud_run_v2_service.cloud_run.uri` | nginx が `/api/*` をプロキシする先の cc-tunnel URI |
| `BACKEND_URL` | `"/api"` | フロントエンドコードが使う API base path（同オリジン化済み） |
| `PORT` | `var.frontend_container_port` | nginx 待受ポート |

nginx は `apps/frontend/nginx.conf.template` の `envsubst` テンプレートで `$API_UPSTREAM` を展開し、
`/api/*` リクエストを cc-tunnel Cloud Run に転送する。circular dependency なし（frontend → cc-tunnel 片方向参照）。

## Cloud SQL 接続方式

| 項目 | 内容 |
|------|------|
| 接続方式 | Cloud Run の Cloud SQL connection（Unix socket `/cloudsql/INSTANCE_CONNECTION_NAME`）|
| DATABASE_URL | `postgres://USER:PASS@/DB?host=/cloudsql/INSTANCE_CONNECTION_NAME&sslmode=disable` |
| sslmode=disable の理由 | Cloud SQL Auth Proxy が TLS を担当するため |
| パスワード管理 | Secret Manager に自動保存（`random_password` で Terraform が生成） |

Cloud Run サービス定義の `cloud_sql_instances` に `INSTANCE_CONNECTION_NAME` を指定することで、
Cloud SQL Auth Proxy が自動的にサイドカーとして起動し Unix socket 経由の接続を提供する。

## マイグレーション適用方式

- **方式**: cc-tunnel 起動時に `goose embed` で自動適用（手動 migration 不要）
- **競合安全**: 複数 revision 同時起動時は `goose advisory lock` で競合を安全に解消

Cloud Run が新しい revision にトラフィックを切り替える際、複数インスタンスが同時起動しても
goose の advisory lock により migration は 1 度だけ実行される。

## Cloud SQL に関する注意事項

| ID | 内容 |
|----|------|
| C001 | POSTGRES_17 を使用（Cloud SQL の GA 版）。`compose/` や CI は `postgres:18-alpine` を使用しており差異がある |
| C002 | Cloud Run スケール時の Cloud SQL connection 数増加に注意（`max_connections` デフォルト 400）|
| C003 | `deletion_protection = false`（dev 用。本番では `true` を推奨）|

## Cloud Run v2 env 展開の落とし穴

Cloud Run v2 の env 定義に関して以下の制約に注意すること。

- `value` フィールド内の `$(VAR_NAME)` 記法は **static value（同じ containers ブロック内の他の value env）のみ** 参照可能
- `value_source.secret_key_ref` で注入される env は `$(VAR_NAME)` 展開の対象外（起動時の動的解決のため展開タイミングが異なる）
- `$(DB_PASSWORD)` のように secret_key_ref 由来の env を `value` 内で参照すると、リテラル文字列 `"$(DB_PASSWORD)"` がそのままアプリに渡され認証エラーになる

**正しい実装**: DATABASE_URL 全体（`postgres://user:pass@/db?host=...`）を Secret Manager に格納し、
`secret_key_ref` で `DATABASE_URL` env を直接注入すること。`random_password` には `special = false` を設定して
URL エンコードが不要な英数字のみのパスワードを生成すること。

## docker_gce Provider との関係

`cmd_cctunnel_docker_gce_impl` で実装された DockerGCEProvider は、
Artifact Registry に push された `cc-remote-agent` イメージを使用して
GCE VM 上でコンテナを起動する。

Artifact Registry セットアップが完了していることが docker_gce Provider
本番運用の前提条件となる。

## cc-remote-agent 統合（modules/cc-tunnel）

`modules/cc-tunnel/cc-remote-agent.tf` が cc-remote-agent の Cloud Build trigger と
Artifact Registry push を管理する。

### 構成方針

| 項目 | 内容 |
|------|------|
| イメージ管理 | Cloud Build trigger で `apps/cc-remote-agent/` 変更時に自動ビルド＆push |
| VM 管理 | GCE VM は Go コード（`dockergce/provider.go`）が動的作成（Terraform は VM resource を持たない） |
| GCE VM OS | COS（Go コードで hardcode。Terraform 設定不要） |
| デフォルト compute SA | `cra_default_compute_sa_reader` で `artifactregistry.reader` を付与（VM が AR から pull するため） |
| Cloud Run runtime SA | `cr_runtime_compute_admin`（roles/compute.instanceAdmin.v1）+ `cr_runtime_default_compute_sa_user`（roles/iam.serviceAccountUser）を付与 |

### Cloud Run env 変数

以下の env が Cloud Run に渡される:

| env 名 | 値 | 用途 |
|--------|----|------|
| `EXECUTION_PROVIDER` | `docker_gce` | 実行プロバイダー選択 |
| `GCE_PROJECT_ID` | `var.project_id` | VM 作成先プロジェクト |
| `GCE_ZONE` | `var.gce_zone` (default: `us-central1-a`) | VM 作成ゾーン |
| `GCE_MACHINE_TYPE` | `var.gce_machine_type` (default: `e2-medium`) | VM マシンタイプ |
| `CC_REMOTE_AGENT_IMAGE` | `local.cra_fqim` | AR 上の cc-remote-agent イメージ URL |

### outputs

`cc_remote_agent_image` output に cc-remote-agent の Artifact Registry イメージ URL が出力される。

### GCE ネットワークタグと Firewall ルール

DockerGCEProvider が動的作成する GCE VM には `cc-tunnel-agent` ネットワークタグが付与される。
このタグに対して以下の Firewall ルールが必要:

| ルール名 (例) | 方向 | ソース | ターゲットタグ | プロトコル/ポート | 用途 |
|---|---|---|---|---|---|
| `allow-docker-daemon` | ingress | cc-tunnel Cloud Run の IP レンジ（VPC 内部 IP）| `cc-tunnel-agent` | TCP/2375 | cc-tunnel → GCE VM Docker デーモンへの接続（暗号化なし、VPC 内部限定） |

**重要**: TCP/2375 は暗号化なし（TLS なし）の Docker デーモン API。**VPC 内部ネットワークかつ `cc-tunnel-agent` タグ付き VM のみに限定**すること。外部 IP からのアクセスを Firewall ルールで明示的に拒否すること。

Terraform での実装例（`modules/cc-tunnel/network.tf`）:

```hcl
resource "google_compute_firewall" "allow_docker_daemon" {
  name    = "${var.deploy_env}-allow-docker-daemon"
  network = "default"

  allow {
    protocol = "tcp"
    ports    = ["2375"]
  }

  # cc-tunnel Cloud Run は Serverless VPC Access Connector 経由で VPC に接続
  source_ranges = [var.vpc_connector_ip_range]
  target_tags   = ["cc-tunnel-agent"]

  description = "Allow cc-tunnel to reach GCE VM Docker daemon (internal only)"
}
```

GCE VM 作成時のネットワークタグ指定は `dockergce/provider.go` の `createGCEVM` 関数内で設定される。

## Phase 2: External Global HTTPS LB + serverless NEG 構成

Phase 2 では External Global HTTPS LB を採用し、cc-tunnel と frontend を
単一ドメイン（cctunnel.pollenjp.com）に統合する:
- LB の path matcher: /api/* → cc-tunnel backend（url_rewrite で /api prefix を削除）
- LB の default: frontend backend
- cc-tunnel/frontend ともに ingress=INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER（LB 経由のみ）
- SSL: Google-managed SSL certificate（DNS 認証、cctunnel.pollenjp.com）

### アーキテクチャ

```
Browser → DNS (cctunnel.pollenjp.com → LB IP) → Global HTTPS LB
          ├── /api/* → cc-tunnel Cloud Run (path rewrite: /api を削除、timeout=3600s for SSE)
          └── /*    → frontend Cloud Run (SPA + nginx try_files)
```

### Apply 後の手順（殿の作業）

1. `terragrunt apply` で Global LB / managed cert を一括作成
   （cert は PROVISIONING 状態で apply 完了）
2. outputs の `lb_ip` を確認し、Cloudflare で A レコード設定:
   - Record: `cctunnel.pollenjp.com` → `<lb_ip>`
   - Proxy: DNS only（灰色雲、Proxy OFF）
3. DNS 伝搬後、Google が cert を発行（数十分〜数時間）
4. cert の ACTIVE を確認:
   ```bash
   gcloud compute ssl-certificates describe <cert-name> \
     --global --format="value(managed.status)"
   ```
   cert-name は `${deploy_env}-${random_id}-lb-cert` 形式。apply 後 `terragrunt output` でリソース名を確認すること。
   `ACTIVE` になれば `https://cctunnel.pollenjp.com/` でアクセス可能

### Terraform 変数

| 変数 | 値 | 説明 |
|------|----|------|
| `lb_fqdn` | `cctunnel.pollenjp.com` | SSL cert のドメイン |

### 注意事項

- serverless NEG 経由（Global LB）でも Cloud Run の IAM invoker チェックは有効。
  `ingress=INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER` で `.run.app` 直接アクセスをブロックしつつ、
  `allUsers` に `roles/run.invoker` を付与して LB からのアクセスを許可すること（`enable_public_access=true`）。
  Phase 3 で IAP を有効化する際は `allUsers` invoker を削除し、IAP サービスアカウントに invoker を付与する。
- cert が ACTIVE になるまでは HTTPS アクセス不可（ERR_SSL_PROTOCOL_ERROR）
- ingress=INTERNAL_LOAD_BALANCING 切替後、Cloud Run の `.run.app` URL への直接アクセスは拒否される
- ローカル開発（compose.yaml）は引き続き nginx の /api/ proxy_pass で動作（Cloud Run 側の変更は影響なし）
- serverless NEG を持つ backend_service は `timeout_sec` 非対応（apply エラー）。
  リクエスト timeout は Cloud Run の `template.timeout` で制御する
  （cc-tunnel デフォルト 300s、変更が必要な場合は main.tf の timeout_seconds を調整）

### Phase 3 IAP 移行時の差分

- backend service に `iap { }` ブロックを追加するだけで IAP 有効化可能（LB 構成の変更最小）
- AppAuth との責務整理（IAP 一本化 or 並走）は Phase 3 cmd で別途決定
