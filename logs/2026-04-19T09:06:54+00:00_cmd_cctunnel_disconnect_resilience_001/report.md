# cmd_cctunnel_disconnect_resilience_001 実装レポート

## 根本原因の分析

`handler.go` の `SendMessage()` はフロントエンド（ブラウザ）が切断されると失敗していた。

**原因**:
- net/http はクライアント切断時に `r.Context()` をキャンセルする
- `h.remote.Execute(r.Context(), ...)` — キャンセルで cc-remote-agent への HTTP 接続が切れ Claude CLI が中断
- `h.repo.CreateMessage(r.Context(), "assistant", ...)` — キャンセルで `context.Canceled` エラー → DB 保存失敗

## TDD サイクル

### Cycle 1: SSE切断後もDB保存が完了すること

**失敗テスト作成**:
```
TestSendMessage_ContextCancelledDuringExecution_AssistantMessageSaved
```
- `mockRemoteWithCancel.Execute()` が r.Context() をキャンセルしてから完了
- `mockRepoCheckCtx.CreateMessage()` が ctx.Done() をチェックし、キャンセルなら error を返す
- `repo.assistantMsgSaved` が false のままであることを確認

**失敗確認**:
```
ERROR failed to save assistant message err="context canceled"
FAIL: assistant message was NOT saved to DB after frontend disconnect
```

**実装**:
```go
execCtx := context.WithoutCancel(r.Context())
h.remote.Execute(execCtx, ...)
h.repo.CreateMessage(execCtx, convIDStr, "assistant", messageData)
h.repo.UpdateConversationUpdatedAt(execCtx, convIDStr)
```

**成功確認**: `--- PASS: TestSendMessage_ContextCancelledDuringExecution_AssistantMessageSaved`

### Cycle 2: ctx.Done() でCLI実行が止まらないこと

**失敗テスト作成**:
```
TestSendMessage_ExecuteContextIsIndependentOfRequestContext
```
- `mockRemoteWithCancelAndCtxCheck.Execute()` が r.Context() をキャンセルしてから
  自分が受け取った ctx が Done() かどうかをチェック
- `remote.executeCtxCancelledAtEntry` が true だったら FAIL

**失敗確認**:
```
FAIL: Execute received a context that was already cancelled
Got: Execute received r.Context() which was cancelled by the simulated disconnect
```

**実装**: 同上 (execCtx := context.WithoutCancel)

**成功確認**: `--- PASS: TestSendMessage_ExecuteContextIsIndependentOfRequestContext`

## 変更点サマリ

| ファイル | 変更内容 |
|---------|---------|
| `internal/api/interfaces.go` (新規) | `repository` / `remoteClient` インターフェース定義 |
| `internal/api/handler.go` | `Server` 構造体をインターフェース型に変更、`context` import 追加、`execCtx` 導入 |
| `internal/api/sendmessage_test.go` (新規) | 2つのTDDテストとモック実装 |
| `docs/architecture.md` | 切断耐性設計セクション追加 |

## 最終 mise run check 成功出力

```
[test:cc-tunnel] ok   github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/api
[lint:cc-tunnel] 0 issues.
[lint:cc-remote-agent] 0 issues.
[test:frontend]  Test Files  1 passed (1)
[lint:frontend]  eslint 0 issues
Finished in 8.18s
```

全テスト PASS、lint 0 issues。
