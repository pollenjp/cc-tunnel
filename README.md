# cc-tunnel

tmux 上で Claude Code CLI を対話モードで起動し、HTTP API 経由で外部から制御するツール。
2 つのサーバー構成 (API プロキシ + tmux ランナー) で動作し、マルチエージェント Shogun セッションにも対応。

## プロジェクト構成

```
cc-tunnel/
├── apps/
│   ├── cc-tunnel/         # Server B: API Server (外部向けプロキシ)
│   ├── cc-tmux-tunnel/    # Server A: Claude Runner (tmux + claude 管理)
│   ├── frontend/          # React フロントエンド (Web UI)
│   └── openapi/           # OpenAPI 定義 (外部 API + 内部 API)
└── design/                # 設計ドキュメント
```

## 前提条件

- Go 1.26+
- Node.js 18+
- tmux 3.0+
- [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen) (コード生成時のみ)

## クイックスタート

### 1. Server A (Claude Runner) 起動

```bash
cd apps/cc-tmux-tunnel
go run ./cmd/cc-tmux-tunnel/ -addr :9090
```

### 2. Server B (API Server) 起動

```bash
cd apps/cc-tunnel
go run ./cmd/cc-tunnel/ -addr :8080 -runner-url http://localhost:9090
```

### 3. フロントエンド起動

```bash
cd apps/frontend
npm install
npm run dev
```

ブラウザで http://localhost:5173 を開く。

### 4. curl で操作

```bash
# セッション作成 (claude_code タイプ)
curl -X POST localhost:8080/sessions \
  -H 'Content-Type: application/json' \
  -d '{"type": "claude_code"}'

# マルチエージェント Shogun セッション作成
curl -X POST localhost:8080/sessions \
  -H 'Content-Type: application/json' \
  -d '{"type": "multi_agent_shogun"}'

# セッション一覧
curl localhost:8080/sessions

# 入力送信 (paneIndex で対象ペインを指定)
curl -X POST 'localhost:8080/sessions/<id>/input?paneIndex=0' \
  -H 'Content-Type: application/json' \
  -d '{"keys": ["hello", "Enter"]}'

# 出力取得
curl 'localhost:8080/sessions/<id>/output?paneIndex=0'

# 全ペイン出力を一括取得
curl localhost:8080/sessions/<id>/outputs

# 未管理セッションの検出
curl localhost:8080/sessions/discover

# セッション削除
curl -X DELETE localhost:8080/sessions/<id>
```

## API 定義

OpenAPI 定義は `apps/openapi/` にある。詳細は [apps/openapi/README.md](apps/openapi/README.md) を参照。

- `openapi.yaml` — 外部 API (Server B)
- `internal-openapi.yaml` — 内部 API (Server A)

## 設計ドキュメント

アーキテクチャや将来の構成計画については [design/architecture.md](design/architecture.md) を参照。
