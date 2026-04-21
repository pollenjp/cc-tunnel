# Docker ドキュメント

## サービス一覧

`apps/compose.yaml` で定義された4つのサービス。

| サービス名        | ビルド/イメージ                            | 公開ポート         | 依存関係                      |
| ----------------- | ------------------------------------------ | ------------------ | ----------------------------- |
| `postgres`        | `mirror.gcr.io/library/postgres:18-alpine` | 5432 (expose のみ) | -                             |
| `cc-remote-agent` | `./cc-remote-agent` (ビルド)               | 9091 (expose のみ) | -                             |
| `cc-tunnel`       | `./cc-tunnel` (ビルド)                     | 8080 (expose のみ) | `cc-remote-agent`, `postgres` |
| `frontend`        | `./frontend` (ビルド)                      | **3000→8080**      | `cc-tunnel`                   |

外部からアクセスできるのは `frontend` の 3000 番ポートのみ。その他のサービスは Docker 内部ネットワーク経由で通信する。

---

## 各サービスの役割

### `postgres`

会話・メッセージデータを永続化する PostgreSQL データベース。`cc-tunnel` からのみアクセスされる。ヘルスチェックが成功してから `cc-tunnel` が起動する。

### `cc-remote-agent`

Claude CLI (`claude`) をプロセスとして実行する Python/Go サービス。Claude CLI のセッションデータ (`~/.claude`) を `claude-sessions` ボリュームに永続化する。`cc-tunnel` からのみアクセスされる。

### `cc-tunnel`

バックエンド API サーバー (Go)。以下の役割を担う。

- 会話・メッセージの CRUD API (`/conversations`)
- 認証 API (`/auth/*`)
- `cc-remote-agent` への Claude CLI コマンド委譲
- SSE ストリーミングによるレスポンス配信

### `frontend`

React SPA を配信する nginx サーバー。

- `/` → Vite ビルド成果物 (SPA ルーティング対応)
- `/api/conversations/{id}/messages` → SSE 専用設定でバックアップの `cc-tunnel` へプロキシ
- `/api/*` → 通常リバースプロキシで `cc-tunnel` へ転送

---

## 起動方法

### Docker Compose

```bash
# apps/ ディレクトリで実行
cd apps/
docker compose up --build -d
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

| 変数名                     | デフォルト値            | 対象サービス                   | 説明                                      |
| -------------------------- | ----------------------- | ------------------------------ | ----------------------------------------- |
| `POSTGRES_PASSWORD`        | `cctunnel_dev`          | `postgres`, `cc-tunnel`        | PostgreSQL パスワード                     |
| `ANTHROPIC_API_KEY`        | (必須)                  | `cc-remote-agent`              | Anthropic API キー                        |
| `CC_REMOTE_AGENT_ENV_PORT` | `9091`                  | `cc-remote-agent`, `cc-tunnel` | リモートエージェントのリッスンポート      |
| `CC_TUNNEL_ENV_PORT`       | `8080`                  | `cc-tunnel`, `frontend`        | バックエンド API のリッスンポート         |
| `FRONTEND_ENV_PORT`        | `8080`                  | `frontend`                     | フロントエンド nginx のリッスンポート     |
| `BACKEND_URL`              | `/api`                  | `frontend`                     | フロントエンドが参照する API のベースパス |
| `API_UPSTREAM`             | `http://cc-tunnel:8080` | `frontend` (nginx)             | nginx がプロキシするバックエンド URL      |

---

## ボリューム

| ボリューム名      | マウント先                        | 用途                              |
| ----------------- | --------------------------------- | --------------------------------- |
| `pgdata`          | `/var/lib/postgresql` (postgres)  | PostgreSQL データ永続化           |
| `claude-sessions` | `/root/.claude` (cc-remote-agent) | Claude CLI セッション・設定永続化 |

---

## ネットワーク

compose.yaml ではカスタムネットワークを明示指定していないため、Docker Compose のデフォルトブリッジネットワーク (`apps_default`) が作成される。全サービスはこのネットワークに参加し、サービス名で相互に名前解決できる。

外部からのアクセスは `frontend:3000` のみ。

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
