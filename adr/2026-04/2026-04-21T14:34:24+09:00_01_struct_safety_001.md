# subtask_struct_safety_001 実施報告

## 概要

`ConversationDetail.Status` 設定漏れ（Goゼロ値 `""` が返るバグ）の根本原因に対し、
構造体フィールド設定漏れを構造的に根絶する 3 層防御を実装した。

## 対策1: コンストラクタ関数（DB→API変換の集約）

### 変更ファイル

- **新規作成**: `apps/cc-tunnel/internal/api/mapping.go`
- **変更**: `apps/cc-tunnel/internal/api/handler.go`

### 実装内容

`mapping.go` に以下の 3 つのコンストラクタを定義:

```
newConversation(c *db.Conversation) Conversation
newMessage(m *db.Message) Message
newConversationDetail(conv *db.Conversation, msgs []*db.Message) ConversationDetail
```

- 全フィールドを明示的に設定（Status 含む）
- `handler.go` の `GetConversation()` でインライン構造体リテラルを廃止し
  `newConversationDetail()` 呼び出しに置換
- 後方互換のため `dbConvToAPI` / `dbMsgToAPI` は `mapping.go` にエイリアスとして保持

### TDD 先行テスト

`mapping_test.go` に追加したテスト:
- `TestNewMessage_allFields` — Status・全フィールド設定確認
- `TestNewMessage_emptyStatus_nilPointer` — status=""時のnil変換
- `TestNewConversationDetail_allFields` — Status含む全フィールド確認
- `TestNewConversationDetail_emptyMessages` — nil msgs → 空スライス

## 対策2: exhaustruct linter 導入

### 変更ファイル

- **変更**: `apps/cc-tunnel/.golangci.yml`

### 設定

```yaml
version: "2"

linters:
  enable:
    - exhaustruct
  settings:
    exhaustruct:
      include:
        - 'github\.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/api\.ConversationDetail$'
        - 'github\.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/api\.Message$'
        - 'github\.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/api\.Conversation$'
```

- API レスポンス型 3 つのみに絞り込み（過剰適用防止）
- golangci-lint v2 形式（`linters.settings` キー）を使用
- `golangci-lint run ./...` → 0 issues 確認

## 対策3: フィールド網羅テスト

### 変更ファイル

- **変更**: `apps/cc-tunnel/internal/api/handler_test.go`

### 追加テスト

- `TestGetConversation_hasAllFields` — ID・Status・CreatedAt・Model・Message.Status 全非ゼロ検証
- `TestListConversations_hasAllFields` — 一覧レスポンスの ID・Status・CreatedAt 非ゼロ検証

## docs 更新

- `docs/architecture.md` に「コンストラクタ関数パターン（構造体フィールド設定漏れ防止）」セクション追記

## 最終 mise run check 出力

```
[test:cc-tunnel] ok  github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/api  0.056s
[lint:cc-tunnel] 0 issues.
[test:frontend]  Test Files  9 passed (9)
[test:frontend]       Tests  50 passed (50)
Finished in 15.34s
```

- SKIP=0
- 全テスト PASS
- lint 0 issues

## 変更ファイル一覧

| ファイル | 変更種別 | 内容 |
|---------|---------|------|
| `internal/api/mapping.go` | 新規作成 | コンストラクタ関数 |
| `internal/api/handler.go` | 変更 | GetConversation → newConversationDetail, dbConvToAPI/dbMsgToAPI 削除 |
| `internal/api/mapping_test.go` | 変更 | newMessage/newConversationDetail TDDテスト追加 |
| `internal/api/handler_test.go` | 変更 | フィールド網羅テスト追加、db/time import追加 |
| `.golangci.yml` | 変更 | exhaustruct linter 追加 |
| `docs/architecture.md` | 変更 | コンストラクタパターンセクション追記 |
