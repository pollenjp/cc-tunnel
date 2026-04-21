# subtask_db_driven_state_001_phase12 実装レポート

## 概要

DB駆動状態管理 Phase 1-2 — DB基盤 + handler変更 — TDD必須

## TDDサイクル記録

### Cycle 1: Repository インターフェース（コンパイル失敗確認）

**赤フェーズ**: `interfaces.go` に新メソッド4つを追加 → `*db.Repository` が interface を満たさずコンパイルエラー

```
internal/api/handler.go:26:23: cannot use repo (variable of type *db.Repository) as repository value in struct literal: *db.Repository does not implement repository (missing method CreateStreamingMessage)
```

**緑フェーズ**: 以下を実装
- `migrations/004_add_message_status.sql` 作成
- `db.Message` struct に `Status`, `UpdatedAt` フィールド追加
- `repository.go` に `CreateStreamingMessage`, `UpdateMessageContentBlocks`, `UpdateMessageStatus`, `MergeMessageData` 実装
- `ListMessages` を status/updated_at を含む SELECT に更新

### Cycle 2: 早期メッセージ作成（テスト失敗確認）

**赤フェーズ**: `TestSendMessage_StreamingMessageCreatedAtStart` 追加 → FAIL

```
FAIL: CreateStreamingMessage was NOT called
Expected: assistant message created with status='streaming' before Execute
```

**緑フェーズ**: `SendMessage()` 内 `execCtx` 定義直後に `CreateStreamingMessage` 呼び出し追加

### Cycle 3: 定期バッチ保存（テスト失敗確認）

**赤フェーズ**: `TestSendMessage_UpdateContentBlocksCalledOnCompletion`, `TestSendMessage_MessageStatusCompletedOnSuccess` 追加 → FAIL

**緑フェーズ**: goroutine + ticker (5s) 実装、完了時の最終保存ロジック追加

## 変更ファイル一覧

| ファイル | 変更内容 |
|---------|---------|
| `internal/db/migrations/004_add_message_status.sql` | 新規作成: messages.status, messages.updated_at |
| `internal/db/repository.go` | Message struct 更新 + 4新メソッド + ListMessages 更新 |
| `internal/db/db.go` | 孤児クリーンアップ追加 (NewPool + cleanupOrphanedStreamingMessages) |
| `internal/api/interfaces.go` | repository interface に4新メソッド追加 |
| `internal/api/handler.go` | Phase 2 実装 (sync, ticker, final save, error handling) |
| `internal/api/sendmessage_test.go` | mock 拡張 + 3新テスト追加 |
| `apps/openapi/openapi.yaml` | Message schema に status, updated_at 追加 |
| `internal/api/gen.go` | go generate で再生成 |
| `apps/frontend/src/api/schema.d.ts` | openapi-typescript で再生成 |
| `docs/database.md` | messages テーブル仕様更新 + 新クエリ記載 |
| `docs/architecture.md` | DB駆動状態管理セクション追加 |

## 最終 mise run check 結果

```
cc-tunnel: test ok (SKIP=0), lint 0 issues
cc-remote-agent: test ok, lint 0 issues
frontend: 4 passed (15 tests), lint 0 vulnerabilities
Finished in ~12s
```

## 設計メモ

- 毎イベント UPDATE は禁止（JSONB 書き換えコスト）→ 5秒バッチのみ
- migration 004 の DEFAULT は 'completed'（既存レコード後方互換）
- goroutine + ticker + sync.Mutex で contentBlocksList を排他制御
- 起動時孤児クリーンアップ: status='streaming' かつ 30分以上前 → 'error'
- SSE の既存コードは変更なし（DB 書き込みを追加しただけ）
