# apps/cc-tmux-tunnel

Server A: Claude Runner。tmux セッションを直接管理し、内部 REST API を公開する。
`claude_code` (単一ペイン) と `multi_agent_shogun` (shogun + multiagent の 10 ペイン) の 2 種類のセッションタイプをサポート。

グレースフルシャットダウンに対応し、SIGINT/SIGTERM 受信時に管理中の tmux セッションをクリーンアップする。

## 前提条件

- Go 1.22+
- tmux 3.0+

## 起動

```bash
go run ./cmd/cc-tmux-tunnel/ -addr :9090
```

| フラグ | デフォルト | 説明 |
|--------|-----------|------|
| `-addr` | `:9090` | リッスンアドレス |

## ビルド

```bash
go build -o cc-tmux-tunnel ./cmd/cc-tmux-tunnel
./cc-tmux-tunnel -addr :9090
```

## コード生成

内部 API のサーバーインターフェースは `apps/openapi/internal-openapi.yaml` から生成する。

```bash
go generate ./internal/api/
```

## セッションタイプ

| タイプ | ペイン数 | 説明 |
|--------|---------|------|
| `claude_code` | 1 | 単一 tmux セッションで Claude Code CLI を起動 |
| `multi_agent_shogun` | 10 | shogun (1 ペイン) + multiagent (9 ペイン) の 2 セッション構成 |

## ディレクトリ構成

```
apps/cc-tmux-tunnel/
├── cmd/cc-tmux-tunnel/main.go      # エントリーポイント (グレースフルシャットダウン対応)
├── internal/
│   ├── api/
│   │   ├── gen.go                  # 生成コード: 内部 API (DO NOT EDIT)
│   │   └── handler.go              # ServerInterface の実装
│   ├── session/manager.go          # セッション管理 (インメモリ)
│   └── tmux/tmux.go               # tmux コマンドラッパー (ペイン指定対応)
├── go.mod
└── go.sum
```
