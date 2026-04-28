# DBスキーマリファクタリング report

**タスクID**: subtask_db_schema_refactor_001  
**親コマンド**: cmd_cctunnel_db_schema_refactor_001  
**実施日**: 2026-04-19  

## 経緯・目的

messagesテーブルの設計を整理。`content` カラム（TEXT NOT NULL）を廃止し、
`metadata` カラムを `message_data` に改名。後方互換不要のため完全にクリーンな形に作り直した。

## 設計決定

- **content カラム削除**: user メッセージの content は `message_data.content` に格納。  
  assistant メッセージのテキストも `message_data.content` に格納（`content_blocks` に加えて）。
- **metadata → message_data 改名**: 用途が「メタデータ」ではなくメッセージデータ全体であるため、
  より意味が明確な名前に変更。

## 変更ファイル一覧

### DDL
- `apps/cc-tunnel/internal/db/migrations/002_create_messages.sql`
  - `content TEXT NOT NULL` 削除
  - `metadata JSONB` → `message_data JSONB`

### Go バックエンド
- `apps/cc-tunnel/internal/db/repository.go`
  - `Message` struct: `Content` フィールド削除, `Metadata` → `MessageData`
  - `CreateMessage`: `content` 引数削除, `metadata` → `messageData`
  - `ListMessages`: SQL・Scan 更新

- `apps/cc-tunnel/internal/api/handler.go`
  - user CreateMessage 呼び出し: `map[string]interface{}{"content": req.Content}` で呼び出し
  - session_id 取得: `history[i].Metadata` → `history[i].MessageData`
  - convHistory 構築: user は `MessageData["content"].(string)`、assistant は `content_blocks` を結合
  - assistant message 保存: `metadata` → `messageData`、`content` フィールドを追加
  - `dbMsgToAPI`: `Content` 削除、`Metadata` → `MessageData`

### OpenAPI
- `apps/openapi/openapi.yaml`
  - `Message` スキーマ: `content` 削除（required からも削除）、`metadata` → `message_data`

### 自動生成ファイル（再生成済み）
- `apps/cc-tunnel/internal/api/gen.go`: oapi-codegen で再生成
- `apps/frontend/src/api/schema.d.ts`: openapi-typescript で再生成

### フロントエンド
- `apps/frontend/src/components/MessageBubble.tsx`
  - `message.metadata` → `message.message_data`
  - `message.content` → `msgData?.content`

- `apps/frontend/src/components/ChatView.tsx`
  - `msg.metadata` → `msg.message_data`
  - old format fallback: `msg.content` → `meta?.content`

- `apps/frontend/src/App.tsx`
  - userMsg: `content` → `message_data: { content }`
  - assistantMsg: `content` フィールド削除
  - finally ブロック: `content` + `metadata` → `message_data` に統合

### ドキュメント
- `docs/database.md`: messages テーブルスキーマ・クエリ例を更新
- `docs/api.md`: Message 型の変更・メタデータセクション更新

## ビルド確認

- Go: `go build ./...` → **OK**
- npm: `npm run build` → **OK** (TypeScript エラーなし)
