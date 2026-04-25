# cc-tunnel アーキテクチャ

## 概要

`cc-tunnel` は外部クライアントからの HTTP リクエストを受け取り、Claude Code を実行する内部エージェント (`cc-remote-agent`) に転送するプロキシサーバーである。

### per-session isolation（LocalDockerProvider）

`local` ExecutionProvider を使用する場合、cc-tunnel は**会話ごとに独立した Docker コンテナ**を起動してリクエストを処理する。

- **Auth 分離**: 認証処理は `cc-remote-agent-auth`（常駐1台）が専任で担当する
- **実行分離**: 各会話セッションは専用コンテナ (`cctunnel-session-{convID[:8]}`) で隔離される
- **Graceful shutdown**: SIGTERM 受信時に `SessionManager` が全コンテナを停止・削除する

## コンポーネント図

```
外部クライアント
      │ HTTP
      ▼
  cc-tunnel
  ├── api.Handler
  │     ├── Auth: cc-remote-agent-auth（常駐コンテナ）
  │     └── Execution: ExecutionProvider
  │           ├── local         → LocalDockerProvider
  │           ├── cloud_run_sandbox → CloudRunSandboxProvider
  │           └── docker_gce    → DockerGCEProvider
  └── db: PostgreSQL（会話履歴）
```

## ExecutionProvider

`ExecutionProvider` インターフェースは、Claude Code の実行環境を抽象化する。

| 環境変数 `EXECUTION_PROVIDER` | 実装 | 概要 |
|-------------------------------|------|------|
| `local` | `LocalDockerProvider` | per-session Docker コンテナ（ローカル Docker） |
| `cloud_run_sandbox` | `CloudRunSandboxProvider` | Google Cloud Run Sandbox |
| `docker_gce` | `DockerGCEProvider` | GCE 上の Docker |

### LocalDockerProvider

`local` プロバイダーは**会話ごとに独立した Docker コンテナ**を起動する実装である。

| 項目 | 値 |
|------|----|
| 実装ファイル | `internal/provider/local/docker_provider.go` |
| コンテナイメージ | `cc-remote-agent:latest`（環境変数 `CC_REMOTE_AGENT_IMAGE` で上書き可） |
| コンテナ命名規則 | `cctunnel-session-{convID[:8]}` |
| ネットワーク | `apps_default`（compose ネットワーク、DNS 名前解決）|
| ボリューム | `claude-sessions`（`/home/user/.claude` にマウント、auth 状態共有）|
| Docker 操作 | Docker SDK (`github.com/docker/docker`) — distroless 環境のため CLI 不使用 |

#### SessionManager ライフサイクル

`SessionManager`（`internal/docker/session_manager.go`）がコンテナのライフサイクルを管理する。

```
会話リクエスト受信
      │
      ▼
GetOrCreate(convID)
      │
      ├── キャッシュ HIT かつ Running → 再利用
      └── ミス or 停止済み → コンテナ起動
            │
            ├── ContainerCreate（命名: cctunnel-session-{convID[:8]}）
            ├── ContainerStart
            ├── ヘルスチェック（GetAuthStatus、500ms ポーリング、最大30秒）
            └── アイドルタイマー設定（デフォルト: 15分）
```

3層クリーンアップ:

| タイミング | 処理 | 方法 |
|-----------|------|------|
| アイドル時 | 単一コンテナ停止・削除 | `idleTimer`（15分）→ `Stop(convID)` |
| SIGTERM 受信 | 全管理コンテナ停止・削除 | `StopAll()` → `io.Closer.Close()` |
| 起動時 | 孤児コンテナ削除 | `CleanupOrphans()`（`cctunnel-session-*` の非 Running を削除）|

## インフラ構成

### compose.yaml（`apps/compose.yaml`）

本番・開発環境の起動に使用するメイン compose ファイル。

```
services:
  postgres            - 会話履歴 DB
  cc-remote-agent-auth - Auth 専用常駐コンテナ（cc-remote-agent:latest イメージ使用）
  cc-tunnel           - プロキシサーバー（Docker socket マウント、EXECUTION_PROVIDER=local）
  frontend            - フロントエンド

volumes:
  pgdata              - PostgreSQL データ
  claude-sessions     - Claude 認証状態（cc-remote-agent-auth および per-session コンテナが共有）
```

主な設定:

- `cc-tunnel` は `/var/run/docker.sock` をマウントし、Docker SDK でコンテナを操作する
- `cc-remote-agent-auth` は `cc-remote-agent:latest` イメージを使用（ビルド設定なし）
- ネットワーク名 `apps_default` は `name: apps` 宣言により固定される

### prepare.compose.yaml（`apps/prepare.compose.yaml`）

`cc-remote-agent:latest` イメージのビルド専用ファイル。`compose.yaml` の起動前に実行する。

```bash
# イメージビルド
docker compose -f apps/prepare.compose.yaml build

# 通常起動
docker compose -f apps/compose.yaml up
```

## ローカル開発手順

### 起動方法（DooD モード）

cc-tunnel を Docker コンテナ内から起動するか、ホストで起動するかに関わらず、
docker.sock マウント（DooD: Docker-out-of-Docker）により docker 操作が可能。
URL 生成は常に Docker ネットワーク DNS 方式を使用。

```bash
# 1. インフラ起動（postgres + cc-remote-agent-auth）
cd apps
mise run dev:up

# 2. cc-tunnel を起動（別ターミナル）
cd apps/cc-tunnel
mise run cc-tunnel:dev:up

# 3. frontend を起動（別ターミナル）
cd apps/frontend
mise run frontend:dev:up
```

#### 環境変数（cc-tunnel:dev:up で自動設定）

| 変数 | 値 | 説明 |
|------|-----|------|
| EXECUTION_PROVIDER | local | SessionManager 経由でコンテナ起動 |
| DOCKER_NETWORK | apps_default | Docker ネットワーク名（DNS 解決に使用） |
| CLAUDE_SESSIONS_VOLUME | claude-sessions | 認証情報共有ボリューム |
| CC_REMOTE_AGENT_PORT | 9091 | cc-remote-agent Listen ポート |
| DATABASE_URL | postgres://...@localhost:5432/... | ローカル postgres |

#### DooD（Docker-out-of-Docker）の動作

cc-tunnel は /var/run/docker.sock をマウントすることで、
コンテナ内からでも docker SDK 操作が可能。
SessionManager はコンテナ起動後、常に DNS URL を使用して接続:
`http://containerName:containerPort`

#### 全 Docker モード（CI・デモ用）

```bash
cd apps
docker compose -f prepare.compose.yaml build
docker compose --profile full up -d
```

## 主要ファイル一覧

| ファイル | 役割 |
|---------|------|
| `cmd/cc-tunnel/main.go` | エントリーポイント、プロバイダー選択、SIGTERM ハンドラ、孤児クリーンアップ |
| `internal/api/handler.go` | HTTP ハンドラ、ConversationID セット |
| `internal/docker/runner.go` | `DockerRunner` インターフェース定義 |
| `internal/docker/sdk_runner.go` | Docker SDK 実装（`SDKRunner`） |
| `internal/docker/session_manager.go` | コンテナライフサイクル管理（`SessionManager`） |
| `internal/provider/local/docker_provider.go` | `LocalDockerProvider`（`ExecutionProvider` 実装） |
| `internal/remoteclient/client.go` | cc-remote-agent HTTP クライアント（`ConversationID` フィールド含む） |
