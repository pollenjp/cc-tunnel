# cc-tunnel アーキテクチャ概要

## コンポーネント構成

```
┌─────────────────────────────────────────────────────────────────┐
│  Browser                                                        │
│  React SPA (Vite + Tailwind CSS + xterm.js)                    │
└───────────────────────────┬─────────────────────────────────────┘
                            │ HTTP / SSE  (port 3000 → nginx)
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│  frontend (nginx)                                               │
│  静的ファイル配信 + /api/* → cc-tunnel プロキシ                 │
└───────────────────────────┬─────────────────────────────────────┘
                            │ HTTP (port 8080)
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│  cc-tunnel (Go)                                                 │
│  APIゲートウェイ・会話管理・SSE ストリーミング                  │
│  OpenAPI → oapi-codegen で型安全なルーティング                  │
└──────────┬───────────────────────────────┬──────────────────────┘
           │ HTTP (port 9091)              │ pgx/v5
           ▼                               ▼
┌──────────────────────┐       ┌───────────────────────┐
│  cc-remote-agent     │       │  PostgreSQL            │
│  (Go)                │       │  会話・メッセージ永続化│
│  claude CLI ラッパー │       │  (goose マイグレーション│
└──────────┬───────────┘       └───────────────────────┘
           │ PTY / exec
           ▼
┌──────────────────────┐
│  claude CLI           │
│  (Claude Code)        │
└──────────────────────┘
```

## データフロー: メッセージ送信時

ユーザーがメッセージを入力して送信してから、レスポンスが画面に表示されるまでの流れ。

```
1. ユーザーがテキスト入力 → MessageInput.tsx で Enter / 送信ボタン押下
2. App.tsx handleSend() が呼ばれる
3. POST /api/conversations/{id}/messages (JSON) → nginx → cc-tunnel
4. cc-tunnel: handler.SendMessage()
   a. PostgreSQL からユーザーメッセージ以前の会話履歴を取得
   b. ユーザーメッセージを PostgreSQL に保存
   c. 過去アシスタントメッセージの metadata から session_id を取得 (--resume 用)
5. cc-tunnel: remoteclient.Execute() → POST cc-remote-agent /execute (JSON)
   リクエスト: { prompt, session_id, model, conversation_history, include_hook_events: true, include_partial_messages: true }
6. cc-remote-agent: claude CLI を exec で起動し ndjson をストリーム出力
   main.go の responseWriter (Flusher ラッパー) により loggingMiddleware 経由でも Flush() が透過的に動作
7. cc-tunnel: ndjson イベントを SSE (text/event-stream) に変換してブラウザへ送信
8. ブラウザ: SSE イベントを処理
   - type=init       → モデル名・session_id を streamMeta に記録
   - type=text       → MessageBubble のテキストに追記
   - type=thinking   → MessageBubble の thinking ブロックに追記
   - type=text_delta / thinking_delta → requestAnimationFrame でバッファをまとめて反映
   - type=tool_use_start / tool_input_delta / tool_result → ToolCallCard に表示
   - type=hook_event → HookEvent リストに追加 (--include-hook-events フラグで有効化)
   - type=rate_limit → レートリミット情報を表示
   - type=cost / done → 完了・コスト情報を streamMeta に記録
9. cc-tunnel: "done" 受信後、アシスタントメッセージを PostgreSQL に保存
   metadata に session_id, thinking, content_blocks, tool_calls, model, cost_usd, duration_ms, hook_events を含める
```

## データフロー: 認証時 (PTY + xterm.js フロー)

ブラウザから Claude アカウントにログインするまでの流れ。

```
1. ブラウザ起動時: useAuth フック → GET /api/auth/status を定期ポーリング
2. 未ログイン → AuthGuard が AuthTerminal.tsx をレンダリング
3. ユーザーが「ログイン」ボタン押下
4. POST /api/auth/login → cc-tunnel → cc-remote-agent POST /auth/login
5. cc-remote-agent: AuthManager.StartLogin()
   - `claude /auth` を creack/pty で PTY 起動 (80×24 ウィンドウ)
   - 非同期 goroutine で PTY 出力を outputBuf に追記し続ける
6. AuthTerminal: ポーリングで GET /api/auth/output?since=N を呼び出す
7. cc-remote-agent: outputBuf の since バイト以降を base64 エンコードして返す
8. AuthTerminal: base64 デコード → @xterm/xterm Terminal.write() で PTY 出力を描画
9. ユーザーが xterm.js 画面上でキー入力 (矢印キー含む)
10. AuthTerminal: POST /api/auth/input { input: "\r" } などを送信
11. cc-remote-agent: AuthManager.SubmitInput() → PTY stdin に書き込み
12. claude /auth TUI が選択肢に応じてログインフロー実行 (OAuth URL など表示)
13. ログイン完了 → loginPending=false → GET /auth/status で loggedIn=true
14. useAuth がステータス変化を検知 → AuthGuard が Chat UI に切り替え
```

## 技術スタック

### バックエンド

| 項目 | 内容 |
|------|------|
| 言語 | Go 1.25.0 (cc-tunnel), Go 1.24.7 (cc-remote-agent) |
| HTTP サーバー | net/http (標準ライブラリ) |
| DB ドライバー | jackc/pgx v5.9.1 |
| DB マイグレーション | pressly/goose v3.27.0 |
| OpenAPI コード生成 | oapi-codegen v2.6.1 |
| PTY 制御 | creack/pty v1.1.24 |
| UUID | google/uuid v1.6.0 |

### フロントエンド

| 項目 | 内容 |
|------|------|
| フレームワーク | React 19.2.4 |
| 言語 | TypeScript 5.9.3 |
| ビルドツール | Vite 8.0.1 |
| CSS フレームワーク | Tailwind CSS v4.2.2 (@tailwindcss/vite プラグイン) |
| ターミナルエミュレーター | @xterm/xterm v6.0.0 |
| API クライアント生成 | openapi-fetch v0.17.0 + openapi-typescript v7.13.0 |
| Markdown レンダリング | react-markdown v10.1.0 + remark-gfm v4.0.1 |
| コードハイライト | react-syntax-highlighter v16.1.1 |

### インフラ

| 項目 | 内容 |
|------|------|
| コンテナ | Docker Compose (4 サービス) |
| リバースプロキシ | nginx (frontend コンテナ内) |
| データベース | PostgreSQL 18-alpine |

## OpenAPI 型統一

`apps/openapi/openapi.yaml` が型定義の single source of truth。

| 生成先 | ツール | 出力ファイル |
|--------|--------|-------------|
| Go (サーバー型 + ルーティング) | oapi-codegen v2.6.1 | `apps/cc-tunnel/internal/api/gen.go` |
| TypeScript (クライアント型) | openapi-typescript v7.13.0 | `apps/frontend/src/api/schema.d.ts` |

Go 側の生成コマンド（`handler.go` に `//go:generate` ディレクティブとして記録）:

```
go tool oapi-codegen -config ../../../openapi/oapi-codegen.yaml -o gen.go ../../../openapi/openapi.yaml
```

### SSE イベント型一覧

openapi.yaml に定義された 13 種の SSE イベント型 + ToolCallData:

| 型名 | type 値 | 説明 |
|------|---------|------|
| SSEInitEvent | `init` | ストリーム開始・モデル名・session_id |
| SSETextEvent | `text` | アシスタントテキスト（全体） |
| SSEThinkingEvent | `thinking` | 拡張思考ブロック（全体） |
| SSETextDeltaEvent | `text_delta` | テキストの差分ストリーム |
| SSEThinkingDeltaEvent | `thinking_delta` | 思考の差分ストリーム |
| SSEToolUseStartEvent | `tool_use_start` | ツール呼び出し開始 |
| SSEToolInputDeltaEvent | `tool_input_delta` | ツール入力 JSON の差分 |
| SSEToolResultEvent | `tool_result` | ツール実行結果 |
| SSEHookEvent | `hook_event` | Claude Code フックイベント |
| SSERateLimitEvent | `rate_limit` | レートリミット情報 |
| SSECostEvent | `cost` | コスト・実行時間 |
| SSEDoneEvent | `done` | ストリーム完了 |
| SSEErrorEvent | `error` | エラー |

`ToolCallData` 型もあわせて定義（`tool_use_id`, `tool_name`, `input_json`, `result`, `is_running`）。

## content_blocks アーキテクチャ

`SendMessage()` (handler.go) は SSE ストリームを処理しながら `contentBlocksList` を構築し、
ストリーム完了後に `metadata["content_blocks"]` として DB に保存する。

### ブロック構造

```json
[
  { "type": "thinking", "content": "<thinking text>" },
  { "type": "text",     "content": "<text content>" },
  { "type": "tool_use", "tool_use_id": "<id>" }
]
```

| type | 生成タイミング | 主なフィールド |
|------|--------------|---------------|
| `thinking` | `event.Type == "assistant"` の thinking ブロック | `content` |
| `text` | `event.Type == "assistant"` の text ブロック | `content` |
| `tool_use` | `event.Type == "assistant"` の tool_use ブロック | `tool_use_id` |

- ブロック順序はストリーム受信順と一致（thinking → text → tool_use の自然な順序）
- ツール実行がある場合は必ず `content_blocks` を保存
- 旧メッセージ（`content_blocks` なし）との後方互換を維持

## ErrorStackHandler（構造化ログ）

`internal/logging/handler.go` で `slog.Handler` インターフェースをラップした `ErrorStackHandler` を実装。

```go
type ErrorStackHandler struct {
    Next slog.Handler
}
```

`slog.Error()` 等でエラー属性を含むレコードが渡されると、自動で `"stack"` 属性（`[]string` の `file:line` リスト）を付与する。

- `extractStack()` が `runtime.Callers` でコールスタックを取得（最大 8 フレーム）
- slog / runtime 内部フレームは自動スキップ
- `Enabled`, `WithAttrs`, `WithGroup` も委譲して完全なハンドラーインターフェースを実装
