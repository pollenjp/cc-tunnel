# apps/cc-tunnel

Server B: 外部向け API サーバー。外部クライアントからの HTTP リクエストを受け取り、Server A (`cc-tmux-tunnel`) の内部 API に転送するプロキシとして機能する。

## 前提条件

- Go 1.22+
- Server A (`cc-tmux-tunnel`) が起動済みであること

## 起動

```bash
go run ./cmd/cc-tunnel/ -addr :8080 -runner-url http://localhost:9090
```

| フラグ | デフォルト | 説明 |
|--------|-----------|------|
| `-addr` | `:8080` | リッスンアドレス |
| `-runner-url` | `http://localhost:9090` | cc-tmux-tunnel の URL |

## ビルド

```bash
go build -o cc-tunnel ./cmd/cc-tunnel
./cc-tunnel -addr :8080 -runner-url http://localhost:9090
```

## コード生成

外部 API のサーバーインターフェースと内部 API のクライアントは OpenAPI 定義から `oapi-codegen` で生成する。

```bash
# 外部 API サーバーコード
go generate ./internal/api/

# 内部 API クライアントコード
go generate ./internal/tmuxclient/
```

## ディレクトリ構成

```
apps/cc-tunnel/
├── cmd/cc-tunnel/main.go           # エントリーポイント (--runner-url フラグ)
├── internal/
│   ├── api/
│   │   ├── gen.go                  # 生成コード: 外部 API (DO NOT EDIT)
│   │   └── handler.go              # ServerInterface の実装 (tmuxclient 使用)
│   └── tmuxclient/
│       ├── generate.go             # go generate ディレクティブ
│       └── gen.go                  # 生成コード: 内部 API クライアント (DO NOT EDIT)
├── go.mod
└── go.sum
```
