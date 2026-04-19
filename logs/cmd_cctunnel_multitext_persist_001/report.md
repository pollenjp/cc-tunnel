# cmd_cctunnel_multitext_persist_001 調査・修正レポート

## 1. 経緯: なぜこの問題が起きたか

前 cmd で `content_blocks` の概念が**フロントエンドの React state のみ**に実装されたため。

`App.tsx` の `finally` ブロック（送信完了後）で `streamBlocks` から `contentBlocks` を構築し、`setMessages` でローカル state に保存しているが、この state は **DB に永続化されない**。バックエンドの Go ハンドラ (`handler.go`) は assistant メッセージを保存する際に `content_blocks` を `metadata` に含めておらず、保存されるのは以下のみだった：

- `content`: 全テキストブロックを結合した文字列（`assistantContent`）
- `metadata.tool_calls`: ツール呼び出しの一覧（フラット）

結果：リロード時に `getConversation` でDBからメッセージを取得すると `metadata.content_blocks` が存在せず、フォールバック（旧フォーマット）が適用されてレイアウトが崩れた。

## 2. 根本原因: 具体的なバグ箇所

### ファイル: `apps/cc-tunnel/internal/api/handler.go`

**バグ1: `assistant` イベント処理で `tool_use` ブロックを無視していた**

```go
// 修正前: tool_use ブロックの処理がない
case "assistant":
    if event.Message != nil {
        for _, block := range event.Message.Content {
            if block.Type == "thinking" && block.Thinking != "" { ... }
            if block.Type == "text" && block.Text != "" { ... }
            if block.Type == "tool_result" { ... }
            // tool_use ブロックの処理なし!
        }
    }
```

**バグ2: `content_blocks` を metadata に保存していなかった**

```go
// 修正前: content_blocks の保存なし
metadata["tool_calls"] = toolCallsData
// content_blocks の保存がない!
h.repo.CreateMessage(...)
```

## 3. 修正内容

### ファイル: `apps/cc-tunnel/internal/api/handler.go`

**変更1: `contentBlocksList` 変数の追加（line 255）**

```go
// 修正前
var (
    assistantContent string
    thinkingContent  string
    ...
    toolCallsData    []ToolCallData
)

// 修正後
var (
    assistantContent  string
    thinkingContent   string
    ...
    toolCallsData     []ToolCallData
    contentBlocksList []map[string]interface{}  // NEW
)
```

**変更2: `case "assistant":` で全ブロックを `contentBlocksList` に記録（lines 277-311）**

```go
// thinking ブロック
contentBlocksList = append(contentBlocksList, map[string]interface{}{
    "type":    "thinking",
    "content": block.Thinking,
})

// text ブロック
contentBlocksList = append(contentBlocksList, map[string]interface{}{
    "type":    "text",
    "content": block.Text,
})

// tool_use ブロック（新規追加）
if block.Type == "tool_use" && block.ID != "" {
    contentBlocksList = append(contentBlocksList, map[string]interface{}{
        "type":        "tool_use",
        "tool_use_id": block.ID,
    })
}
```

**変更3: metadata に `content_blocks` を保存（lines 591-593）**

```go
if len(contentBlocksList) > 0 {
    metadata["content_blocks"] = contentBlocksList
}
```

## 4. 修正の仕組み

`assistant` イベントはマルチターン（tool_use → tool_result → 次のassistant）のたびに発火し、そのターンの完全なメッセージを含む。このイベントから順序付きブロックリストを構築することで、**ターンをまたいだテキスト・thinking・tool_use の正しい順序**を保持できる。

例（text0 → tool_use → text1 の場合）:
```json
"content_blocks": [
  {"type": "text", "content": "text0"},
  {"type": "tool_use", "tool_use_id": "toolu_01..."},
  {"type": "text", "content": "text1"}
]
```

リロード時、フロントエンドの `ChatView.tsx` は `content_blocks` が存在すれば新フォーマットで復元する（line 69-89）。`tool_use_id` を `metadata.tool_calls` と突き合わせてツールカードを構築し、正しい順序で表示される。

## 5. 再発防止ポイント

1. **フロントエンド state は揮発性**: `setMessages` でローカル state を更新しても DB には反映されない。永続化が必要なメタデータはバックエンド側で `metadata` に保存すること。

2. **`assistant` イベントのブロックタイプを全て処理する**: `text`・`thinking` だけでなく `tool_use` ブロックも `content_blocks` に記録が必要。省くと順序情報が欠落する。

3. **マルチターン応答の `content_blocks` は複数の `assistant` イベントにまたがる**: 各ターンの `assistant` イベントで `contentBlocksList` に追記する設計が必要（1回の `assistant` イベントだけ見ても不完全）。

4. **旧フォーマットとの後方互換**: `content_blocks` がない古いメッセージは旧フォーマット（テキスト結合 → ツールカード）で表示されるため、既存データの移行は不要。
