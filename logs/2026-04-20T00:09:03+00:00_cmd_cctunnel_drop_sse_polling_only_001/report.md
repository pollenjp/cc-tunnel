# subtask_drop_sse_polling_only_001 実施報告

## 概要
SSE直接受信を廃止し、DBポーリング一本化を5サイクルTDDで実施。

## Cycle 1: OpenAPI契約変更

### 変更内容
- `apps/openapi/openapi.yaml`
  - POST /messages レスポンス: `200 text/event-stream` → `202 application/json`
  - `SendMessageResponse` スキーマ追加（`message_id: uuid`）
  - SSEスキーマ13種削除（SSETextEvent, SSEThinkingEvent, SSETextDeltaEvent, SSEThinkingDeltaEvent, SSEToolUseStartEvent, SSEToolInputDeltaEvent, SSEToolResultEvent, SSEInitEvent, SSEHookEvent, SSERateLimitEvent, SSECostEvent, SSEDoneEvent, SSEErrorEvent）
  - POST /messages のSSE説明文を削除

### コード再生成
- Go: `go generate ./internal/api/...` → `gen.go` 更新（SSE型削除、SendMessageResponse追加）
- TypeScript: `npx openapi-typescript` → `schema.ts` 更新

---

## Cycle 2: Backend handler変更

### TDD: 失敗テスト先行

**mapping_test.go**:
- SSEイベント直列化テスト8件を全削除（型が存在しなくなるため）

**sendmessage_test.go**:
- `TestSendMessage_Returns202WithMessageID` 追加
- 全テストに `doneCh chan struct{}` を追加（goroutine完了同期のため）
- レスポンスコード確認を202に更新

### 実装: handler.go

**大きな変更点**:
1. `Server` 構造体に `doneCh chan struct{}` フィールド追加（テスト用）
2. SSEヘッダ設定削除（`Content-Type: text/event-stream`, `Cache-Control`, `Connection`）
3. `assistantMsg` 作成後に即 `202 SendMessageResponse` を返却
4. `Execute` + DB保存処理全体を `go func()` でgoroutine化
5. goroutine内でステータス更新・バッチ保存・最終保存を実施
6. Execute callbackからSSE書込み全削除（約30箇所）、DB状態蓄積ロジックは維持
7. `batchInterval` デフォルト: 5秒 → 2秒

### 削除行数 (handler.go)
- SSEヘッダ設定: 4行
- SSE書込み (fmt.Fprintf + flusher.Flush): 約60行
- `fmt` import削除: 1行

---

## Cycle 3: Frontend sendMessage書換え

### 変更内容 (apps/frontend/src/api/client.ts)
- SSEイベント型export 13種を全削除
- `SSEEvent` union type削除
- `isAbortError()` 関数削除
- `sendMessage()` を単純POST + 202レスポンスパースに書換え
  - シグネチャ: `(conversationId, content) => Promise<{ message_id: string }>`

---

## Cycle 4: Frontend App.tsx簡素化

### 変更内容

**App.tsx**:
- `useSSEAbort` import削除
- `SSEHookEvent` import削除
- `StreamMeta` 型定義削除
- 削除したstate: `streamMeta`, `hookEvents`, `streamBlocks`
- 削除したref: `rafIdRef`, `streamMetaRef`, `hookEventsRef`, `streamBlocksRef`, `lastMessageIdRef`
- `scheduleRafUpdate()` 削除
- `handleSelectConversation` 簡素化（SSEクリーンアップ削除）
- `handleSend` 簡素化（SSEハンドラ削除、202後にポーリング開始）
- `isRunning` 計算式修正: `sending || isPolling || hasStreamingMessage`

**useSSEAbort.ts**: ファイル削除

**ChatView.tsx**:
- `SSEHookEvent` / `StreamMeta` import削除
- props削除: `isStreaming`, `streamMeta`, `hookEvents`, `streamBlocks`
- SSEストリーミング描画パス削除
- MessageInput disabled: `isStreaming || isPolling` → `isRunning === true`

**MessageBubble.tsx**:
- `SSEHookEvent` / `StreamMeta` import削除
- `streamMeta` / `hookEvents` props削除
- model/cost/hookEventsを`msgData`から取得（live streamMeta不要）

**App.test.tsx**: `sendMessage` mock型を `Promise<{ message_id: string }>` に更新

**ChatView.test.tsx**: SSEストリーミングテスト2件削除、`isStreaming` prop参照を削除

**useSSEAbort.test.ts**: ファイル削除

**client.test.ts**: 旧SSE API → 新POST APIのテストに全書換え（3件）

---

## Cycle 5: バッチ間隔調整

- `handler.go`: batchInterval デフォルト 5秒 → 2秒（Cycle 2で実施済み）
- `useConversationPoller.ts`: `intervalMs` デフォルト 2000 → 1000
- `App.tsx`: `useConversationPoller` に `intervalMs: 1000` を明示渡し

---

## 最終 mise run check 結果

```
Test Files  8 passed (8)
Tests  37 passed (37)
SKIP: 0
```

全テストPASS、SKIP=0、lint 0 issues。
