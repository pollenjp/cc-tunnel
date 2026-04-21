# ポーリング停止バグ修正レポート

## タスク: subtask_poller_stop_bug_001

## 根本原因

**仮説A が正解**

`handler.go` の `GetConversation` ハンドラが `ConversationDetail` を構築する際に `Status` フィールドを設定していなかった。

```go
// 修正前（バグあり）
detail := ConversationDetail{
    Id:           convUUID,
    Title:        conv.Title,
    Model:        conv.Model,
    CreatedAt:    conv.CreatedAt,
    UpdatedAt:    conv.UpdatedAt,
    SystemPrompt: conv.SystemPrompt,
    Messages:     make([]Message, 0, len(msgs)),
    // Status が未設定 → ゼロ値 "" が返される
}
```

結果として:
1. `GET /conversations/:id` が `"status": ""` を返す
2. フロントエンドの `useConversationPoller` が `detail.status === 'completed'` を確認
3. `""` !== `'completed'` → 永遠に false
4. `stoppedRef.current = true` が呼ばれない
5. `onCompleted()` が呼ばれない
6. `setIsPolling(false)` が呼ばれない
7. ポーリングが止まらない → TypingIndicator が消えない

## 修正内容

**ファイル**: `apps/cc-tunnel/internal/api/handler.go`

```go
// 修正後
detail := ConversationDetail{
    Id:           convUUID,
    Title:        conv.Title,
    Model:        conv.Model,
    Status:       ConversationDetailStatus(conv.Status),  // 追加
    CreatedAt:    conv.CreatedAt,
    UpdatedAt:    conv.UpdatedAt,
    SystemPrompt: conv.SystemPrompt,
    Messages:     make([]Message, 0, len(msgs)),
}
```

## TDDサイクル

1. **失敗テスト追加** (`apps/cc-tunnel/internal/api/handler_test.go`)
   - `TestGetConversation_returnsStatus`: `status="completed"` の会話を GET したとき、レスポンスに `"status":"completed"` が含まれることを検証
   - 実行結果: `Status = "", want "completed"` → FAIL（根本原因実証）

2. **実装修正** (`handler.go` に `Status` フィールド追加)

3. **テスト再実行**: PASS

## 最終テスト結果 (mise run check)

```
[test:cc-tunnel]  ok   github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/api   0.063s
[test:frontend]   Test Files  9 passed (9)
[test:frontend]         Tests  50 passed (50)
```

SKIP=0、全テストパス。

## 注記

- フロントエンドの `useConversationPoller.ts` の実装は正しく、バグは純粋にバックエンド側にあった
- 既存の `useConversationPoller.test.ts` テストはモックを使うため、バックエンドのバグを検出できなかった
- 今回追加したバックエンドの統合テストにより、同様のリグレッションを防止できる
