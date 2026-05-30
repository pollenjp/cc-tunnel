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
| roles/iap.admin | IAP backend service の IAM policy 操作 (Phase 3 IAP) |

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
| `github_owner` | 全 Cloud Build trigger の owner (default: `pollenjp`) | 省略可 |
| `github_repo_name` | 全 Cloud Build trigger の repo 名 (default: `cc-tunnel`) | 省略可 |
| `github_branch_name` | ビルドを発火させる push 対象ブランチ (default: `main`) | 省略可 |
| `cc_tunnel_dockerfile_dir` | cc-tunnel Dockerfile ディレクトリ (default: `apps/cc-tunnel`) | 省略可 |

> GitHub trigger の source（owner / repo / branch / Dockerfile dir）は commit 0e7f875 (#71) で
> module 変数に切り出され、cc-tunnel / frontend / cc-remote-agent / container-manager / vm-image の
> 全 Cloud Build trigger がこれらの変数を参照する。デフォルト値が従来のハードコード値と一致するため、
> 通常は terragrunt.hcl で上書き不要。

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
| `GCE_VM_IMAGE` | `local.vm_image_url` | Packer で焼いた VM イメージの URL |
| `GCE_VM_SERVICE_ACCOUNT` | `google_service_account.vm_runtime_sa.email` | VM にアタッチする SA。AR reader / logging / monitoring の最小権限 |
| `GCE_VM_SUBNETWORK` | `google_compute_subnetwork.cc_remote_agent_vm.id` | VM をぶら下げる subnet（カスタム VPC の `cc-remote-agent-vm`）。VM は ephemeral external IP を付与され、AR / 外部依存の pull は public 経路を通る（Private Google Access は不要）。詳細は ADR `2026-05-09T11:50:55+09:00_01_gce_vm_egress_via_external_ip.md` |
| `CC_REMOTE_AGENT_IMAGE` | `local.cra_fqim` | AR 上の cc-remote-agent イメージ URL |

### outputs

`cc_remote_agent_image` output に cc-remote-agent の Artifact Registry イメージ URL が出力される。

### カスタム VPC（`vpc.tf`）

cc-tunnel は default VPC に依存せず、専用のカスタム VPC を作成する（commit 7586d56, #60）。
`vpc.tf` で以下を管理する:

| リソース | 内容 |
|---|---|
| `google_compute_network.cc_tunnel` | カスタム VPC（`var.network_name`、default `cc-tunnel`、`auto_create_subnetworks=false`、`REGIONAL`）|
| `google_compute_subnetwork.cc_tunnel_egress` | Cloud Run Direct VPC egress 用 subnet（`cc-tunnel-egress`、CIDR=`var.vpc_connector_subnet_cidr`、最小 /28）。cc-tunnel が GCE VM 内部 IP の container-manager API に到達するために使用 |
| `google_compute_subnetwork.cc_remote_agent_vm` | cc-remote-agent VM 用 subnet（`cc-remote-agent-vm`、CIDR=`var.cc_remote_agent_subnet_cidr`、default `10.16.0.0/20`）|

VM は ephemeral external IP（`ONE_TO_ONE_NAT`）を持つため、Artifact Registry / 外部依存の pull は public 経路を通る。Private Google Access は不要。

### GCE ネットワークタグと Firewall ルール（`firewall.tf`）

DockerGCEProvider が動的作成する GCE VM には `cc-tunnel-agent` ネットワークタグが付与される
（タグ指定は `dockergce/provider.go` の `createGCEVM` 関数内）。`firewall.tf` は以下のルールを管理する:

| ルール名 | 方向 | ソース | ターゲットタグ | プロトコル/ポート | 用途 |
|---|---|---|---|---|---|
| `cc-tunnel-container-manager` | ingress | `var.vpc_connector_subnet_cidr`（Cloud Run Direct VPC egress subnet）| `cc-tunnel-agent` | TCP `container_manager_port`（default 9090）+ `cc_remote_agent_host_port_start`-`cc_remote_agent_host_port_end`（default 61000-61999）| cc-tunnel → container-manager API + 各 cc-remote-agent コンテナの公開ホストポート（`/auth/status` 等のポーリング）|
| `cc-tunnel-iap-ssh`（任意）| ingress | `35.235.240.0/20`（IAP TCP forwarding レンジ）| `cc-tunnel-agent` | TCP/22 | デバッグ用 SSH。`var.enable_ssh_debug=true`（default false）時のみ作成。IAP 経由のみ許可され、外部 IP 直接 SSH は不可 |

**重要**:
- dockerd は Unix socket にバインドされ**ネットワーク到達不可**。VM 上の操作は container-manager HTTP API（旧 TCP/2375 Docker デーモン直叩きは廃止）経由で行う。
- container-manager API・agent ホストポートへの ingress は **Cloud Run Direct VPC egress subnet（`vpc_connector_subnet_cidr`）かつ `cc-tunnel-agent` タグ付き VM のみ**に限定される。
- agent ホストポートレンジは `dockergce/provider.go` の `PortRangeStart`/`PortRangeEnd` 定数と一致させること。

## ログ集約 (Cloud Logging / Ops Agent)

GCE 側のログを Cloud Logging に一元集約する（commit 9ca2e50, #72）。

| レイヤ | 仕組み |
|---|---|
| Cloud Build trigger（cc-tunnel / frontend / cc-remote-agent / container-manager / vm-image）| ビルドステップの `options { logging = "CLOUD_LOGGING_ONLY" }` で Cloud Logging のみに出力（GCS ログバケット不要） |
| GCE VM の dockerd | Packer イメージ（`apps/vm-image/packer.pkr.hcl`）で `daemon.json` に `"log-driver": "gcplogs"` を設定し、コンテナ stdout/stderr を Cloud Logging へ送る |
| container-manager コンテナ | systemd unit の `docker run` に `--log-driver=gcplogs --log-opt labels=component --label component=container-manager` を付与 |
| Ops Agent | Packer ビルド時に Google Cloud Ops Agent をインストール（`add-google-cloud-ops-agent-repo.sh --also-install`）し、VM 本体のシステムログ/メトリクスを収集 |

VM ランタイム SA には `roles/logging.logWriter` 等の最小権限が付与される（`gce_vm_sa.tf` / 上記 env テーブルの `GCE_VM_SERVICE_ACCOUNT`）。

## VM リープ (Cloud Scheduler safety-net)

docker_gce の GCE VM は二経路でリープ（自動削除）される。設計詳細は
`docs/docker-gce-design.md` §5.2 と ADR `2026-05/2026-05-20T20:46:00+09:00_01_vm_reap_dual_path.md` を参照。

| 経路 | 主体 | 間隔 | 役割 |
|---|---|---|---|
| 主経路 | VM 上の container-manager 自己リーパー（`SELF_REAP_*` env、systemd unit で有効化）| 10 分 | agent コンテナ数が 0 を観測し続けたら VM 自身を削除 |
| safety-net | Cloud Scheduler → cc-tunnel `POST /internal/reconcile-vms`（`scheduler.tf`）| 6 時間 | 自己リーパーが死んだ（OOM / systemd 失敗等）VM を回収 |

`scheduler.tf` が管理する safety-net 経路の Terraform リソース:

- `google_cloud_scheduler_job.reconcile_vms`: `schedule = "0 */6 * * *"`、`POST <cloud_run_uri>/internal/reconcile-vms`
- `google_service_account.scheduler_sa` + `google_cloud_run_v2_service_iam_member.scheduler_invoker`: Scheduler 用 SA に Cloud Run invoker を付与
- 認証: Cloud Scheduler が OIDC ID トークンを署名（`aud=cc-tunnel-reconcile-vms` の静的 custom audience）。cc-tunnel は `idtoken.Validate` で検証し、email を `RECONCILE_VMS_ALLOWED_EMAILS` と照合する（`main.tf` で Scheduler SA を設定）

> audience に Cloud Run URL ではなく静的文字列を使うのは、env var ⇔ `cloud_run.uri` 間の terraform 循環依存を避けるため。

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
  `ingress=INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER` で `.run.app` 直接アクセスをブロックする。
  IAP 無効で LB 直公開する場合のみ `allUsers` に `roles/run.invoker` を付与する（`enable_public_access=true`、default false）。
  IAP 有効化後 (Phase 3) は `allUsers` invoker を撤去し IAP P4SA のみに invoker を付与する
  defense-in-depth 構成に移行済み（commit d9bd840, Issue #62 クローズ）。
- cert が ACTIVE になるまでは HTTPS アクセス不可（ERR_SSL_PROTOCOL_ERROR）
- ingress=INTERNAL_LOAD_BALANCING 切替後、Cloud Run の `.run.app` URL への直接アクセスは拒否される
- ローカル開発（compose.yaml）は引き続き nginx の /api/ proxy_pass で動作（Cloud Run 側の変更は影響なし）
- serverless NEG を持つ backend_service は `timeout_sec` 非対応（apply エラー）。
  リクエスト timeout は Cloud Run の `template.timeout` で制御する
  （cc-tunnel デフォルト 300s、変更が必要な場合は main.tf の timeout_seconds を調整）

## Phase 3: Identity-Aware Proxy (IAP) — LB backend service レベル

Phase 3 では LB の両 backend service (cc-tunnel / frontend) に IAP を被せ、
Google アカウントログイン + 許可リスト方式で外部公開を絞る。
AppAuth との責務分離: IAP は「アプリに到達する前段の認証ゲート」、AppAuth は
アプリ内部の RBAC / セッション管理という二段構成で並走する。

### アーキテクチャ

```
Browser ─▶ Global HTTPS LB
            └── backend_service (cc-tunnel / frontend)
                  └── iap { enabled = true, oauth2_client_id, oauth2_client_secret }
                        ├── 認証 ─▶ Google OAuth (Console 作成済みの brand + client)
                        └── 認可 ─▶ roles/iap.httpsResourceAccessor の IAM members
                                   (var.iap_allowed_members)
                  └── (LB → Cloud Run): IAP P4SA
                        service-<PROJECT_NUMBER>@gcp-sa-iap.iam.gserviceaccount.com
                        が roles/run.invoker で Cloud Run を呼び出す
```

### 重要: IAP 関連 API の deprecation

- `google_iap_brand` / `google_iap_client` は **2025-01-22 deprecate**、
  裏側の "IAP OAuth Admin APIs" は **2026-03-19 に shutdown 済み**。
- Terraform からは brand / OAuth client を作れないため、両方とも **GCP Console で手動作成** する。
- 本リポジトリの `cc-tunnel-iap` モジュールは「Console 作成済みの client_id / client_secret を
  入力として受け取り、cc-tunnel モジュールに output として渡すだけの thin passthrough」になっている。

### Console 手順 (一度だけ)

1. **OAuth consent screen**: APIs & Services > OAuth consent screen
   - 個人プロジェクト (組織なし) では User type=External を選択
   - Application title / Support email を設定して作成
   - Test users に IAP 経由でアクセスする Google アカウントを追加
     (Publishing status=Testing のままなら必須)
2. **OAuth client**: APIs & Services > Credentials > Create credentials > OAuth client ID
   - Application type=Web application で作成
   - 作成直後にダイアログで Client ID が表示されるので控えておく
   - Authorized redirect URIs に以下を追記して保存:
     `https://iap.googleapis.com/v1/oauth/clientIds/<CLIENT_ID>:handleRedirect`
3. 控えた Client ID / Client secret を環境変数に export:
   ```bash
   export IAP_OAUTH_CLIENT_ID=<...>.apps.googleusercontent.com
   export IAP_OAUTH_CLIENT_SECRET=<...>
   ```

### Terragrunt unit と変数

`terraform/live/local/cc-tunnel-iap/` が独立 unit として存在し、
`cc-tunnel` unit はそこに dependency する。

| unit / 変数 | 値 | 説明 |
|------|----|------|
| `cc-tunnel-iap` / `oauth_client_id` | `IAP_OAUTH_CLIENT_ID` env var | Console 作成済みの OAuth client ID |
| `cc-tunnel-iap` / `oauth_client_secret` | `IAP_OAUTH_CLIENT_SECRET` env var | OAuth client secret (sensitive) |
| `cc-tunnel` / `iap_enabled` | `true` で IAP を有効化 | false 時は backend service の iap block も IAM binding も全部 noop |
| `cc-tunnel` / `iap_allowed_members` | `["user:foo@example.com", ...]` | `roles/iap.httpsResourceAccessor` を付与する IAM principals |
| `cc-tunnel` / `iap_oauth_client_id` | `dependency.cc_tunnel_iap.outputs.oauth_client_id` | dependency 経由で自動配線 |
| `cc-tunnel` / `iap_oauth_client_secret` | `dependency.cc_tunnel_iap.outputs.oauth_client_secret` | 同上 |

### Apply 順序と必要ロール

1. `terraform/prepare/local/terraform_sa/` を再 apply (人間アカウントで)
   - Runner SA に `roles/iap.admin` を付与する。これが無いと
     `google_iap_web_backend_service_iam_member` が IAM policy 取得時に 403 になる。
2. `terraform/live/local/cc-tunnel-iap/` を apply
3. `terraform/live/local/cc-tunnel/` を apply
   - `iap_enabled=true` の場合、以下が同時に作成される:
     - backend service の `iap { ... }` block (lb.tf)
     - `google_project_service_identity.iap` (IAP P4SA をプロビジョニング)
     - `google_cloud_run_v2_service_iam_member` × 2 (P4SA に Cloud Run invoker)
     - `google_iap_web_backend_service_iam_member` × N (許可ユーザに accessor)

### 認証 / 認可の評価順 (AND)

ブラウザがアクセスして以下の **すべて** を満たす必要がある:

1. **OAuth Test users** (Publishing=Testing 時のみ): Console 側のリストに登録済み
2. **IAM `roles/iap.httpsResourceAccessor`**: `iap_allowed_members` に `user:foo@example.com` 形式で含まれる
3. **AppAuth** (アプリ層): IAP を抜けた後、cc-tunnel 内の RBAC で許可

両者を別々に管理する必要がある点に注意:
- OAuth Test users は生メール (`foo@gmail.com`)
- `iap_allowed_members` は IAM principal (`user:foo@gmail.com`)

### よくあるエラー

| エラー | 原因 | 解決 |
|------|------|------|
| `"Error retrieving IAM policy ... 403"` (terraform plan/apply) | runner SA に `roles/iap.admin` が無い | `terraform/prepare/local/terraform_sa/` を再 apply |
| `"The IAP service account is not provisioned"` (ブラウザログイン後) | IAP P4SA が project に未作成 / Cloud Run invoker 未付与 | `iap_enabled=true` で `cc-tunnel` を apply (本リポジトリでは `google_project_service_identity.iap` + invoker IAM が自動で入る) |
| `"Access blocked: <app> has not completed the Google verification process"` (Google ログイン画面) | Publishing=Testing で Test users 未登録 | OAuth consent screen の Test users にアカウント追加 (or Publish) |
| `"403"` を IAP が返す (consent 後) | `iap_allowed_members` 未設定 / アカウントが含まれない | `iap_allowed_members` に `user:<email>` を追加 |

### defense-in-depth (Issue #62, 対応済み)

IAP が backend service レベルで効いた上で、Cloud Run service の invoker も
IAP P4SA のみに絞る defense-in-depth 構成が完了している（commit d9bd840）:

1. `google_cloud_run_v2_service_iam_member.public_access` (allUsers) は `var.enable_public_access`
   が `true` の場合のみ作成され、IAP 運用時（default）は付与されない（`main.tf`）
2. `iap.tf` で IAP P4SA に `roles/run.invoker` を付与（`var.iap_enabled=true` 時）

`ingress=INTERNAL_LOAD_BALANCER` と合わせ、`.run.app` への直接アクセス・LB 経由の未認証アクセスの双方を遮断する。
allUsers の撤去と P4SA への付与は 1 PR で行い（順序を逆にすると一瞬全リクエスト 403 になる）、
terraform に dependency 解決を任せる。

