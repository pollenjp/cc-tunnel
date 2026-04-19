# サイドバースピナー不表示バグ修正レポート

## タスク ID
subtask_sidebar_spinner_debug_001

## 根本原因

バックエンドは正常に `status='running'` を返していたが、フロントエンドのタイミングに問題があった:

1. `handleSend` が sendMessage を await し始める前に、`refreshConversations()` は呼ばれない
2. sendMessage 完了後（CLI が終わった後）に `refreshConversations()` が呼ばれるが、その時点では既に `status='completed'` になっている
3. → サイドバーは `'running'` 状態を全く受け取れない

## TDD サイクル 1: 楽観的 status 更新

### 失敗テスト (`src/__tests__/App.test.tsx`)
- sendMessage 呼び出し後すぐ（awaiting 中）に、サイドバーの会話リストで当該会話の status が `'running'` になっていること

### 実装 (`src/App.tsx`)
`handleSend` の冒頭（`await sendMessage(...)` の前）に楽観的更新を追加:

```ts
setConversations(prev =>
  prev.map(c => c.id === selectedId ? { ...c, status: 'running' as const } : c)
);
```

送信完了後の `refreshConversations()` で DB の実際の値（`'completed'`）に戻る。

## TDD サイクル 2: conversations ポーリング

### 失敗テスト (`src/hooks/useConversationListPoller.test.ts`)
- `hasRunning=true` のとき 3 秒ごとに `onPoll` が呼ばれること
- `hasRunning=false` のとき `onPoll` が呼ばれないこと
- `hasRunning` が `true→false` に変わるとポーリングが停止すること

### 実装 (`src/hooks/useConversationListPoller.ts`)
新規フック作成。`hasRunning` が `true` の間 `setInterval` でポーリング。

### App.tsx への統合
```ts
const hasRunning = conversations.some(c => c.status === 'running');
useConversationListPoller({
  hasRunning,
  onPoll: refreshConversations,
});
```

## 変更ファイル
- `apps/frontend/src/App.tsx` — 楽観的更新 + `useConversationListPoller` 追加
- `apps/frontend/src/hooks/useConversationListPoller.ts` — 新規フック
- `apps/frontend/src/hooks/useConversationListPoller.test.ts` — 新規テスト (TDD Cycle 2)
- `apps/frontend/src/__tests__/App.test.tsx` — 新規テスト (TDD Cycle 1)
- `docs/frontend.md` — サイドバースピナー制御仕様を追記

## 最終 mise run check 結果

```
Test Files  8 passed (8)
     Tests  32 passed (32)
  SKIP      0
```

lint: CLEAN
typecheck: PASS
test: 32/32 PASS, SKIP=0
