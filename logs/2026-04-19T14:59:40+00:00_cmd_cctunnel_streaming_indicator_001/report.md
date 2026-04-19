# report: subtask_streaming_indicator_001

**タスク**: ストリーミングインジケータ改善 — パルスドット表示  
**日時**: 2026-04-19T14:59:40+00:00

---

## TDDサイクル1: TypingIndicator コンポーネント作成

### 失敗テスト作成
`src/components/TypingIndicator.test.tsx` 新規作成:
- `renders 3 dots with animate-pulse class` → FAIL (コンポーネント未存在)
- `sets staggered animation delays on each dot (0s, 0.2s, 0.4s)` → FAIL

### 実装
`src/components/TypingIndicator.tsx` 新規作成:
- 3つの `span` 要素、各 `animate-pulse` クラス
- animationDelay: 0s / 0.2s / 0.4s
- カラー: `var(--color-text-muted)`
- `data-testid="typing-indicator"` 付与

### 結果
```
Test Files  1 passed (1)
      Tests  2 passed (2)
```

---

## TDDサイクル2: ChatView での TypingIndicator 統合

### 失敗テスト追加 (ChatView.test.tsx)
- `vi.mock('./TypingIndicator', ...)` を追加
- 新テスト6件追加:
  1. `shows TypingIndicator instead of empty bubble when isPolling=true and content_blocks is empty`
  2. `shows text content and TypingIndicator when isPolling=true and content_blocks has text`
  3. `does not show TypingIndicator when isPolling=false and isStreaming=false`
  4. `shows TypingIndicator for SSE streaming with empty streamBlocks`
  5. `shows content and TypingIndicator for SSE streaming with streamBlocks`

→ 4件 FAIL (確認済み)

### 実装 (ChatView.tsx の修正)
- `TypingIndicator` import 追加
- `isInProgress = isStreamingMsg || isPollingStreamingMsg` 追加
- `isEmptyBlocks` チェック追加 (blocks が空テキストのみ)
- `!isEmptyBlocks && blocks.map(...)` に変更
- 旧 `{isPollingStreamingMsg && <div>●●● 生成中...</div>}` を削除
- `{isInProgress && <TypingIndicator />}` に置換

### 旧テスト更新
- `shows empty text block with streaming animation...` → `shows TypingIndicator (not empty bubble)...` に更新
- `shows pulse indicator with 生成中...` → `shows TypingIndicator for status=streaming...` に更新

### 結果
```
Test Files  1 passed (1)
      Tests  13 passed (13)
```

---

## 最終 mise run check 成功出力

```
 Test Files  9 passed (9)
       Tests  39 passed (39)
    Start at  14:59:14
    Duration  10.88s
Finished in 16.39s
```

SKIP=0, 全テストパス。

---

## 変更点まとめ

| ファイル | 変更内容 |
| ------- | ------- |
| `src/components/TypingIndicator.tsx` | 新規作成: パルスドットコンポーネント |
| `src/components/TypingIndicator.test.tsx` | 新規作成: 2テスト |
| `src/components/ChatView.tsx` | TypingIndicator import、旧pulse div削除、新ロジック追加 |
| `src/components/ChatView.test.tsx` | vi.mock追加、旧2テスト更新、新6テスト追加 |
| `docs/frontend.md` | TypingIndicator仕様セクション追加 |
