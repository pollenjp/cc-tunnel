# subtask_typing_on_reconnect_001 実装レポート

## 概要

会話復帰時（ポーリング中）に TypingIndicator が表示されないバグを修正。
`isPolling` フラグのタイミングズレ（race condition）に起因する問題を、
`isRunning` prop（メッセージの `status=streaming` を直接検知）で解決した。

## TDD サイクル1: ChatView の isRunning prop + TypingIndicator 表示

### 失敗テスト追加（ChatView.test.tsx）

- `isRunning=true, isPolling=false` でも TypingIndicator が表示されること（reconnect race condition）
- `isRunning=true, isPolling=false, message_data={}` でも TypingIndicator が表示されること
- `isRunning=false, isPolling=false, isStreaming=false` では TypingIndicator が表示されないこと

### 実装（ChatView.tsx）

`ChatViewProps` に `isRunning?: boolean` を追加。

`isInProgress` の条件を以下のように更新:

```ts
// Before
const isInProgress = isStreamingMsg || isPollingStreamingMsg;
// After
const isInProgress = isStreamingMsg || isPollingStreamingMsg || isRunning === true;
```

## TDD サイクル2: App.tsx の hasStreamingMessage 検知

### 失敗テスト追加（App.test.tsx）

- `messages` に `status=streaming` のメッセージがある場合、ChatView に `isRunning=true` が渡されること
- `messages` に `status=streaming` のメッセージがない場合、ChatView に `isRunning=false` が渡されること

ChatView モックを更新して `isRunning` prop を `data-is-running` 属性として露出。

### 実装（App.tsx）

```ts
const hasStreamingMessage = messages.some(m => m.status === 'streaming');
```

ChatView に `isRunning={sending || hasStreamingMessage}` を渡す。

## 変更ファイル

| ファイル | 変更内容 |
| -------- | -------- |
| `apps/frontend/src/components/ChatView.tsx` | `isRunning?: boolean` prop 追加、`isInProgress` 条件更新 |
| `apps/frontend/src/components/ChatView.test.tsx` | 新テスト3件追加 |
| `apps/frontend/src/__tests__/App.test.tsx` | ChatViewモック更新、新テスト2件追加 |
| `apps/frontend/src/App.tsx` | `hasStreamingMessage` 計算追加、ChatView に `isRunning` prop 追加 |
| `docs/frontend.md` | TypingIndicator 表示条件・ChatView Props テーブル更新 |

## 最終 mise run check 出力

```
[lint:frontend] Finished in 12.76s
[test:frontend]  Test Files  9 passed (9)
[test:frontend]       Tests  45 passed (45)
[test:frontend]    Start at  23:27:59
[test:frontend]    Duration  10.52s
SKIP=0, FAIL=0
```
