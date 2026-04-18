# Database

cc-tunnel のデータ永続化層は PostgreSQL を使用する。マイグレーション管理には goose、接続プールには pgx/v5 の pgxpool を使用する。

## テーブル一覧

### conversations

会話セッションを管理するテーブル。

| カラム | 型 | 制約 | デフォルト | 説明 |
|---|---|---|---|---|
| `id` | UUID | PRIMARY KEY | `gen_random_uuid()` | 会話の一意識別子 |
| `title` | TEXT | NOT NULL | `''` | 会話タイトル |
| `model` | TEXT | NOT NULL | `'claude-sonnet-4-6'` | 使用する Claude モデル名 |
| `system_prompt` | TEXT | nullable | — | システムプロンプト |
| `created_at` | TIMESTAMPTZ | NOT NULL | `NOW()` | 作成日時 |
| `updated_at` | TIMESTAMPTZ | NOT NULL | `NOW()` | 最終更新日時 |

**インデックス**:
- `idx_conversations_updated_at ON conversations(updated_at DESC)` — 最近更新された会話順での一覧取得を高速化

### messages

会話内のメッセージを管理するテーブル。

| カラム | 型 | 制約 | デフォルト | 説明 |
|---|---|---|---|---|
| `id` | UUID | PRIMARY KEY | `gen_random_uuid()` | メッセージの一意識別子 |
| `conversation_id` | UUID | NOT NULL, FK | — | 所属する会話の ID |
| `role` | TEXT | NOT NULL, CHECK | — | `'user'` / `'assistant'` / `'system'` |
| `content` | TEXT | NOT NULL | — | メッセージ本文 |
| `metadata` | JSONB | NOT NULL | `'{}'` | 拡張情報（`session_id`、`thinking` など） |
| `created_at` | TIMESTAMPTZ | NOT NULL | `NOW()` | 作成日時 |

**外部キー**: `conversation_id` → `conversations(id) ON DELETE CASCADE`（会話削除時にメッセージも削除）

**インデックス**:
- `idx_messages_conversation_created ON messages(conversation_id, created_at ASC)` — 会話内のメッセージを時系列順で取得するクエリを高速化

**metadata の主なフィールド**:
- `session_id` (string): cc-remote-agent の Claude CLI セッション ID。`--resume` による会話継続に使用
- `thinking` (string): 拡張思考モード時の thinking コンテンツ

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
├── 001_create_conversations.sql   # conversations テーブル作成
└── 002_create_messages.sql        # messages テーブル作成
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
INSERT INTO messages (conversation_id, role, content, metadata)
VALUES ($1, $2, $3, $4)
RETURNING id, conversation_id, role, content, metadata, created_at
```
`CreateMessage(ctx, conversationID, role, content, metadata)` で呼び出す。`metadata` は `map[string]interface{}` を JSON シリアライズして格納。

**会話内メッセージ取得**
```sql
SELECT id, conversation_id, role, content, metadata, created_at
FROM messages WHERE conversation_id = $1 ORDER BY created_at ASC
```
`ListMessages(ctx, conversationID)` で呼び出す。時系列昇順で返す。

**updated_at 更新**
```sql
UPDATE conversations SET updated_at = NOW() WHERE id = $1
```
`UpdateConversationUpdatedAt(ctx, id)` で呼び出す。メッセージ送信後に会話のタイムスタンプを更新するために使用。

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
