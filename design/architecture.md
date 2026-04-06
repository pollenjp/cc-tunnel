# cc-tunnel アーキテクチャ設計

## ユースケース

tmux 上で Claude Code CLI を対話モードで起動し、外部から API 経由で制御する。

### 登場するコンポーネント

- **Server A (Claude Runner) = `cc-tmux-tunnel`**: tmux を起動し、その中で `claude` CLI を対話モードで実行するマシン
  - `tmux new -s 'claude-<random-string>'`
  - tmux セッション内で `claude` を起動
  - 内部 REST API を公開し、Server B からの指示を受ける
- **Server B (API Server) = `cc-tunnel`**: 外部クライアントから HTTP リクエストを受け取り、Server A の tmux + claude を制御する
  - Server A の内部 API を呼び出して tmux + claude を操作
  - 外部向け HTTP API を公開

### 通信の方向

```
Client (外部) ──HTTP API──► Server B (cc-tunnel) ──内部 REST API──► Server A (cc-tmux-tunnel)
                            :8080                                    :9090
```

Server B が外部からリクエストを受け取り、Server A の内部 API に転送する。

## アーキテクチャ

### 分離構成

Server A と Server B を別プロセスとして稼働させる。同一マシンでも別マシンでも動作する。

```
┌──────────────────┐              ┌─────────────────────┐
│ Server B         │              │   Server A           │
│ (cc-tunnel)      │── REST API ──│ (cc-tmux-tunnel)     │
│ :8080            │              │ :9090                │
│ 外部 API プロキシ │              │ tmux + claude 管理    │
└──────────────────┘              └─────────────────────┘
```

### Server A - Server B 間通信

REST + OpenAPI を採用。

| 選定理由 |
|---------|
| 操作は6つの単純な CRUD エンドポイント、ストリーミング不要 |
| 既存の oapi-codegen パイプラインをサーバー側・クライアント側の両方で再利用 |
| gRPC は protobuf ツールチェーンの追加コストに見合わない |
| curl でデバッグ可能 |

内部 API 定義: `apps/openapi/internal-openapi.yaml`

## セッションタイプ

| タイプ | ペイン数 | 説明 |
|--------|---------|------|
| `claude_code` | 1 | 単一 tmux セッション (`claude-<id>`) で Claude Code CLI を起動 |
| `multi_agent_shogun` | 10 | shogun (1 ペイン) + multiagent (9 ペイン) の 2 セッション構成 |

`multi_agent_shogun` では `paneIndex` クエリパラメータで操作対象ペインを指定する (0 = shogun, 1-9 = multiagent)。

## API 設計

### 外部 API (Server B: cc-tunnel)

| メソッド | パス | 説明 |
|---------|------|------|
| `POST` | `/sessions` | セッション作成 (type: `claude_code` / `multi_agent_shogun`) |
| `GET` | `/sessions` | セッション一覧取得 |
| `GET` | `/sessions/discover` | 未管理 tmux セッションの検出 |
| `POST` | `/sessions/{id}/input?paneIndex=N` | 入力送信 (`tmux send-keys`、ペイン指定可) |
| `GET` | `/sessions/{id}/output?paneIndex=N` | ペイン出力取得 (`tmux capture-pane`) |
| `GET` | `/sessions/{id}/outputs` | 全ペイン出力の一括取得 |
| `POST` | `/sessions/{id}/resize` | ウィンドウリサイズ |
| `DELETE` | `/sessions/{id}` | セッション終了 (`tmux kill-session`) |

### 内部 API (Server A: cc-tmux-tunnel)

外部 API と同一のエンドポイント・スキーマ。Server B は薄いプロキシとして機能する。

### リクエスト/レスポンス例

#### セッション作成

```
POST /sessions
{"type": "claude_code"}
→ 201 Created
{
  "id": "a1b2c3d4e5f6g7h8",
  "type": "claude_code",
  "tmux_name": "claude-a1b2c3d4e5f6g7h8",
  "pane_count": 1,
  "created_at": "2026-04-05T12:00:00Z"
}
```

#### 入力送信

```
POST /sessions/{id}/input?paneIndex=0
{"keys": ["hello", "Enter"]}
→ 200 OK
{"status": "ok"}
```

#### 出力取得

```
GET /sessions/{id}/output?paneIndex=0
→ 200 OK
{"output": "...tmux pane content..."}
```

#### 全ペイン出力取得

```
GET /sessions/{id}/outputs
→ 200 OK
{"panes": {"0": "...shogun output...", "1": "...agent1 output...", ...}}
```

#### 未管理セッション検出

```
GET /sessions/discover
→ 200 OK
[{"type": "claude_code", "tmux_names": ["claude-abc123"]}, ...]
```

#### セッション削除

```
DELETE /sessions/{id}
→ 200 OK
{"status": "deleted"}
```

## 技術選定

- **言語**: Go
- **API 定義**: OpenAPI 3.0
- **コード生成**: `oapi-codegen` で `ServerInterface` + モデル型 + HTTP クライアントを生成
- **HTTP**: 標準ライブラリ `net/http` (生成されたルーティングを使用)
- **tmux 操作**: `os/exec` で tmux コマンドを直接呼び出し (Server A のみ)
- **セッション管理**: インメモリ (`sync.RWMutex` + `map`) (Server A のみ)
- **グレースフルシャットダウン**: SIGINT/SIGTERM 受信時に管理中の tmux セッションをクリーンアップ (Server A)

### コード生成フロー

```
apps/openapi/openapi.yaml          ──(oapi-codegen)──►  apps/cc-tunnel/internal/api/gen.go
                                                           ├── ServerInterface (外部 API)
                                                           └── モデル型

apps/openapi/internal-openapi.yaml ──(oapi-codegen)──►  apps/cc-tmux-tunnel/internal/api/gen.go
                                                           ├── ServerInterface (内部 API)
                                                           └── モデル型

apps/openapi/internal-openapi.yaml ──(oapi-codegen)──►  apps/cc-tunnel/internal/tmuxclient/gen.go
                                                           ├── ClientWithResponses (HTTP クライアント)
                                                           └── モデル型
```

再生成コマンド:
```bash
# 外部 API サーバー
cd apps/cc-tunnel && go generate ./internal/api/

# 内部 API サーバー
cd apps/cc-tmux-tunnel && go generate ./internal/api/

# 内部 API クライアント
cd apps/cc-tunnel && go generate ./internal/tmuxclient/
```

## プロジェクト構成

```
cc-tunnel/
├── apps/
│   ├── cc-tunnel/                         # Server B: API Server (外部向けプロキシ)
│   │   ├── cmd/cc-tunnel/main.go          # エントリーポイント (--runner-url フラグ)
│   │   ├── internal/
│   │   │   ├── api/
│   │   │   │   ├── gen.go                 # 生成コード: 外部 API (DO NOT EDIT)
│   │   │   │   └── handler.go             # ServerInterface の実装 (tmuxclient 使用)
│   │   │   └── tmuxclient/
│   │   │       └── gen.go                 # 生成コード: 内部 API クライアント (DO NOT EDIT)
│   │   ├── go.mod
│   │   └── go.sum
│   ├── cc-tmux-tunnel/                    # Server A: Claude Runner (tmux 管理)
│   │   ├── cmd/cc-tmux-tunnel/main.go     # エントリーポイント
│   │   ├── internal/
│   │   │   ├── api/
│   │   │   │   ├── gen.go                 # 生成コード: 内部 API (DO NOT EDIT)
│   │   │   │   └── handler.go             # ServerInterface の実装
│   │   │   ├── session/manager.go         # セッション管理
│   │   │   └── tmux/tmux.go              # tmux コマンドラッパー
│   │   ├── go.mod
│   │   └── go.sum
│   ├── frontend/                          # React フロントエンド
│   └── openapi/                           # API 定義
│       ├── openapi.yaml                   # 外部 API 定義 (Single Source of Truth)
│       ├── oapi-codegen.yaml              # 外部 API 生成設定
│       ├── internal-openapi.yaml          # 内部 API 定義
│       ├── internal-oapi-codegen-server.yaml  # 内部 API サーバー生成設定
│       └── internal-oapi-codegen-client.yaml  # 内部 API クライアント生成設定
├── design/                                # 設計ドキュメント
└── README.md
```

## 起動方法

```bash
# Server A (Claude Runner) を起動
cd apps/cc-tmux-tunnel && go run ./cmd/cc-tmux-tunnel/ -addr :9090

# Server B (API Server) を起動
cd apps/cc-tunnel && go run ./cmd/cc-tunnel/ -addr :8080 -runner-url http://localhost:9090
```
