# cc-tunnel アーキテクチャ設計

## ユースケース

cc-remote-agent 経由で Claude Code CLI を `claude -p --output-format=stream-json` で直接実行し、外部から API 経由で制御する。

### 登場するコンポーネント

- **Server A (Claude Runner) = `cc-remote-agent`**: `claude` CLI を `claude -p --output-format=stream-json` で直接実行するマシン
  - 内部 REST API を公開し、Server B からの指示を受ける
- **Server B (API Server) = `cc-tunnel`**: 外部クライアントから HTTP リクエストを受け取り、Server A の cc-remote-agent を制御する
  - Server A の内部 API を呼び出して cc-remote-agent を操作
  - 外部向け HTTP API を公開

### 通信の方向

```
Client (外部) ──HTTP API──► Server B (cc-tunnel) ──内部 REST API──► Server A (cc-remote-agent)
                            :8080                                    :9090
```

Server B が外部からリクエストを受け取り、Server A の内部 API に転送する。

## アーキテクチャ

### 分離構成

Server A と Server B を別プロセスとして稼働させる。同一マシンでも別マシンでも動作する。

```
┌──────────────────┐              ┌─────────────────────┐
│ Server B         │              │   Server A           │
│ (cc-tunnel)      │── REST API ──│ (cc-remote-agent)    │
│ :8080            │              │ :9090                │
│ 外部 API プロキシ │              │ claude CLI 管理       │
└──────────────────┘              └─────────────────────┘
```

### Server A - Server B 間通信

REST + OpenAPI を採用。

| 選定理由 |
|---------|
| 操作は単純な REST エンドポイント、ストリーミングは ndjson で対応 |
| 既存の oapi-codegen パイプラインをサーバー側で再利用 |
| gRPC は protobuf ツールチェーンの追加コストに見合わない |
| curl でデバッグ可能 |

内部 API 定義: `apps/openapi/openapi.yaml`

## API 設計

### 外部 API (Server B: cc-tunnel)

| メソッド | パス | 説明 |
|---------|------|------|
| `GET` | `/auth/status` | 認証状態取得 |
| `POST` | `/auth/login` | ログイン開始 |
| `POST` | `/auth/logout` | ログアウト |
| `POST` | `/auth/cancel` | ログインキャンセル |
| `POST` | `/auth/input` | 認証入力送信 |
| `GET` | `/auth/output` | 認証出力取得 |
| `POST` | `/conversations` | 会話作成 |
| `GET` | `/conversations` | 会話一覧取得 |
| `GET` | `/conversations/{id}` | 会話詳細取得 |
| `DELETE` | `/conversations/{id}` | 会話削除 |
| `POST` | `/conversations/{id}/messages` | メッセージ送信 |

### 内部 API (Server A: cc-remote-agent)

cc-remote-agent が提供する REST エンドポイント。Server B は HTTP クライアント (`remoteclient.Client`) で呼び出す。

| メソッド | パス | 説明 |
|---------|------|------|
| `GET` | `/auth/status` | 認証状態取得 |
| `POST` | `/auth/login` | ログイン開始 |
| `POST` | `/auth/logout` | ログアウト |
| `POST` | `/auth/cancel` | ログインキャンセル |
| `POST` | `/auth/input` | 認証入力送信 |
| `GET` | `/auth/output?since=N` | 認証出力取得 |
| `POST` | `/execute` | claude CLI 実行 (ndjson ストリーミング) |

### リクエスト/レスポンス例

#### 会話作成

```
POST /conversations
{"title": "新規会話", "model": "claude-sonnet-4-6"}
→ 201 Created
{
  "id": "a1b2c3d4-...",
  "title": "新規会話",
  "model": "claude-sonnet-4-6",
  "created_at": "2026-04-05T12:00:00Z"
}
```

#### メッセージ送信

```
POST /conversations/{id}/messages
{"content": "hello"}
→ 202 Accepted
{"message_id": "b2c3d4e5-..."}
```

#### claude CLI 実行 (内部 API)

```
POST /execute
{
  "prompt": "hello",
  "session_id": "prev-session-id",
  "model": "claude-sonnet-4-6",
  "include_partial_messages": true
}
→ 200 OK (ndjson stream)
{"type": "assistant", "message": {...}}
{"type": "result", "session_id": "new-session-id", "total_cost_usd": 0.001}
```

## 技術選定

- **言語**: Go
- **API 定義**: OpenAPI 3.0
- **コード生成**: `oapi-codegen` で `ServerInterface` + モデル型を生成
- **HTTP**: 標準ライブラリ `net/http` (生成されたルーティングを使用)
- **claude 実行**: `os/exec` で `claude -p --output-format=stream-json` を直接呼び出し (Server A のみ)
- **セッション継続**: `--resume <session_id>` フラグで claude CLI のセッションを再利用 (Server A のみ)
- **DB**: PostgreSQL で会話・メッセージを永続化 (Server B のみ)
- **グレースフルシャットダウン**: SIGINT/SIGTERM 受信時にクリーンアップ (Server A)

### コード生成フロー

```
apps/openapi/openapi.yaml          ──(oapi-codegen)──►  apps/cc-tunnel/internal/api/gen.go
                                                           ├── ServerInterface (外部 API)
                                                           └── モデル型
```

再生成コマンド:
```bash
# 外部 API サーバー
cd apps/cc-tunnel && go generate ./internal/api/
```

## プロジェクト構成

```
cc-tunnel/
├── apps/
│   ├── cc-tunnel/                         # Server B: API Server (外部向けプロキシ)
│   │   ├── cmd/cc-tunnel/main.go          # エントリーポイント (--agent-url フラグ)
│   │   ├── internal/
│   │   │   ├── api/
│   │   │   │   ├── gen.go                 # 生成コード: 外部 API (DO NOT EDIT)
│   │   │   │   └── handler.go             # ServerInterface の実装 (remoteclient 使用)
│   │   │   ├── db/                        # PostgreSQL リポジトリ
│   │   │   └── remoteclient/
│   │   │       └── client.go              # cc-remote-agent HTTP クライアント
│   │   ├── go.mod
│   │   └── go.sum
│   ├── cc-remote-agent/                   # Server A: Claude Runner (claude CLI 直接実行)
│   │   ├── internal/
│   │   │   ├── api/
│   │   │   │   └── handler.go             # 内部 API ハンドラ
│   │   │   ├── auth/                      # 認証管理
│   │   │   └── claude/
│   │   │       └── executor.go            # claude CLI 実行ロジック
│   │   ├── go.mod
│   │   └── go.sum
│   ├── frontend/                          # React フロントエンド
│   └── openapi/                           # API 定義
│       ├── openapi.yaml                   # 外部 API 定義 (Single Source of Truth)
│       └── oapi-codegen.yaml              # 外部 API 生成設定
├── design/                                # 設計ドキュメント
└── README.md
```

## 起動方法

### 個別起動

```bash
# Server A (Claude Runner) を起動
cd apps/cc-remote-agent && go run ./cmd/cc-remote-agent/ -addr :9090

# Server B (API Server) を起動
cd apps/cc-tunnel && go run ./cmd/cc-tunnel/ -addr :8080 -agent-url http://localhost:9090

# フロントエンドを起動
cd apps/frontend && npm install && npm run dev
```

### Docker Compose

```bash
cd apps && docker compose up --build -d
```

http://localhost:3000 でフロントエンドにアクセスできる。
