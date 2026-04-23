# cc-tunnel アーキテクチャ概要

## コンポーネント構成

### 現行（ローカル開発構成）

```
┌─────────────────────────────────────────────────────────────────┐
│  Browser                                                        │
│  React SPA (Vite + Tailwind CSS + xterm.js)                    │
└───────────────────────────┬─────────────────────────────────────┘
                            │ HTTP (port 3000 → nginx)
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│  frontend (nginx)                                               │
│  静的ファイル配信 + /api/* → cc-tunnel プロキシ                 │
└───────────────────────────┬─────────────────────────────────────┘
                            │ HTTP (port 8080)
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│  cc-tunnel (Go)                                                 │
│  APIゲートウェイ・会話管理・非同期メッセージ処理                  │
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

### 将来（Cloud Run + GCE 構成）

```
┌─────────────────────────────────────────────────────────────────┐
│  Browser                                                        │
│  React SPA (Vite + Tailwind CSS + xterm.js)                    │
└───────────────────────────┬─────────────────────────────────────┘
                            │ HTTPS (port 443)
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│  frontend (nginx)                                               │
│  静的ファイル配信 + /api/* → cc-tunnel プロキシ                 │
└───────────────────────────┬─────────────────────────────────────┘
                            │ HTTP
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│  cc-tunnel (Go, Cloud Run)                                      │
│  APIゲートウェイ・会話管理・SessionManager                       │
└──────────┬───────────────────────────────┬──────────────────────┘
           │ pgx/v5                        │ HTTP (VPC Connector)
           ▼                               ▼
┌──────────────────────┐       ┌───────────────────────────────────┐
│  PostgreSQL           │       │  GCE VM (Docker Host)             │
│  (Cloud SQL)          │       │  ├─ session-abc (cc-remote-agent, :9091) │
│  会話・メッセージ永続化│       │  ├─ session-def (cc-remote-agent, :9092) │
│  セッションメタデータ  │       │  └─ ...                           │
└──────────────────────┘       └───────────────────────────────────┘
```

## データフロー: メッセージ送信時

ユーザーがメッセージを入力して送信してから、レスポンスが画面に表示されるまでの流れ。

```
1. ユーザーがテキスト入力 → MessageInput.tsx で Enter / 送信ボタン押下
2. ChatView.tsx handleSend() が呼ばれる
3. POST /api/conversations/{id}/messages (JSON) → nginx → cc-tunnel
4. cc-tunnel: handler.SendMessage()
   a. PostgreSQL からユーザーメッセージ以前の会話履歴を取得
   b. ユーザーメッセージを PostgreSQL に保存
   c. 過去アシスタントメッセージの metadata から session_id を取得 (--resume 用)
   d. アシスタントメッセージを status='streaming' で事前作成
   e. 202 Accepted + message_id を即時返却
5. cc-tunnel goroutine: remoteclient.Execute() → POST cc-remote-agent /execute (JSON)
   リクエスト: { prompt, session_id, model, conversation_history, include_hook_events: true, include_partial_messages: true }
6. cc-remote-agent: claude CLI を exec で起動し ndjson をストリーム出力
7. cc-tunnel goroutine: ndjson イベントを処理し、2 秒ごとに content_blocks / tool_calls を DB に保存（バッチ保存）
8. cc-tunnel goroutine: 実行完了後、最終 content_blocks・session_id・model 等を message_data にマージ、
   status を 'completed' に更新。conversations.status も 'completed' に更新。
   最新アシスタント応答の先頭 60 文字をタイトルとして自動生成。
9. ブラウザ (ChatView): isPolling=true で GET /api/conversations/{id} を 1 秒間隔でポーリング
   - status === 'running' → streaming メッセージの content_blocks を表示 + TypingIndicator
   - status === 'completed' → ポーリング停止、最終メッセージ表示
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
| コンテナ | ローカル開発: Docker Compose / 本番: Cloud Run (cc-tunnel) + GCE (Docker Host) |
| リバースプロキシ | nginx (frontend コンテナ内) |
| データベース | PostgreSQL 18-alpine |
| セッション隔離方式 | Docker on GCE（コンテナ隔離）/ Cloud Run Sandbox（gVisor隔離）から選択 |

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

`ToolCallData` 型が定義されている（`tool_use_id`, `tool_name`, `input_json`, `result`, `is_running`）。

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

## アーキテクチャパターン分類

### 結論

cc-tunnel の内部アーキテクチャは **Transaction Script + 二層構成（Handler-Repository）** に分類される。

### パターン名と根拠

| 観点 | パターン名 | 根拠 |
|------|-----------|------|
| ビジネスロジック構成 | **Transaction Script** | 各 HTTP ハンドラーメソッドが「1リクエスト = 1手続き」として完結。`SendMessage()` に会話履歴取得→保存→リモート実行→非同期DB保存が手続き的に記述される。ドメインモデル層は存在せず、`Conversation`/`Message` はメソッドを持たない純粋な DTO |
| サービス内部構造 | **二層（Handler + Repository）** | Presentation 層（api/handler.go）と Data Access 層（db/repository.go）は分離されているが、Business Logic 層（service/usecase）は独立パッケージとして存在しない。三層アーキテクチャの中間層が欠落 |
| デプロイメント | **軽量マイクロサービス** | cc-tunnel と cc-remote-agent が独立した go.mod・Docker コンテナを持ち、HTTP で通信。共有コードなし |
| データアクセス | **Repository / Table Data Gateway** | `db.Repository` が SQL クエリを集約。構造体はテーブルと 1:1 マッピング |
| フロントエンド | **コンポーネントベース SPA** | React useState/useRef のみ。外部状態管理なし。App.tsx に会話一覧管理。メッセージ管理は ChatView に委譲 |
| API 設計 | **OpenAPI-First（Contract-First）** | openapi.yaml が型の Single Source of Truth。Go/TypeScript 双方の型を自動生成 |

### 該当しないパターン

| パターン | 不一致理由 |
|---------|-----------|
| クリーンアーキテクチャ | 依存性逆転原則（DIP）未適用。handler が具象型に直接依存。ポート/アダプターの概念なし |
| 三層アーキテクチャ | Presentation と Data Access は分離されているが、Business Logic 層が独立していない |
| ヘキサゴナル（Ports & Adapters） | インターフェース（Port）が未定義 |
| DDD | ドメインモデル・集約・値オブジェクトなし |

### 強みと弱み

**強み**: シンプルさ（Go ソース計 10 ファイル）、開発速度（handler に直接記述で完了）、OpenAPI 駆動の型安全性、依存の軽量さ（cc-remote-agent の外部依存は creack/pty のみ）

**弱み**: テスタビリティ（具象型依存でモック注入困難、`handler_test.go` はヘルパーのみ）、Handler 肥大化（`SendMessage` 230 行超）、ビジネスロジックの再利用不可（handler 内インライン）

詳細分析は `logs/2026-04-19T082423JST_cmd_cctunnel_arch_analysis_001/report.md` を参照。

## フロントエンド切断耐性（Disconnect Resilience）

`SendMessage()` は会話処理中にフロントエンド（ブラウザ）が切断（リロード・別セッション選択・タブ閉鎖）
されても、バックエンドの Claude CLI 実行と DB 保存を継続する設計になっている。

### 問題の根本原因

HTTP サーバー（net/http）はクライアント切断を検知すると `r.Context()` をキャンセルする。
修正前のコードは `r.Context()` を Execute および DB 保存に直接渡していたため:

1. `remoteclient.Execute(r.Context(), ...)` — コンテキストがキャンセルされると
   cc-remote-agent への HTTP リクエストが中断され、Claude CLI 実行が途中で止まる
2. `repo.CreateMessage(r.Context(), ...)` — キャンセルされたコンテキストで呼ぶと
   `context.Canceled` エラーとなりアシスタントメッセージが DB に保存されない

### 設計解決策: context.WithoutCancel

```go
// http.Request の ctx とは独立したコンテキストを生成
// フロントエンド切断で r.Context() がキャンセルされても execCtx は影響を受けない
execCtx := context.WithoutCancel(r.Context())

newSessionID, err := h.remote.Execute(execCtx, executeReq, callback)
// ...
h.repo.CreateMessage(execCtx, convIDStr, "assistant", messageData)
h.repo.UpdateConversationUpdatedAt(execCtx, convIDStr)
```

`context.WithoutCancel` (Go 1.21+) は親コンテキストの値（deadline, value）を引き継ぎつつ
キャンセル信号だけを切り離す。これにより:

- フロントエンド切断 → `r.Context()` キャンセル
- Claude CLI 実行 (`execCtx`) は継続 → 最後まで完走
- DB 保存 (`execCtx`) も継続 → アシスタントメッセージが確実に保存される

### コンテキスト分離の境界

| 処理 | 使用コンテキスト | 理由 |
|------|----------------|------|
| GetConversation / ListMessages | `r.Context()` | リクエスト検証フェーズ。切断なら早期リターンが正しい |
| CreateMessage (user msg) | `r.Context()` | リクエスト受付フェーズ。切断なら保存不要 |
| Execute (Claude CLI) | `execCtx` | フロントエンドとは独立して最後まで実行 |
| CreateMessage (assistant msg) | `execCtx` | Execute 完了後に必ず保存する |
| UpdateConversationUpdatedAt | `execCtx` | 同上 |

### インターフェース抽象化

テスタビリティ向上のため `repository` / `remoteClient` インターフェースを
`internal/api/interfaces.go` に定義し、`Server` 構造体はインターフェースに依存するよう変更した。

```go
type Server struct {
    repo   repository   // *db.Repository (本番) / モック (テスト)
    remote remoteClient // *remoteclient.Client (本番) / モック (テスト)
}
```

`sendmessage_test.go` のテストはモック実装でコンテキストキャンセル挙動を再現し、
切断耐性を単体テストで検証する。

## DBポーリング方式（フロントエンド状態更新）

フロントエンドは `GET /conversations/{id}` を 1 秒間隔でポーリングして処理結果を取得する。SSE は使用しない。

### 実装概要

**バックエンド**:

1. `conversations.status` カラム（`idle` / `running` / `completed`）を追加（migration 003）
2. `SendMessage()` 実行時:
   - 実行開始前に `status = 'running'` に更新
   - 実行終了後（`defer`）に `status = 'completed'` に更新
3. `GET /conversations/{id}` のレスポンスに `status` と `messages` を含める

**フロントエンド（ChatView）**:

1. メッセージ送信後: `isPolling = true` にしてポーリング開始
2. 会話選択時: `GET /conversations/{id}` で `status` を確認し、`running` なら `isPolling = true`
3. `useConversationPoller` フック:
   - 1 秒間隔で `GET /conversations/{id}` を呼び出す
   - `onMessages(全メッセージ)` で全置換更新
   - `status === 'completed'` になったらポーリング停止、`onCompleted()` を呼ぶ

### コンテキスト分離との連携

| 処理 | context | 説明 |
|------|---------|------|
| status='running' 更新 | `execCtx` | フロントエンド切断後も確実に更新される |
| status='completed' 更新 | `execCtx` | `defer` で必ず実行される |
| ポーリング (`GET /conversations/{id}`) | フロントエンド fetch | 独立したリクエスト |

`execCtx = context.WithoutCancel(r.Context())` により、フロントエンド切断後も status 更新が確実に実行される。

## DB駆動状態管理（Streaming Message Persistence）

`SendMessage()` は Claude CLI 実行中に中間状態を DB に保存し、サーバークラッシュや長時間実行に対する耐性を持つ設計になっている。

### 設計の核心

1. **早期メッセージ作成**: Execute 前にアシスタントメッセージを `status='streaming'` で INSERT。サーバークラッシュ後も「ストリーム中だった」ことが DB に残る。
2. **2秒バッチ保存**: goroutine + `time.Ticker(2s)` で `content_blocks` を定期的に UPDATE。イベントごとの UPDATE は JSONB 書き換えコストが高いため禁止。
3. **mutex 排他制御**: `contentBlocksList` への書き込み（SSE コールバック）と読み取り（ticker goroutine）を `sync.Mutex` で保護。
4. **完了時の最終保存**: Execute 完了後、`UpdateMessageContentBlocks`（最終 content_blocks）→ `MergeMessageData`（session_id / model 等）→ `UpdateMessageStatus("completed")` の順で更新。
5. **エラー時**: `UpdateMessageStatus("error")` を設定して状態を確定させる。
6. **孤児クリーンアップ**: 起動時に `status='streaming'` かつ 30 分以上前に作成されたメッセージを `status='error'` に更新（`NewPool()` 内で実行）。

### タイムライン

```
SendMessage 開始
  │
  ├─ CreateStreamingMessage → status='streaming', message_data={}
  │
  ├─ ticker goroutine 起動（2秒ごと UpdateMessageContentBlocks + tool_calls マージ）
  │
  ├─ UpdateConversationStatus → 'running'
  │
  ├─ remote.Execute (Claude CLI)
  │   ├─ SSE callback: mu.Lock → contentBlocksList append → mu.Unlock
  │   ├─ [5s] ticker fires → UpdateMessageContentBlocks(snapshot)
  │   └─ ...
  │
  ├─ UpdateMessageContentBlocks (最終 content_blocks)
  ├─ MergeMessageData (session_id, model, cost_usd, ...)
  ├─ UpdateMessageStatus → 'completed'
  │
  └─ defer: UpdateConversationStatus → 'completed'
```

### コンテキスト分離との連携

全ての DB 操作は `execCtx = context.WithoutCancel(r.Context())` を使用するため、フロントエンド切断後も継続される。

## コンストラクタ関数パターン（構造体フィールド設定漏れ防止）

DB から API レスポンス型への変換は、専用のコンストラクタ関数（`internal/api/mapping.go`）に集約されている。

### 設計の目的

`ConversationDetail.Status` のようなフィールドが Go のゼロ値（`""`）のままレスポンスに含まれると、
フロントエンドのポーリングが完了を検知できないバグが発生した（2026-04-21実例）。
このクラスのバグを構造的に根絶するため 3 層防御を設けた。

### 3 層防御

| 層 | 手段 | 実装箇所 |
|----|------|---------|
| 1 | コンストラクタ関数（単一変換経路） | `internal/api/mapping.go` |
| 2 | exhaustruct linter（コンパイル前検知） | `.golangci.yml` |
| 3 | フィールド網羅テスト（回帰防止） | `internal/api/handler_test.go`, `mapping_test.go` |

### コンストラクタ関数

```go
// DB → API Conversation
func newConversation(c *db.Conversation) Conversation

// DB → API Message
func newMessage(m *db.Message) Message

// DB → API ConversationDetail（messages を含む完全レスポンス）
func newConversationDetail(conv *db.Conversation, msgs []*db.Message) ConversationDetail
```

`handler.go` は直接構造体リテラルを組み立てず、必ずこれらのコンストラクタを呼び出す。

### exhaustruct linter 設定

API レスポンス型のみに適用（全構造体への過剰適用を防ぐため `include` で絞り込み）:

```yaml
linters-settings:
  exhaustruct:
    include:
      - 'github\.com/pollenjp/.../internal/api\.ConversationDetail$'
      - 'github\.com/pollenjp/.../internal/api\.Message$'
      - 'github\.com/pollenjp/.../internal/api\.Conversation$'
```

これらの型の struct literal に未設定フィールドがあると `golangci-lint` がコンパイル前に検知する。

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

## セッション隔離アーキテクチャ（Docker on GCE）

### 概要

会話セッションごとに独立した cc-remote-agent コンテナを動的生成する方式。
詳細設計は `docs/docker-gce-design.md` を参照。

### 現行との差分

| 項目 | 現行（単一インスタンス） | 新方式（Docker on GCE） |
|------|----------------------|----------------------|
| cc-remote-agent | 常時稼働の単一インスタンス | セッションごとに動的生成 |
| 隔離性 | なし（全セッション共有） | コンテナレベル（namespaces） |
| セッション接続 | 固定 -agent-url | SessionManager による per-session routing |
| インフラ | Docker Compose（ローカル） | Cloud Run + GCE（本番） |

### 新規コンポーネント: SessionManager

cc-tunnel に追加する中核コンポーネント。
- 責務: 会話→エンドポイントのマッピング管理、GCE VM/Dockerコンテナのライフサイクル管理
- 配置: apps/cc-tunnel/internal/sessionmanager/
- 詳細: `docs/docker-gce-design.md` §2「コンポーネント詳細」参照

### 変更影響ファイル

- apps/cc-tunnel/cmd/cc-tunnel/main.go: SessionManager初期化追加
- apps/cc-tunnel/internal/api/handler.go: per-session routing
- apps/cc-tunnel/internal/db/repository.go: session_endpoints, vm_instances テーブル追加

（変更不要: cc-remote-agent, frontend, openapi）

## セッション隔離アーキテクチャ（Cloud Run Sandbox）

### 概要

会話セッションごとに Cloud Run Sandbox（gVisor隔離）を使用する方式。
詳細設計は `docs/cloud-run-sandbox-design.md` を参照。

### Docker on GCE との対比

設計書の比較表を参考に、用途・適用シナリオの違い:

- Docker on GCE: 重い作業・長時間セッション・カスタム環境向け
- Cloud Run Sandbox: 軽量・高速起動・高セキュリティ（gVisor）・checkpoint/restore 対応

### ユーザー選択メカニズム

`POST /api/conversations` の `execution_mode` パラメータで選択:

- `"docker_gce"`: Docker on GCE 方式
- `"cloud_run_sandbox"`: Cloud Run Sandbox 方式

### 新規コンポーネント: SandboxManager

- 責務: Cloud Run Sandbox のライフサイクル管理・WebSocket 通信
- `SessionProvider` インターフェースで `SessionManager` と統一インターフェース
- 詳細: `docs/cloud-run-sandbox-design.md` 参照
