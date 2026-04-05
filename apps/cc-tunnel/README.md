# apps/cc-tunnel

cc-tunnel の Go バックエンド。HTTP API サーバーとして動作し、ローカルの tmux セッションを制御する。

## 前提条件

- Go 1.22+
- tmux 3.0+

## 起動

```bash
go run ./cmd/cc-tunnel --addr :8080
```

`--addr` でリッスンアドレスを指定 (デフォルト: `:8080`)。

## ビルド

```bash
go build -o cc-tunnel ./cmd/cc-tunnel
./cc-tunnel --addr :8080
```

## コード生成

API ハンドラのインターフェースとモデル型は OpenAPI 定義 (`apps/openapi/openapi.yaml`) から `oapi-codegen` で生成する。

### 前提

```bash
go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
```

### 生成コマンド

```bash
go generate ./internal/api/
```

これにより `internal/api/gen.go` が再生成される。このファイルは自動生成なので手動で編集しない。

### 生成されるもの

- `ServerInterface` - 各エンドポイントのハンドラインターフェース
- モデル型 (`Session`, `SendInputRequest`, `OutputResponse`, `StatusResponse`, `Error`)
- `HandlerFromMux` - `http.ServeMux` へのルーティング登録

`internal/api/handler.go` が `ServerInterface` を実装する。

## ディレクトリ構成

```
apps/cc-tunnel/
├── cmd/cc-tunnel/main.go       # エントリーポイント
├── internal/
│   ├── api/
│   │   ├── gen.go              # 生成コード (DO NOT EDIT)
│   │   └── handler.go          # ServerInterface の実装
│   ├── session/manager.go      # セッション管理 (インメモリ)
│   └── tmux/tmux.go            # tmux コマンドラッパー
├── go.mod
└── go.sum
```
