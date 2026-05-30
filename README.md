# cc-tunnel

Claude Code CLI をリモートから操作するためのチャット型 AI インターフェース。
Docker コンテナ上で `claude` CLI を実行し、Web ブラウザからチャット形式でやり取りできる。
会話セッションは PostgreSQL に永続化され、`--resume` による高効率なコンテキスト継続をサポート。

## アーキテクチャ

```
Browser
   │  (HTTP / SSE)
   ▼
frontend (React Chat UI)
   │  (HTTP proxy via nginx)
   ▼
cc-tunnel (Go API Server :8080)
   │  ├─ PostgreSQL (会話・メッセージ永続化)
   │  └─ cc-remote-agent (ndjson streaming)
   ▼
cc-remote-agent (Go :9091)
   │  (os/exec)
   ▼
claude CLI (`claude -p --output-format=stream-json --verbose`)
```

## コンポーネント

| コンポーネント | 役割 |
|---|---|
| **cc-remote-agent** | Docker上でclaude CLIを実行。プロンプトを受け取り、stream-json形式でndjsonストリーミング返却 |
| **cc-tunnel** | 外部向けAPIサーバ。会話セッション管理、PostgreSQL永続化、cc-remote-agentへのプロキシ |
| **frontend** | React製チャットUI。会話一覧・作成・メッセージ送受信。アシスタント応答はポーリングで逐次表示 |
| **PostgreSQL** | 会話（conversations）とメッセージ（messages）を永続化 |

## 会話継続メカニズム

claude CLIの `--resume <session_id>` フラグを主軸に使用する。
- **初回**: 新規セッションとして実行 → `result.session_id` を DB に保存
- **2回目以降**: `--resume <session_id>` でCLI内部のコンテキストを再利用
- **フォールバック**: resumeが失敗した場合、DBの過去メッセージをプロンプトに組み込んで再実行

これにより、長い会話でもトークン消費をO(1)/メッセージに抑える。

## セットアップ

### 前提条件

- Docker / Docker Compose
- Anthropic API キー

### 起動

```bash
# .env ファイルを作成
echo "ANTHROPIC_API_KEY=sk-ant-..." > apps/.env

# 全サービス起動
cd apps
docker compose up -d

# ブラウザで開く
open http://localhost:3000
```

### 環境変数

| 変数 | デフォルト | 説明 |
|---|---|---|
| `ANTHROPIC_API_KEY` | （必須） | Anthropic API キー |
| `POSTGRES_PASSWORD` | `cctunnel_dev` | PostgreSQL パスワード |
| `CC_REMOTE_AGENT_ENV_PORT` | `9091` | cc-remote-agent ポート |
| `CC_TUNNEL_ENV_PORT` | `8080` | cc-tunnel ポート |
| `FRONTEND_ENV_PORT` | `8080` | フロントエンドポート |

## API

cc-tunnel は以下の REST API を提供する（`/api/` プレフィックス経由）。

### 会話管理

| メソッド | パス | 説明 |
|---|---|---|
| `POST` | `/conversations` | 新規会話を作成 |
| `GET` | `/conversations` | 会話一覧を取得 |
| `GET` | `/conversations/{id}` | 会話詳細（メッセージ履歴含む）を取得 |
| `DELETE` | `/conversations/{id}` | 会話を削除 |

### メッセージ送信（非同期処理 + ポーリング）

メッセージ送信は即時 `202 Accepted` を返し、アシスタント応答はバックグラウンドで非同期処理する。
フロントエンドは `GET /conversations/{id}` をポーリングして応答を逐次表示する。

```
POST /conversations/{id}/messages
Content-Type: application/json

{"content": "Goでhello worldを書いて"}
```

```jsonc
// 202 Accepted — メッセージ受付済み（作成されたアシスタントメッセージ ID を返す）
{"message_id": "..."}
```

その後、会話の `status` が `running` の間は `GET /conversations/{id}` を約 1 秒間隔でポーリングし、
`completed` になったら停止する。詳細は [docs/api.md](./docs/api.md) を参照。

## 開発

### コード生成

OpenAPI定義からGoコード・TypeScript型を再生成:

```bash
# Go (cc-tunnel)
cd apps/cc-tunnel && go generate ./internal/api/

# TypeScript (frontend)
cd apps/frontend && npm run generate
```

### ビルド確認

```bash
# Go
cd apps/cc-tunnel && go build ./...
cd apps/cc-remote-agent && go build ./...

# TypeScript
cd apps/frontend && npm run build
```

## 技術スタック

| レイヤー | 技術 |
|---|---|
| バックエンド | Go 1.25、net/http |
| API仕様 | OpenAPI 3.0 + oapi-codegen |
| DB | PostgreSQL 17 + pgx/v5 + goose（マイグレーション） |
| フロントエンド | React 19 + TypeScript + Vite |
| AI | Claude Code CLI（`claude -p --output-format=stream-json`） |
| インフラ | Docker Compose / GCP (Cloud Run + GCE, Terraform) |

## ドキュメント

詳細なドキュメントは [`docs/`](./docs/README.md) にあります。アーキテクチャ・API・データベース・
認証・フロントエンド・インフラ（Terraform）・セッション隔離方式の設計などを索引ページからたどれます。
