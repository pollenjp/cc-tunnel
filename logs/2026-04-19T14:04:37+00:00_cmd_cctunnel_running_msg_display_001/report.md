# subtask_running_msg_display_001 実装報告

## 概要

会話復帰時の assistant 生成中表示を実装。TDD サイクル 2 本で完了。

## バックエンド確認・修正

### 問題
`handler.go` の `dbMsgToAPI` 関数で `Status` フィールドが設定されていなかった。

```go
// 修正前
msg := Message{
    Id:             msgID,
    ConversationId: convID,
    Role:           MessageRole(m.Role),
    CreatedAt:      m.CreatedAt,
}

// 修正後
if m.Status != "" {
    status := MessageStatus(m.Status)
    msg.Status = &status
}
```

これにより、フロントエンドが `GetConversation` でメッセージを取得したとき `msg.status` が正しく返されるようになった。

## TDD サイクル 1: メッセージバブルへの生成中インジケータ

### 失敗テスト追加 (ChatView.test.tsx)
- `shows pulse indicator with 生成中... text in message bubble for status=streaming when isPolling is true`
- `does not show pulse indicator for status=streaming when isPolling is false`

### 実装 (ChatView.tsx)
`blocks.map` の後にパルスインジケータを追加:

```tsx
{isPollingStreamingMsg && (
  <div className="flex items-center gap-1 px-4 py-1 text-xs text-[var(--color-text)]">
    <span className="animate-pulse">●</span>
    <span className="animate-pulse" style={{ animationDelay: '0.2s' }}>●</span>
    <span className="animate-pulse" style={{ animationDelay: '0.4s' }}>●</span>
    <span className="ml-1 text-[var(--color-text-muted)]">生成中...</span>
  </div>
)}
```

## TDD サイクル 2: tool_use ブロックのポーリング中レンダリング

### 失敗テスト追加 (ChatView.test.tsx)
- `renders ToolCallCard for tool_use block when status=streaming and isPolling is true`
- `does not render ToolCallCard for tool_use when tool_calls data is missing during polling`

### 実装 (ChatView.tsx)
`isPollingStreamingMsg` ブランチで `tool_use` ブロックも処理するよう変更:

```ts
const toolCallsData = (meta?.tool_calls as ToolCallData[] | undefined) ?? [];
const toolCallMap = new Map(toolCallsData.map(tc => [tc.tool_use_id, tc]));
blocks = contentBlocks && contentBlocks.length > 0
  ? contentBlocks.flatMap((cb): AssistantBlock[] => {
      if (cb.type === 'thinking') return [{ type: 'thinking', content: cb.content }];
      if (cb.type === 'text') return [{ type: 'text', content: cb.content }];
      const tc = toolCallMap.get(cb.tool_use_id);
      if (!tc) return [];
      return [{ type: 'tool', toolCall: { ..., isRunning: true } }];
    })
  : [{ type: 'text', content: '' }];
```

## 最終 mise run check 結果

```
Test Files  5 passed (5)
      Tests  24 passed (24)
   Start at  14:04:25
   Duration  7.98s

lint: 0 issues (cc-tunnel, cc-remote-agent, frontend)
```

SKIP=0 で全テストパス。
