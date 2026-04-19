# Database

cc-tunnel のデータ永続化層は PostgreSQL を使用する。マイグレーション管理には goose、接続プールには pgx/v5 の pgxpool を使用する。

## テーブル一覧

### conversations

会話セッションを管理するテーブル。

| カラム          | 型          | 制約        | デフォルト            | 説明                     |
| --------------- | ----------- | ----------- | --------------------- | ------------------------ |
| `id`            | UUID        | PRIMARY KEY | `gen_random_uuid()`   | 会話の一意識別子         |
| `title`         | TEXT        | NOT NULL    | `''`                  | 会話タイトル             |
| `model`         | TEXT        | NOT NULL    | `'claude-sonnet-4-6'` | 使用する Claude モデル名 |
| `system_prompt` | TEXT        | nullable    | —                     | システムプロンプト       |
| `status`        | TEXT        | NOT NULL, CHECK | `'idle'`          | 実行状態 (`idle` / `running` / `completed`) |
| `created_at`    | TIMESTAMPTZ | NOT NULL    | `NOW()`               | 作成日時                 |
| `updated_at`    | TIMESTAMPTZ | NOT NULL    | `NOW()`               | 最終更新日時             |

**インデックス**:

- `idx_conversations_updated_at ON conversations(updated_at DESC)` — 最近更新された会話順での一覧取得を高速化

### messages

会話内のメッセージを管理するテーブル。

| カラム            | 型          | 制約            | デフォルト          | 説明                                                                                |
| ----------------- | ----------- | --------------- | ------------------- | ----------------------------------------------------------------------------------- |
| `id`              | UUID        | PRIMARY KEY     | `gen_random_uuid()` | メッセージの一意識別子                                                              |
| `conversation_id` | UUID        | NOT NULL, FK    | —                   | 所属する会話の ID                                                                   |
| `role`            | TEXT        | NOT NULL, CHECK | —                   | `'user'` / `'assistant'` / `'system'`                                               |
| `message_data`    | JSONB       | NOT NULL        | `'{}'`              | メッセージデータ（user: `content`、assistant: `content_blocks`・`session_id` など） |
| `status`          | TEXT        | NOT NULL, CHECK | `'completed'`       | メッセージ状態 (`streaming` / `completed` / `error`)。既存レコードとの後方互換のため DEFAULT は `'completed'` |
| `created_at`      | TIMESTAMPTZ | NOT NULL        | `NOW()`             | 作成日時                                                                            |
| `updated_at`      | TIMESTAMPTZ | NOT NULL        | `NOW()`             | 最終更新日時（content_blocks 更新・status 変更時に自動更新）                         |

**外部キー**: `conversation_id` → `conversations(id) ON DELETE CASCADE`（会話削除時にメッセージも削除）

**インデックス**:

- `idx_messages_conversation_created ON messages(conversation_id, created_at ASC)` — 会話内のメッセージを時系列順で取得するクエリを高速化

**message_data の主なフィールド**（user メッセージ）:

- `content` (string): ユーザーが入力したメッセージ本文

**message_data の主なフィールド**（assistant メッセージ）:

- `content` (string): アシスタント応答のテキスト全文（content_blocks から結合）
- `session_id` (string): cc-remote-agent の Claude CLI セッション ID。`--resume` による会話継続に使用
- `thinking` ([]string): 拡張思考ブロックのテキスト配列
- `content_blocks` (array): thinking / text / tool_use の全ブロックを受信順に格納
  - `{ "type": "thinking", "content": string }`
  - `{ "type": "text", "content": string }`
  - `{ "type": "tool_use", "tool_use_id": string }`
- `tool_calls` (array): `ToolCallData` の配列。各要素は `tool_use_id`, `tool_name`, `input_json`, `result` を持つ
- `model` (string): 使用したモデル名
- `cost_usd` (float64): API コスト（USD）
- `duration_ms` (int64): 実行時間（ミリ秒）
- `hook_events` (array): フックイベントのリスト

## マイグレーション管理（goose）

マイグレーションには [pressly/goose v3](https://github.com/pressly/goose) を使用する。SQL ファイルは `apps/cc-tunnel/internal/db/migrations/` に配置され、バイナリに埋め込まれる。

```go
//go:embed migrations/*.sql
var migrations embed.FS
```

### 起動時の自動適用

`NewPool()` 呼び出し時に `goose.Up()` が自動実行される。アプリケーション起動と同時にマイグレーションが適用されるため、手動実行は不要。

```
apps/cc-tunnel/internal/db/migrations/
├── 001_create_conversations.sql    # conversations テーブル作成
├── 002_create_messages.sql         # messages テーブル作成
├── 003_add_conversation_status.sql # conversations.status カラム追加
└── 004_add_message_status.sql      # messages.status / updated_at カラム追加
```

各マイグレーションファイルは `-- +goose Up` / `-- +goose Down` アノテーションで Up/Down を定義する。

## データアクセス層（Repository パターン）

`db.Repository` 構造体が `*pgxpool.Pool` をラップし、すべての DB アクセスを一元管理する。

```go
type Repository struct {
    pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository
```

### 主要クエリパターン

**会話作成**

```sql
INSERT INTO conversations (title, model, system_prompt)
VALUES ($1, $2, $3)
RETURNING id, title, model, system_prompt, created_at, updated_at
```

`CreateConversation(ctx, title, model, systemPrompt)` で呼び出す。`systemPrompt` は `*string`（nullable）。

**会話一覧取得**

```sql
SELECT id, title, model, system_prompt, created_at, updated_at
FROM conversations ORDER BY updated_at DESC
```

`ListConversations(ctx)` で呼び出す。最近更新された会話が先頭に来る。

**メッセージ追加**

```sql
INSERT INTO messages (conversation_id, role, message_data)
VALUES ($1, $2, $3)
RETURNING id, conversation_id, role, message_data, created_at
```

`CreateMessage(ctx, conversationID, role, messageData)` で呼び出す。`messageData` は `map[string]interface{}` を JSON シリアライズして格納。

**会話内メッセージ取得**

```sql
SELECT id, conversation_id, role, message_data, created_at
FROM messages WHERE conversation_id = $1 ORDER BY created_at ASC
```

`ListMessages(ctx, conversationID)` で呼び出す。時系列昇順で返す。

**updated_at 更新**

```sql
UPDATE conversations SET updated_at = NOW() WHERE id = $1
```

`UpdateConversationUpdatedAt(ctx, id)` で呼び出す。メッセージ送信後に会話のタイムスタンプを更新するために使用。

**タイトル自動更新**

```sql
UPDATE conversations SET title = $1, updated_at = NOW() WHERE id = $2
```

`UpdateConversationTitle(ctx, id, title)` で呼び出す。アシスタントメッセージ保存後に、応答テキストから自動生成したタイトルで `title` カラムを更新する。

**ストリーミングメッセージ作成**

```sql
INSERT INTO messages (conversation_id, role, message_data, status)
VALUES ($1, $2, $3, 'streaming')
RETURNING id, conversation_id, role, message_data, status, created_at, updated_at
```

`CreateStreamingMessage(ctx, conversationID, role, messageData)` で呼び出す。`SendMessage()` 冒頭でアシスタントメッセージを `status='streaming'` で事前作成する。サーバークラッシュ時の復旧に利用。

**メッセージ content_blocks 更新（バッチ保存）**

```sql
UPDATE messages
SET message_data = message_data || jsonb_build_object('content_blocks', $1::jsonb),
    updated_at = NOW()
WHERE id = $2
```

`UpdateMessageContentBlocks(ctx, messageID, contentBlocks)` で呼び出す。5秒 ticker による定期バッチ保存と、実行完了時の最終保存に使用。

> **バッチ保存における tool_calls**: 5秒 ticker の goroutine は `UpdateMessageContentBlocks` に加えて `MergeMessageData({tool_calls: snapshotTools})` も呼び出す。これにより、フロントエンドがポーリング（`isPolling=true`）で会話を復元した際、SSE完了前でも `tool_calls` を参照できる。

**メッセージステータス更新**

```sql
UPDATE messages SET status = $1, updated_at = NOW() WHERE id = $2
```

`UpdateMessageStatus(ctx, messageID, status)` で呼び出す。完了時は `'completed'`、エラー時は `'error'` を設定。

**メッセージデータマージ**

```sql
UPDATE messages
SET message_data = message_data || $1::jsonb,
    updated_at = NOW()
WHERE id = $2
```

`MergeMessageData(ctx, messageID, extra)` で呼び出す。2つの用途で使用される:

1. **バッチ保存（5秒 ticker）**: `{tool_calls: snapshotTools}` をマージ。ポーリング中のフロントエンドがtool_calls情報を取得できるようにする。
2. **実行完了時**: session_id / model / cost_usd / duration_ms / hook_events / tool_calls 等を一括マージ。

**実行状態更新**

```sql
UPDATE conversations SET status = $1 WHERE id = $2
```

`UpdateConversationStatus(ctx, id, status)` で呼び出す。`SendMessage()` 実行時に以下のタイミングで更新する:

- Claude CLI 実行開始前: `status = 'running'`
- 実行完了後（`defer` で保証）: `status = 'completed'`

これにより、フロントエンドが別セッションから戻った際に `GET /conversations/:id` で `status` を確認し、実行継続中かどうかを判断できる。

タイトル生成ロジック（`generateTitle` 関数、`internal/api/title.go`）:

- 最新アシスタント応答の text ブロックから先頭 60 文字を抽出
- 改行は半角スペースに変換
- Markdown 記法（`#`, `*`, `` ` `` 等）を除去して平文にする
- 60 文字で切った場合は `...` を付加
- 空の場合のフォールバック: `"New Conversation"`

## PostgreSQL バージョン要件

Docker Compose では `postgres:18-alpine` を使用する。

```yaml
services:
  postgres:
    image: mirror.gcr.io/library/postgres:18-alpine
```

PostgreSQL 18 を使用する理由:

- `gen_random_uuid()` が組み込み関数として利用可能（pgcrypto 拡張不要）
- `JSONB` 型の最新最適化
- ボリュームパスは `/var/lib/postgresql`（data サブディレクトリを含む親ディレクトリをマウント）

**接続設定**（`DATABASE_URL` 環境変数）:

```
postgres://cctunnel:<password>@postgres:5432/cctunnel?sslmode=disable
```
