# Docker ドキュメント

## サービス一覧

`apps/compose.yaml` で定義されたサービス。

### デフォルト起動サービス（常時起動）

| サービス名              | ビルド/イメージ                            | 公開ポート                           | 依存関係 |
| ----------------------- | ------------------------------------------ | ------------------------------------ | -------- |
| `postgres`              | `mirror.gcr.io/library/postgres:18-alpine` | `127.0.0.1:5432:5432`（ホスト開発用）| -        |
| `cc-tunnel`             | `./cc-tunnel` (ビルド)                     | `8080:8080`                          | `postgres` |

### `profiles: ["full"]` サービス（フル起動時のみ）

| サービス名  | ビルド/イメージ        | 公開ポート      | 依存関係                               |
| ----------- | ---------------------- | --------------- | -------------------------------------- |
| `frontend`  | `./frontend` (ビルド)  | `3000:8080`     | `cc-tunnel`                            |

---

## 各サービスの役割

### `postgres`

会話・メッセージデータを永続化する PostgreSQL データベース。`cc-tunnel` からのみアクセスされる。ヘルスチェックが成功してから `cc-tunnel` が起動する。ホスト開発用に `127.0.0.1:5432` を公開。

### `cc-tunnel`

バックエンド API サーバー (Go)。以下の役割を担う。

- 会話・メッセージの CRUD API (`/conversations`)
- 認証 API (`/auth/*`) → per-session container にルーティング（`conversationId` で特定）
- `EXECUTION_PROVIDER` に応じた実行プロバイダーでセッションコンテナを管理（詳細は下記「§ プロバイダー選択」参照）
- SSE ストリーミングによるレスポンス配信
- Docker-out-of-Docker (DooD): `/var/run/docker.sock` をマウントし、コンテナ内から Docker API を操作

---

## プロバイダー選択（EXECUTION_PROVIDER）

`cc-tunnel` は環境変数 `EXECUTION_PROVIDER` で実行プロバイダーを切り替える。

| 値 | プロバイダー | 用途・特徴 |
|----|-------------|-----------|
| `local` | `LocalDockerProvider` | **ローカル開発用**。compose ネットワーク (`apps_default`) 内で `/var/run/docker.sock` (DooD) を使い、同一ホスト上に `cctunnel-session-{convID}` コンテナを動的生成。セッション情報はメモリ管理（再起動でリセット） |
| `docker_gce` | `DockerGCEProvider` | **GCP 本番用**。GCE VM プールを動的生成し、各 VM 上に `session-{conversationID}` コンテナを起動。VM・セッション情報は DB 永続化。アイドル VM は自動削除 |
| `cloud_run_sandbox` | `CloudRunSandboxProvider` | **モック実装**（将来の Cloud Run サンドボックス対応用） |

**compose.yaml でのデフォルト**: `EXECUTION_PROVIDER=local`（cc-tunnel サービスの env に設定済み）

**docker_gce 使用時の追加設定**（`apps/compose.yaml` または Cloud Run env に追記）:

```yaml
environment:
  - EXECUTION_PROVIDER=docker_gce
  - GCE_PROJECT_ID=<your-project-id>
  - GCE_ZONE=us-central1-a
  - GCE_MACHINE_TYPE=e2-medium
  - CC_REMOTE_AGENT_IMAGE=<artifact-registry-image-url>
```

### `frontend`

React SPA を配信する nginx サーバー。

- `/` → Vite ビルド成果物 (SPA ルーティング対応)
- `/api/conversations/{id}/messages` → SSE 専用設定でバックエンドの `cc-tunnel` へプロキシ
- `/api/*` → 通常リバースプロキシで `cc-tunnel` へ転送

---

## cc-remote-agent イメージのビルド

`cc-tunnel` のセッションコンテナは `cc-remote-agent:latest` イメージを使用する。このイメージは `apps/prepare.compose.yaml` でビルドする（`compose.yaml` には含まれていない）。

```bash
cd apps/
docker compose -f prepare.compose.yaml build
```

`compose.yaml` を使う前に必ずこのビルドを実行すること。

---

## Docker-out-of-Docker (DooD)

`cc-tunnel` サービスはホストの Docker デーモンに `/var/run/docker.sock` をマウントする（`compose.yaml` の `volumes` セクション）。これにより `cc-tunnel` がコンテナ内から Docker SDK を使い、per-session `cc-remote-agent` コンテナを動的に生成・管理できる。

```yaml
volumes:
  - /var/run/docker.sock:/var/run/docker.sock
```

---

## 起動方法

### デフォルト起動（postgres のみ）

```bash
cd apps/
# cc-remote-agent イメージを事前ビルド
docker compose -f prepare.compose.yaml build
# デフォルトサービス起動
docker compose up -d
```

### フル起動（全サービス）

```bash
cd apps/
# cc-remote-agent イメージを事前ビルド
docker compose -f prepare.compose.yaml build
# 全サービス起動（cc-tunnel + frontend も含む）
docker compose --profile full up --build -d
# アクセス: http://localhost:3000
```

### mise タスク

```bash
# apps/ ディレクトリで実行
cd apps/
mise run docker:up    # 起動 (--build -d)
mise run docker:down  # 停止
mise run docker:logs  # ログをフォロー
```

---

## 環境変数

| 変数名                      | デフォルト値            | 対象サービス                        | 説明                                      |
| --------------------------- | ----------------------- | ----------------------------------- | ----------------------------------------- |
| `POSTGRES_PASSWORD`         | `cctunnel_dev`          | `postgres`, `cc-tunnel`             | PostgreSQL パスワード                     |
| `CC_TUNNEL_ENV_PORT`        | `8080`                  | `cc-tunnel`, `frontend`             | バックエンド API のリッスンポート         |
| `FRONTEND_ENV_PORT`         | `8080`                  | `frontend`                          | フロントエンド nginx のリッスンポート     |
| `BACKEND_URL`               | `/api`                  | `frontend`                          | フロントエンドが参照する API のベースパス |
| `API_UPSTREAM`              | `http://cc-tunnel:8080` | `frontend` (nginx)                  | nginx がプロキシするバックエンド URL      |

---

## ボリューム

| ボリューム名      | マウント先                                  | 用途                              |
| ----------------- | ------------------------------------------- | --------------------------------- |
| `pgdata`          | `/var/lib/postgresql` (postgres)            | PostgreSQL データ永続化           |

---

## ネットワーク

compose.yaml ではカスタムネットワークを明示指定していないため、Docker Compose のデフォルトブリッジネットワーク (`apps_default`) が作成される。全サービスはこのネットワークに参加し、サービス名で相互に名前解決できる。

フル起動時の外部からのアクセスは `frontend:3000` のみ。

---

## nginx の設定概要

`apps/frontend/nginx.conf.template` による設定。起動時に環境変数 (`$PORT`, `$API_UPSTREAM`) が展開される。

### SPA ルーティング

```nginx
location / {
    try_files $uri $uri/ /index.html;
}
```

React Router などのクライアントサイドルーティングに対応するため、存在しないパスは `index.html` にフォールバックする。

### SSE プロキシ (`/api/conversations/{id}/messages`)

```nginx
location ~ ^/api/conversations/[^/]+/messages {
    proxy_pass $API_UPSTREAM;
    proxy_buffering off;
    proxy_cache off;
    proxy_set_header X-Accel-Buffering no;
    proxy_set_header Connection '';
    proxy_read_timeout 300s;
}
```

SSE (Server-Sent Events) のストリーミングが途切れないよう、バッファリングを無効化し、タイムアウトを 300 秒に延長している。

### 通常 API プロキシ (`/api/`)

```nginx
location /api/ {
    proxy_pass $API_UPSTREAM/;
}
```

`/api/` プレフィックスを除いてバックエンドに転送する。

### 静的アセットキャッシュ

- Vite がコンテンツハッシュ付きファイル名を生成する JS/CSS/画像は 1 年間キャッシュ (`immutable`)。
- `index.html` と `env.js` はキャッシュ無効 (`no-store`) でデプロイ間の変更を即時反映する。
