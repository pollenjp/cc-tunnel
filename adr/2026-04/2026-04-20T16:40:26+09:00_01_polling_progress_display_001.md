# subtask_polling_progress_display_001 報告

## 根本原因

### バックエンド調査結果

`handler.go` の `dbMsgToAPI` は `message_data` の `content_blocks` を正しく返す（非空の場合）。

バッチ ticker（2秒間隔）が `contentBlocksList` を定期保存する設計だが、`contentBlocksList` への追記は `assistant` イベント（完了メッセージ）到着時のみ行われる。`stream_event` の `text_delta` は蓄積されない。そのため **streaming 中は content_blocks が空** となる。

→ バックエンドのアーキテクチャ上、ポーリングで途中経過が見えるタイミングは `IncludePartialMessages=true` で `assistant` イベントが複数回届く場合のみ。

### フロントエンド調査結果（ChatView.tsx）

**修正前の問題**: `isInProgress` の条件が `isPollingStreamingMsg || isRunning === true` であった。

`isPollingStreamingMsg` = `isPolling === true && msg.status === 'streaming'`

これにより、`isPolling=true` だが `isRunning` が渡されていないテストでも `isInProgress=true` となり、TypingIndicator が表示されていた。実際のアプリでは `isPolling=true` → `isRunning=true` は常に成立するが、テストが実態と乖離していた。

## 修正内容

### ChatView.tsx

```diff
- const isInProgress = isPollingStreamingMsg || isRunning === true;
+ const isInProgress = isRunning === true;
```

`isInProgress` を `isRunning === true` に統一。実際の App.tsx では `isRunning = sending || isPolling || hasStreamingMessage` であり、`isPolling=true` のとき必ず `isRunning=true` となるため動作に変化なし。ロジックをシンプル化。

### ChatView.test.tsx (TDD)

新規追加テスト（3件）:
1. `shows content_blocks text when isRunning=true and isPolling=false` — isRunning=true + content_blocks あり → テキスト表示
2. `shows TypingIndicator only (no bubble) when isRunning=true and content_blocks is empty` — isRunning=true + empty → TypingIndicator のみ、bubble なし
3. `shows TypingIndicator after content when isRunning=true and content_blocks has data` — isRunning=true + content_blocks あり → テキスト + TypingIndicator

既存テスト修正（4件）: `isPolling={true}` を含むテストに `isRunning={true}` を追加（実際の App.tsx の動作に合わせる）。

### docs/frontend.md

- TypingIndicator `isInProgress` 定義を `isRunning === true` に更新
- ChatView Props テーブルから削除済みの SSE 関連 props（`isStreaming`, `streamMeta`, `hookEvents`, `streamBlocks`）を削除
- メッセージ表示優先順位から SSE パスを削除
- streaming メッセージのレンダリングロジックセクションを追加

## テスト結果

```
Test Files  9 passed (9)
      Tests  44 passed (44)  (SKIP=0)
```

## 備考

**実際の途中経過表示について**: 現状、バックエンドは `text_delta` イベントを蓄積しないため、ポーリングで途中テキストを確認できる窓口は限定的。`IncludePartialMessages=true` での `assistant` 複数イベントが届く場合のみ。フロントエンドのレンダリング自体は正しく実装済み。
