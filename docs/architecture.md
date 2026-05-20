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
│  LocalDockerProvider + SessionManager (per-session コンテナ管理) │
│  OpenAPI → oapi-codegen で型安全なルーティング                  │
└──────┬──────────────────────────────────────────┬──────────────┘
       │ /var/run/docker.sock (DooD)              │ pgx/v5
       ▼                                          ▼
┌────────────────┐                     ┌─────────────────────┐
│ Docker daemon  │                     │  PostgreSQL         │
│ (ホスト共有)   │                     │  会話・メッセージ   │
└───────┬────────┘                     │  永続化             │
        │ manage                       └─────────────────────┘
        ▼
┌─────────────────────────────────────────────────────┐
│  cctunnel-session-{convID[:8]} (per-session)        │
│  cc-remote-agent コンテナ（会話ごとに動的生成）     │
│  /auth/* + Execute API 提供                         │
│  tmpfs: /home/user/.claude                          │
│  URL: http://cctunnel-session-{convID[:8]}:9091     │
└─────────────────────┬───────────────────────────────┘
                          │ PTY / exec
                          ▼
              ┌───────────────────────┐
              │  claude CLI           │
              │  (Claude Code)        │
              └───────────────────────┘
```

### 本番（Cloud Run + GCE 構成）

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
│  APIゲートウェイ・会話管理・DockerGCEProvider (ExecutionProvider) │
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

## Bearer 認証フロー

cc-tunnel の保護済みエンドポイント（会話管理・credentials 系）は Bearer Token 認証を使用する。

```
1. ユーザーがアプリにログイン: POST /api/app-auth/login { "username": "alice" }
2. cc-tunnel: 32 バイトのランダム値を crypto/rand で生成 → hex エンコード（64 文字）
3. レスポンス: { "token": "a3f2c1...64文字hex", "user": { "id": "...", "name": "alice" } }
4. フロントエンド: トークンを sessionStorage に保管
5. 以降のリクエスト: Authorization: Bearer a3f2c1... ヘッダを自動付与
6. cc-tunnel: bearerToken(r) でヘッダ検証 → session.get(token) でユーザー特定
```

**対象エンドポイント（Bearer 必須）**:
- `GET /conversations`, `POST /conversations`, `GET /conversations/{id}`, `DELETE /conversations/{id}`
- `POST /conversations/{id}/messages`
- `GET /credentials/status`, `POST /credentials/relogin/start`, `POST /credentials/relogin/finalize`

## データフロー: 認証時 (PTY + SSE フロー)

ブラウザから Claude アカウントにログインするまでの流れ。

```
1. ブラウザ起動時: useAuth フック → GET /api/auth/status を定期ポーリング
2. 未ログイン → AuthGuard が AuthTerminal.tsx をレンダリング
3. ユーザーが「ログイン」ボタン押下
4. POST /api/auth/login → cc-tunnel → session container POST /auth/login（conversationId で特定）
5. session container: AuthManager.StartLogin()
   - `claude /auth` を creack/pty で PTY 起動 (80×24 ウィンドウ)
   - 非同期 goroutine で PTY バイト列を fan-out チャネルへ broadcast し続ける
   （旧: outputBuf に追記 → GetOutput(since) でポーリング取得 は除去済み）
6. AuthTerminal: GET /api/auth/pty/stream?conversationId=... で SSE 接続を確立
7. session container: AuthManager.Subscribe(ctx) で fan-out チャネルを取得
   → PTY バイト列を受信するたびに base64 エンコードして SSE data として送信
   （ANSI エスケープはストリップせずそのまま通過）
8. AuthTerminal: SSE data を base64 デコード → Uint8Array → @xterm/xterm Terminal.write()
   で PTY 出力を描画（ANSI エスケープシーケンスは xterm.js が処理）
9. ユーザーが xterm.js 画面上でキー入力 (矢印キー含む)
10. AuthTerminal: POST /api/auth/pty/input { conversationId, input: "\r" } などを送信
11. session container: AuthManager.SubmitInput() → PTY stdin に書き込み
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
  │   ├─ [2s] ticker fires → UpdateMessageContentBlocks(snapshot)
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

## セッション隔離アーキテクチャ（local provider / DooD）

### 概要

`local` provider（`EXECUTION_PROVIDER=local`）では、会話セッションごとに独立した cc-remote-agent コンテナを
DooD（Docker outside of Docker）方式で動的生成する。cc-tunnel コンテナは `/var/run/docker.sock` を
マウントしてホストの Docker daemon を操作する。

### コンポーネント構成

| コンポーネント | 説明 |
|--------------|------|
| `cctunnel-session-{convID[:8]}` | セッションコンテナ。LocalDockerProvider が動的生成。claude CLI 実行を担当 |

### SessionManager（実装済み）

`internal/docker/session_manager.go` に実装済みの中核コンポーネント。

- 責務: convID → コンテナの 1:1 マッピング管理、Docker コンテナのライフサイクル管理
- 配置: `apps/cc-tunnel/internal/docker/session_manager.go`

#### GetOrCreate フロー

```
GetOrCreate(ctx, convID)
  │
  ├─ sessions キャッシュにヒット → ContainerInspect で running 確認
  │   ├─ running → lastUsed/idleTimer リセット → client 返却
  │   └─ dead → sessions から削除 → 再作成フローへ
  │
  └─ キャッシュミス（新規）
      ├─ containerName = "cctunnel-session-" + convID[:8]
      ├─ ContainerCreate: Image, Network, Env, VolumeMounts 設定
      ├─ ContainerStart
      ├─ waitForReady: /auth/status 500ms ポーリング（StartTimeout: 30s）
      ├─ idleTimer 設定（IdleTimeout: デフォルト 15分）
      └─ sessions に登録 → client 返却
```

#### コンテナ設定

| 設定項目 | 値 |
|---------|-----|
| コンテナ名 | `cctunnel-session-{convID[:8]}` |
| イメージ | `CC_REMOTE_AGENT_IMAGE`（デフォルト: `cc-remote-agent:latest`） |
| ネットワーク | `DOCKER_NETWORK`（デフォルト: `apps_default`） |
| ボリューム | tmpfs → `/home/user/.claude` |
| URL 方式 | `http://cctunnel-session-{convID[:8]}:{port}`（DNS 方式のみ） |
| idle timeout | 15分（デフォルト）|

#### ライフサイクル管理

- `Stop(convID)`: idleTimer 停止 → ContainerStop → ContainerRemove → sessions から削除
- `StopAll`: 全セッションの Stop（graceful shutdown 用）
- `CleanupOrphans`: 起動時に `cctunnel-session-*` の stopped コンテナを一括削除

### セッション隔離アーキテクチャ（docker_gce provider）

`docker_gce` provider は GCE VM 上の cc-remote-agent コンテナに TCP 接続してセッションを管理する（**本格実装済み**）。
詳細設計は `docs/docker-gce-design.md` を参照。

## DooD（Docker outside of Docker）方式

cc-tunnel コンテナはホストの `/var/run/docker.sock` をマウントすることで、コンテナ内部から
ホストの Docker daemon を操作する（DooD 方式）。

### 特徴

| 項目 | 内容 |
|------|------|
| ソケットマウント | `compose.yaml`: `volumes: - /var/run/docker.sock:/var/run/docker.sock` |
| URL 生成方式 | DNS 方式のみ: `http://{containerName}:{port}` |

## compose 構成

### ファイル役割

| ファイル | 目的 |
|---------|------|
| `apps/prepare.compose.yaml` | **イメージビルドのみ**: `cc-remote-agent:latest` をビルドする |
| `apps/compose.yaml` | **サービス起動**: postgres（デフォルト）、profile "full" で全サービス起動 |

### compose.yaml サービス構成

**デフォルト（profile 指定なし）**:

| サービス | 説明 |
|---------|------|
| `postgres` | PostgreSQL 18-alpine、ヘルスチェック付き |

**profile "full"（`--profile full`）**:

| サービス | 説明 |
|---------|------|
| `cc-tunnel` | cc-tunnel 本体、`/var/run/docker.sock` マウント（DooD）、`EXECUTION_PROVIDER=local` |
| `frontend` | nginx + React SPA |

### 起動手順

```bash
# 1. イメージビルド
docker compose -f apps/prepare.compose.yaml build

# 2. 基本サービス起動（postgres）
docker compose -f apps/compose.yaml up -d

# 3. 全サービス起動（cc-tunnel + frontend も含む）
docker compose -f apps/compose.yaml --profile full up -d
```

## ExecutionProvider パターン（実装）

### 概要

cc-remote-agent の実行基盤を `ExecutionProvider` インターフェースで抽象化した実装。
`execution_mode` パラメータで会話ごとに provider を選択できる。

### Provider 一覧

| provider | パッケージ | 実装状態 | 説明 |
|----------|------------|---------|------|
| local | internal/provider/local | 実装済み | LocalDockerProvider: SessionManager で会話ごとに Docker コンテナを動的生成（DooD 方式） |
| cloud_run_sandbox | internal/provider/cloudrunsandbox | mock | Cloud Run Sandbox 方式（将来実装） |
| docker_gce | internal/provider/dockergce | **本格実装済み** | DockerGCEProvider: GCE VM ライフサイクル管理 + Docker TCP 接続（詳細: `docs/docker-gce-design.md`） |

### インターフェース

```go
type ExecutionProvider interface {
    Execute(ctx context.Context, req remoteclient.Request, onEvent func(remoteclient.StreamEvent)) (string, error)
}
```

パッケージ: `github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/provider`

### EXECUTION_PROVIDER 環境変数

| 値 | Provider | 説明 |
|----|----------|------|
| `local` | local.LocalDockerProvider | DooD 方式で per-session Docker コンテナを動的生成（/auth/* も per-session コンテナにルーティング） |
| `cloud_run_sandbox` | cloudrunsandbox.MockProvider | Cloud Run Sandbox 方式（将来実装） |
| `docker_gce` | dockergce.DockerGCEProvider | Docker on GCE 方式（**本格実装済み**。GCE_PROJECT_ID / GCE_ZONE / GCE_MACHINE_TYPE / GCE_AGENT_IMAGE 環境変数が必要） |

**必須**: `EXECUTION_PROVIDER` は必ず明示的に設定すること。未設定・不正値はエラー終了（デフォルト動作なし）。

```
EXECUTION_PROVIDER=local cc-tunnel
```

### 使用方法

`POST /api/conversations` または `POST /api/conversations/{id}/messages` に
`execution_mode: "local" | "cloud_run_sandbox" | "docker_gce"` を指定。
`EXECUTION_PROVIDER` 環境変数で起動時に provider を選択（必須、省略不可）。

### 実装ファイル概要

- `internal/provider/provider.go`: `ExecutionProvider` インターフェース定義
- `internal/provider/local/local.go`: `remoteclient.Client` をラップする基本 local provider
- `internal/provider/local/docker_provider.go`: `LocalDockerProvider` — `SessionManager` 経由で per-session コンテナを動的生成
- `internal/provider/cloudrunsandbox/mock.go`: cloud_run_sandbox mock provider（固定レスポンス）
- `internal/provider/dockergce/provider.go`: DockerGCEProvider 本格実装（GCE VM ライフサイクル + startup-script + TCP 接続）
- `internal/provider/dockergce/idle_checker.go`: IdleChecker goroutine（アイドルセッション検出・削除）
- `internal/provider/dockergce/vmscaler.go`: VMScaler goroutine（旧: in-process な周期 reap）。Cloud Run 上では信頼できないため default 無効。詳細は ADR `2026-05-20T20:46:00+09:00_01_vm_reap_dual_path.md`
- `internal/provider/dockergce/provider.go::ReconcileVMs`: container-manager の実測値で `zero_agents_since` を更新し、しきい値超で GCE delete を呼ぶ権威ロジック。Cloud Scheduler 経由 HTTP (`POST /internal/reconcile-vms`) と VM 側 self-reaper の双方から呼ばれる想定（テスト: `reconcile_vms_test.go`）
- `internal/provider/dockergce/mock.go`: docker_gce mock provider（フォールバック用スタブ）
- `internal/gce/client.go`: GCEClient インターフェース定義
- `internal/gce/sdk_client.go`: cloud.google.com/go/compute/apiv1 を使った本番実装
- `internal/gce/mock_client.go`: テスト用 MockGCEClient
- `internal/docker/runner.go`: `DockerRunner` インターフェース（コンテナ操作の抽象化）
- `internal/docker/session_manager.go`: `SessionManager` — convID → コンテナの 1:1 管理
- `internal/api/handler.go`: `Server` 構造体に `executionProvider` フィールドを追加、`SendMessage()` で利用
- `internal/api/interfaces.go`: `remoteClient` インターフェースから `Execute` を削除（`ExecutionProvider` へ移管）
- `apps/openapi/openapi.yaml`: `CreateConversationRequest` に `execution_mode` フィールドを追加
- `cmd/cc-tunnel/main.go`: `newProviderFromEnv` 関数で `EXECUTION_PROVIDER` に応じた provider を生成

### newProviderFromEnv 関数

`cmd/cc-tunnel/main.go` の `newProviderFromEnv(envVal string)` が provider インスタンスを生成する。

| EXECUTION_PROVIDER | 生成されるもの |
|--------------------|---------------|
| `local` | `SDKRunner` → `SessionManager` → `LocalDockerProvider` |
| `cloud_run_sandbox` | `cloudrunsandbox.New()` |
| `docker_gce` | `dockergce.New()` |
| 未設定 / 不正値 | エラー終了 |

`local` 選択時、`SessionManagerConfig` は環境変数から設定される:

| 環境変数 | デフォルト | 説明 |
|----------|-----------|------|
| `CC_REMOTE_AGENT_IMAGE` | `cc-remote-agent:latest` | セッションコンテナのイメージ名 |
| `DOCKER_NETWORK` | `apps_default` | compose ネットワーク名 |
| `CLAUDE_SESSIONS_VOLUME` | `claude-sessions` | claude 認証情報ボリューム名 |
| `CC_REMOTE_AGENT_PORT` | `9091` | コンテナの Listen ポート |

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
