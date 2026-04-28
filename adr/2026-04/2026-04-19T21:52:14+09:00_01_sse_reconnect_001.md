# SSE切断後の復帰メカニズム実装 報告

**タスクID**: subtask_sse_reconnect_001  
**親コマンド**: cmd_cctunnel_sse_reconnect_001  
**実施日時**: 2026-04-19T12:52:14+00:00  

---

## 実装サマリー

殿の3要件に対し、DBポーリング方式で実装完了:

1. **別会話選択/リロード時もバックエンドCLI実行継続** → 既実装済み (context.WithoutCancel)
2. **元の会話に戻った時、CLI実行中ならリアルタイム更新** → `useConversationPoller` フックで2秒間隔ポーリング実装
3. **CLI完了済みならDBから結果表示** → `status === 'completed'` 検知後ポーリング停止、既存DBロード利用

---

## TDDサイクル

### サイクル1: UpdateConversationStatus (repository)

**失敗テスト**: `interfaces.go` に `UpdateConversationStatus` を追加 → 既存モックが interface を未実装でコンパイルエラー

```
FAIL  github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/api [build failed]
*db.Repository does not implement repository (missing method UpdateConversationStatus)
```

**最小実装**:
- `003_add_conversation_status.sql`: `ALTER TABLE conversations ADD COLUMN status TEXT NOT NULL DEFAULT 'idle' CHECK (...)`
- `db.Conversation` 構造体に `Status string` 追加
- `GetConversation`, `ListConversations`, `CreateConversation` クエリに `status` カラム追加
- `UpdateConversationStatus(ctx, id, status)` メソッド追加

**全パス**: `go test ./... → ok internal/api`

### サイクル2: handler.go の status 更新フロー

**失敗テスト**: `TestSendMessage_StatusUpdatedToRunningThenCompleted`

```go
// statusHistory: expected ["running", "completed"] but got []
```

**最小実装** (handler.go):
```go
execCtx := context.WithoutCancel(r.Context())

if err := h.repo.UpdateConversationStatus(execCtx, convIDStr, "running"); err != nil {
    slog.Warn(...)
}
defer func() {
    if err := h.repo.UpdateConversationStatus(execCtx, convIDStr, "completed"); err != nil {
        slog.Warn(...)
    }
}()
```

**全パス**: `go test ./... → ok internal/api`

### サイクル3: フロントエンドポーリングフック

**失敗テスト**: `useConversationPoller.test.ts` (ファイル未存在でインポートエラー)

**最小実装**: `src/hooks/useConversationPoller.ts`
- `setInterval` で `intervalMs` 間隔ポーリング
- 最後のメッセージIDで差分検出
- `status === 'completed'` でポーリング停止

**テスト修正**: fake timer + async の組み合わせで `waitFor` タイムアウト → `flushMicrotasks()` で解決

**全パス**: `npx vitest run → 15 passed`

---

## 変更ファイル一覧

### バックエンド (Go)

| ファイル | 変更内容 |
|---------|---------|
| `apps/cc-tunnel/internal/db/migrations/003_add_conversation_status.sql` | 新規: status カラム追加マイグレーション |
| `apps/cc-tunnel/internal/db/repository.go` | Conversation.Status フィールド追加, クエリ更新, UpdateConversationStatus 追加 |
| `apps/cc-tunnel/internal/api/interfaces.go` | UpdateConversationStatus をインターフェースに追加 |
| `apps/cc-tunnel/internal/api/handler.go` | SendMessage に status='running'→'completed' 更新追加, dbConvToAPI に Status 追加 |
| `apps/cc-tunnel/internal/api/gen.go` | 再生成 (ConversationStatus 型追加) |
| `apps/openapi/openapi.yaml` | Conversation スキーマに status フィールド追加 |

### フロントエンド (TypeScript)

| ファイル | 変更内容 |
|---------|---------|
| `apps/frontend/src/api/schema.d.ts` | 再生成 (Conversation.status フィールド追加) |
| `apps/frontend/src/hooks/useConversationPoller.ts` | 新規: ポーリングフック |
| `apps/frontend/src/hooks/useConversationPoller.test.ts` | 新規: ポーリングフックテスト (5テスト) |
| `apps/frontend/src/App.tsx` | isPolling state追加, handleSelectConversation にポーリング開始ロジック追加, useConversationPoller 呼び出し |
| `apps/frontend/src/components/ChatView.tsx` | isPolling プロップ追加, 「処理中...」インジケータ表示 |

### ドキュメント

| ファイル | 変更内容 |
|---------|---------|
| `docs/architecture.md` | 復帰メカニズム設計セクション追記 |
| `docs/api.md` | Conversation.status フィールド説明, SSE切断後復帰フロー追記 |
| `docs/database.md` | conversations.status カラム追記, UpdateConversationStatus クエリ追記, migration 003 追記 |

---

## 最終 mise run check 成功出力

```
Test Files  4 passed (4)
      Tests  15 passed (15)
   Start at  12:52:05
   Duration  6.12s

0 issues. (golangci-lint)
ok  github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/api
```

全テスト・lint パス。SKIP=0。
