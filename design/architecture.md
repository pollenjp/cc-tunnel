# cc-tunnel アーキテクチャ設計

## ユースケース

tmux 上で Claude Code CLI を対話モードで起動し、外部から API 経由で制御する。

### 登場するコンポーネント

- **Server A (Claude Runner)**: tmux を起動し、その中で `claude` CLI を対話モードで実行するマシン
  - `tmux new -s 'claude-<random-string>'`
  - tmux セッション内で `claude` を起動
- **Server B (API Server)**: 外部クライアントから HTTP リクエストを受け取り、Server A の tmux + claude を制御する
  - Server A の tmux + claude の起動を指示
  - Server A の tmux + claude への入力を指示
  - Server A の tmux + claude の出力を取得

### 通信の方向

```
Client (外部) ──HTTP API──► Server B (API Server) ──制御──► Server A (tmux + claude)
```

Server B が外部からリクエストを受け取る。Server A から Server B にリクエストする前提ではない。

## フェーズ計画

### Phase 1: 同一マシン構成 (現在)

Server A と Server B を同一マシン上で動かす。API サーバーがローカルの tmux を直接操作する。

```
┌───────────────────────────┐
│  Server A = Server B      │
│  API Server + tmux + claude│
│  (同一マシン)              │
└───────────────────────────┘
```

### Phase 2: 分離構成 (将来)

Server B から Server A を SSH またはエージェントデーモン経由で制御する。

```
┌──────────┐              ┌─────────────────┐
│ Server B │── SSH/Agent ─│   Server A      │
│ (API)    │              │ (tmux + claude) │
└──────────┘              └─────────────────┘
```

制御方式の候補:

| 方式 | 概要 | メリット | デメリット |
|------|------|---------|-----------|
| SSH | Server B が SSH で Server A に接続し tmux を操作 | Server A に追加ソフト不要 | SSH 接続管理が必要 |
| Agent 型 | Server A にも軽量デーモンを置き、Server B から指示を受ける | 柔軟・堅牢 | Server A にもプロセスが必要 |

## API 設計

| メソッド | パス | 説明 |
|---------|------|------|
| `POST` | `/sessions` | tmux セッション作成 + `claude` 起動 |
| `GET` | `/sessions` | セッション一覧取得 |
| `POST` | `/sessions/{id}/input` | テキスト入力送信 (`tmux send-keys`) |
| `GET` | `/sessions/{id}/output` | 画面出力取得 (`tmux capture-pane`) |
| `DELETE` | `/sessions/{id}` | セッション終了 (`tmux kill-session`) |

### リクエスト/レスポンス例

#### セッション作成

```
POST /sessions
→ 201 Created
{
  "id": "a1b2c3d4e5f6g7h8",
  "tmux_name": "claude-a1b2c3d4e5f6g7h8",
  "created_at": "2026-04-05T12:00:00Z"
}
```

#### 入力送信

```
POST /sessions/{id}/input
{"text": "hello"}
→ 200 OK
{"status": "ok"}
```

#### 出力取得

```
GET /sessions/{id}/output
→ 200 OK
{"output": "...tmux pane content..."}
```

#### セッション削除

```
DELETE /sessions/{id}
→ 200 OK
{"status": "deleted"}
```

## 技術選定

- **言語**: Go
- **HTTP**: 標準ライブラリ `net/http` (Go 1.22+ のルーティングパターン使用)
- **tmux 操作**: `os/exec` で tmux コマンドを直接呼び出し
- **セッション管理**: インメモリ (`sync.RWMutex` + `map`)

## プロジェクト構成

```
cc-tunnel/
├── cmd/cc-tunnel/main.go          # エントリーポイント
├── design/                        # 設計ドキュメント
├── internal/
│   ├── api/handler.go             # HTTP ハンドラ
│   ├── session/manager.go         # セッション管理
│   └── tmux/tmux.go               # tmux コマンドラッパー
├── go.mod
└── README.md
```
