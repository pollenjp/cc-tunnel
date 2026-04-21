# cmd_cctunnel_polling_tool_display_001 Report

## 根本原因

ポーリング中（SSE切断後、DBポーリングで会話を復元する状態）にtool_useブロックが表示されない問題。

### バックエンド

5秒バッチ（ticker goroutine）は `content_blocks` のみ保存し、`tool_calls` は保存しなかった。

```
contentBlocksList → UpdateMessageContentBlocks (バッチ: 毎5秒)
toolCallsData     → MergeMessageData (完了時のみ)
```

ポーリング中クライアントが `GET /conversations/{id}` でメッセージを取得すると、
`message_data.tool_calls` は空のため、フロントエンドがtool_useをスキップしていた。

### フロントエンド

`ChatView.tsx` の `isPollingStreamingMsg` ブランチ:

```ts
const tc = toolCallMap.get(cb.tool_use_id);
if (!tc) return [];  // tool_calls未保存 → スキップ
```

## TDDサイクル

### TDD Cycle 1 (Backend)

**失敗テスト**: `TestSendMessage_BatchTickerSavesToolCalls`
- `mockRemoteSlowExec` (30ms sleep) + `batchInterval: 1ms`
- バッチticker goroutineが `MergeMessageData("tool_calls")` を呼ぶことを確認
- 条件: `MergeMessageData` に "tool_calls" キーが含まれる呼び出しが ≥ 2回

**実装**:
- `Server.batchInterval` フィールド追加（0=5s デフォルト、テスト時は短縮可能）
- `cloneToolCalls()` ヘルパー追加
- バッチ goroutine に `MergeMessageData({tool_calls: snapshotTools})` 追加

### TDD Cycle 2 (Frontend)

**失敗テスト**: 既存テスト「does not render ToolCallCard...」を変更
- 新しい期待: `tool_calls` 未保存でもプレースホルダー `ToolCallCard` が表示される

**実装**: `ChatView.tsx` の `isPollingStreamingMsg` ブランチでフォールバック追加
- `tc` が undefined の場合: `{toolUseId: cb.tool_use_id, toolName: '', inputJson: '', isRunning: true}`

## 変更ファイル

- `apps/cc-tunnel/internal/api/handler.go`
  - `Server` 構造体に `batchInterval time.Duration` 追加
  - バッチ goroutine に `cloneToolCalls` + `MergeMessageData` 追加
  - `cloneToolCalls()` ヘルパー追加

- `apps/cc-tunnel/internal/api/sendmessage_test.go`
  - `mockRepoCheckCtx.mergeDataHistory` 追加
  - `MergeMessageData` モックで呼び出し記録
  - `mockRemoteSlowExec` 追加
  - `makeToolUseStartStreamEvent` ヘルパー追加
  - `TestSendMessage_BatchTickerSavesToolCalls` テスト追加

- `apps/frontend/src/components/ChatView.tsx`
  - `isPollingStreamingMsg` ブランチでフォールバック実装

- `apps/frontend/src/components/ChatView.test.tsx`
  - 既存テスト「does not render...」→「renders placeholder...」に変更

## 最終 mise run check 成功出力

```
[test:cc-tunnel] ok  github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/api  0.062s
[test:frontend]  Test Files  9 passed (9)
[test:frontend]       Tests  39 passed (39)
[lint:cc-tunnel] 0 issues.
[lint:frontend] (no issues)
[lint:cc-remote-agent] 0 issues. (SKIP=0)
```

全テスト SKIP=0 でパス。
