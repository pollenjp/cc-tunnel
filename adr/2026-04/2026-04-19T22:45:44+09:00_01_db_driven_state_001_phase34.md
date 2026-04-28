# subtask_db_driven_state_001_phase34 実装レポート

## 概要

DB駆動状態管理 Phase 3-4: フロントエンド更新 (TDD必須)

## TDDサイクル4: useConversationPoller フル上書き対応

### 変更ファイル

- `apps/frontend/src/hooks/useConversationPoller.ts`
- `apps/frontend/src/hooks/useConversationPoller.test.ts`
- `apps/frontend/src/App.tsx`

### インターフェース変更

```ts
// 変更前
export interface UseConversationPollerOptions {
  conversationId: string | null;
  isRunning: boolean;
  lastKnownMessageId?: string;
  onNewMessages: (messages: Message[]) => void;
  onCompleted: () => void;
  intervalMs?: number;
}

// 変更後
export interface UseConversationPollerOptions {
  conversationId: string | null;
  isRunning: boolean;
  onMessages: (messages: Message[]) => void;  // 全メッセージ（差分でなく全置換）
  onCompleted: () => void;
  intervalMs?: number;
}
```

### 動作変更

- 旧: `lastKnownMessageId`以降の差分のみ取得して `onNewMessages` に渡す
- 新: 毎ポーリングで全メッセージを取得して `onMessages` に渡す（同一IDでも再送）

### 追加テスト（失敗→実装→通過）

1. `calls onMessages with full message list on every poll when running`
2. `calls onMessages on every poll even when message ids are unchanged (streaming update)`
3. `calls onMessages with all messages then onCompleted when status becomes completed`（順序保証テスト含む）

## TDDサイクル5: ChatView ストリーミング表示 + エラー表示

### 変更ファイル

- `apps/frontend/src/components/ChatView.tsx`
- `apps/frontend/src/components/ChatView.test.tsx` (新規)

### 実装内容

1. `isPollingStreamingMsg = isPolling === true && msg.status === 'streaming'`
   - `message_data.content_blocks` があればDBの部分コンテンツを表示
   - なければ空テキストブロック
   - ストリーミングアニメーション（`isStreaming=true`）を付与

2. `msg.status === 'error'` のメッセージに赤いエラーバッジを表示
   - テキスト: 「エラーが発生しました」

### 追加テスト（失敗→実装→通過）

1. `shows content_blocks text with streaming animation for status=streaming message when isPolling is true`
2. `shows empty text block with streaming animation for status=streaming message with no content_blocks when isPolling is true`
3. `shows error display for status=error message`
4. `does not apply streaming animation for status=streaming message when isPolling is false`

## 最終 mise run check 成功出力

```
Test Files  5 passed (5)
      Tests  20 passed (20)
   Start at  13:46:29
   Duration  6.49s
```

SKIP=0、全テストパス。

## docs更新

`docs/frontend.md` に以下を追記:
- ChatView Props に `isPolling` を追加
- ChatView メッセージ表示の優先順位を追加
- `useConversationPoller` API変更・動作仕様・streaming表示・エラー表示の仕様を新セクションとして追加
