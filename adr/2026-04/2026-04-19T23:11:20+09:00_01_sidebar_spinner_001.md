# subtask_sidebar_spinner_001 実施報告

## 変更点

### apps/frontend/src/components/Sidebar.tsx
- 会話リスト各アイテムのタイトル `<span>` を `flex items-center gap-1.5 min-w-0` に変更
- `conv.status === 'running'` の場合、タイトル左側にスピナーを追加
  - クラス: `shrink-0 inline-block h-3 w-3 rounded-full border-2 border-[var(--color-accent)] border-t-transparent animate-spin`
  - ログアウトボタンのスピナーと同じデザイン
- タイトルテキストを `<span className="overflow-hidden text-ellipsis whitespace-nowrap">` で内包

### apps/frontend/src/components/Sidebar.test.tsx (新規)
- status=running の会話にスピナーが表示されること
- status=idle の会話にスピナーが表示されないこと
- status=completed の会話にスピナーが表示されないこと
- 混在リストで running の会話のみスピナー1個が表示されること

### docs/frontend.md
- Sidebar コンポーネントの説明にスピナー表示仕様テーブルを追記

## mise run check 最終結果

```
[test:frontend]  Test Files  6 passed (6)
[test:frontend]       Tests  28 passed (28)
[test:frontend]    Start at  14:10:50
[test:frontend]    Duration  8.17s
[lint:frontend] Finished in 13.66s
Finished in 14.09s
```

SKIP=0、全テストパス。
