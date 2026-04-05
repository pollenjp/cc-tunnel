# cc-tunnel

tmux 上で Claude Code CLI を対話モードで起動し、HTTP API 経由で外部から制御するツール。

## プロジェクト構成

```
cc-tunnel/
├── apps/
│   ├── cc-tunnel/     # Go バックエンド (API サーバー + tmux 制御)
│   ├── frontend/      # React フロントエンド (Web UI)
│   └── openapi/       # OpenAPI 定義 (API の Single Source of Truth)
└── design/            # 設計ドキュメント
```

## 前提条件

- Go 1.22+
- Node.js 18+
- tmux 3.0+
- [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen) (コード生成時のみ)

## クイックスタート

### 1. バックエンド起動

```bash
cd apps/cc-tunnel
go run ./cmd/cc-tunnel --addr :8080
```

### 2. フロントエンド起動

```bash
cd apps/frontend
npm install
npm run dev
```

ブラウザで http://localhost:5173 を開く。

### 3. curl で操作

```bash
# セッション作成
curl -X POST localhost:8080/sessions

# 入力送信
curl -X POST localhost:8080/sessions/<id>/input \
  -H 'Content-Type: application/json' \
  -d '{"text": "hello"}'

# 出力取得
curl localhost:8080/sessions/<id>/output

# セッション削除
curl -X DELETE localhost:8080/sessions/<id>
```

## API 定義

OpenAPI 定義は `apps/openapi/openapi.yaml` にある。詳細は [apps/openapi/README.md](apps/openapi/README.md) を参照。

## 設計ドキュメント

アーキテクチャや将来の構成計画については [design/architecture.md](design/architecture.md) を参照。
