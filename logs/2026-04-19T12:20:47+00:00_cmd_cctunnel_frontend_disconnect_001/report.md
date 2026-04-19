# subtask_frontend_disconnect_001 レポート

## 現状の問題点

`App.tsx` / `client.ts` を読んで確認した問題:

1. **`sendMessage` に AbortSignal がなかった** — fetch をキャンセルする手段がなく、セッション切替後もストリームが継続していた。
2. **`handleSelectConversation` が SSE を abort しない** — state はリセットされるが、旧セッションの SSE コールバックが引き続き state を更新する可能性があった。
3. **`handleSend` の `finally` が無条件に state を更新** — セッション切替後にも `setMessages` / `setSending` が呼ばれ、新セッションの表示が汚染されるリスクがあった。
4. **アンマウント時のクリーンアップがない** — コンポーネント破棄時に接続が残る。

## TDD サイクル

### Cycle 1: sendMessage AbortSignal 対応

**Red** — `src/__tests__/client.test.ts` を作成:
- `passes signal to fetch`
- `returns without calling onEvent when signal is pre-aborted`
- `handles AbortError thrown by reader.read() gracefully`

→ 3 tests FAIL (signal パラメータなし)

**Green** — `client.ts` 修正:
- `signal?: AbortSignal` パラメータ追加
- `fetch()` に `signal` を渡す
- `isAbortError()` helper を追加
- fetch / reader.read() の AbortError を catch して return

→ 3 tests PASS

### Cycle 2: useSSEAbort フック

**Red** — `src/hooks/useSSEAbort.test.ts` を作成:
- `startStream returns a non-aborted signal initially`
- `startStream aborts the previous controller when called again`
- `switchSession aborts the current controller`
- `isActiveSession returns true for the current session`
- `isActiveSession returns false for old session after switchSession`
- `aborts the current controller on unmount`

→ 6 tests FAIL (ファイルなし)

**Green** — `src/hooks/useSSEAbort.ts` を作成:
- `startStream` / `switchSession` / `isActiveSession` を実装
- `useEffect` cleanup で自動 abort

→ 6 tests PASS

### リファクタ: lint 修正

`no-unsafe-finally` エラー修正 — `finally` 内の `return` を `if (isActiveSession)` ブロックに変換。

## 変更点サマリ

| ファイル | 変更内容 |
| -------- | -------- |
| `src/api/client.ts` | `sendMessage` に `signal?: AbortSignal` 追加、AbortError ハンドリング |
| `src/hooks/useSSEAbort.ts` | 新規作成: AbortController ライフサイクル管理フック |
| `src/App.tsx` | `useSSEAbort` 統合: セッション切替時 abort、コールバック/finally ガード |
| `vite.config.ts` | `test.environment: 'jsdom'` 追加 |
| `package.json` | `@testing-library/react`, `jsdom` devDependency 追加 |
| `src/__tests__/client.test.ts` | 新規: sendMessage abort テスト (3件) |
| `src/hooks/useSSEAbort.test.ts` | 新規: useSSEAbort テスト (6件) |
| `docs/frontend.md` | SSE 切断耐性設計セクション追加 |

## 最終 mise run check 成功出力

```
[test:frontend]  Test Files  3 passed (3)
[test:frontend]       Tests  10 passed (10)
[test:frontend]    Start at  12:20:21
[test:frontend]    Duration  4.44s

Finished in 16.67s
```

全チェック (lint + test: frontend / cc-tunnel / cc-remote-agent) PASS。
