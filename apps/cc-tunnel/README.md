# apps/cc-tunnel

Server B: 外部向け API サーバー。外部クライアントからの HTTP リクエストを受け取り、Server A (`cc-remote-agent`) の内部 API に転送するプロキシとして機能する。

## 前提条件

- Go 1.26+
- Server A (`cc-remote-agent`) が起動済みであること

## 起動

```bash
go run ./cmd/cc-tunnel/ -addr :8080 -runner-url http://localhost:9090
```

| フラグ | デフォルト | 説明 |
|--------|-----------|------|
| `-addr` | `:8080` | リッスンアドレス |
| `-runner-url` | `http://localhost:9090` | cc-remote-agent の URL |

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
```

## ディレクトリ構成

```
apps/cc-tunnel/
├── cmd/cc-tunnel/main.go           # エントリーポイント (--runner-url フラグ)
├── internal/
│   ├── api/
│   │   ├── gen.go                  # 生成コード: 外部 API (DO NOT EDIT)
│   │   └── handler.go              # ServerInterface の実装 (remoteclient 使用)
│   └── remoteclient/
│       └── client.go               # 内部 API クライアント
├── go.mod
└── go.sum
```
