# subtask_sidebar_spinner_bug_002 完了報告

## 根本原因

**仮説A 確定**: ChatView 自己完結化リファクタリング後、App.tsx の楽観的 `status='running'` 更新が消えた。

### フロー比較

**旧フロー (App.tsx が handleSend を持っていた時)**:
```
handleSend → setConversations(... status:'running') → hasRunning=true
→ useConversationListPoller がポーリング開始 → サイドバーのスピナー表示
```

**新フロー（ChatView が handleSend を持つ）**:
```
ChatView.handleSend → setIsPolling(true)（ChatView 内部のみ更新）
→ App.tsx の conversations state は更新されない
→ hasRunning = conversations.some(c => c.status === 'running') = false のまま
→ useConversationListPoller はポーリングしない
→ サイドバーのスピナーが表示されない ← バグ
```

バックエンド（仮説B）・ポーラー判定（仮説C）は問題なし:
- `mapping.go`: `newConversation` が `Status: ConversationStatus(c.Status)` を正しく設定
- `ListConversations` → `dbConvToAPI` → `newConversation` で status フィールドを返却済み

## 修正内容

### ChatView.tsx
- `ChatViewProps` に `onSendStart?: () => void` を追加
- `handleSend` の冒頭で `onSendStart?.()` を呼ぶ（送信開始を App.tsx に通知）

### App.tsx
- `ChatView` に `onSendStart` コールバックを追加
- コールバック内で `setConversations` により `selectedId` の会話を楽観的に `status='running'` に更新
- これにより `hasRunning=true` → `useConversationListPoller` がポーリング開始 → サイドバースピナー表示

## 変更ファイル

- `apps/frontend/src/components/ChatView.tsx`: `onSendStart` prop 追加
- `apps/frontend/src/App.tsx`: `onSendStart` コールバック追加
- `apps/frontend/src/components/ChatView.test.tsx`: `[Cycle3]` 失敗テスト追加
- `apps/frontend/src/__tests__/App.test.tsx`: mock 更新 + 失敗テスト追加

## TDDサイクル

1. **Red**: 2テスト追加・FAIL 確認
   - `[Cycle3] メッセージ送信開始時に onSendStart が呼ばれること`
   - `onSendStart が呼ばれると useConversationListPoller に hasRunning=true が渡されること`
2. **Green**: 実装後 PASS 確認
3. **Refactor**: 変更なし（最小限の実装で十分）

## mise run check 最終結果

```
Test Files  9 passed (9)
Tests  52 passed (52)
SKIP=0（全テストパス）
```
