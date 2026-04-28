# テスト追加報告 — subtask_unit_tests_001

## テスト対象として選定した箇所と選定理由

### 1. `cc-tunnel/internal/logging/handler_test.go` (新規)
**対象**: `ErrorStackHandler.Handle`

**選定理由**: エラー属性が存在する場合にのみスタックトレースを追加するという条件分岐ロジック。誤動作すれば「エラーなのにスタックなし」または「エラーでもないのにスタックが付く」バグになる。副作用のないロジックで単体テスト適性が高い。

**追加テスト**:
- `withErrorAttr_addsStack` — エラー属性あり → "stack" 属性が Next に渡される
- `withoutErrorAttr_noStack` — 非エラー属性のみ → "stack" 属性なし
- `noAttrs_noStack` — 属性なし → "stack" なし（ERROR レベルでも誤検知しないこと）
- `stackIsNonEmpty` — stack が空でないこと（extractStack の動作確認）
- `WithAttrs_returnsErrorStackHandler` — ラッパー型が保持されること
- `WithGroup_returnsErrorStackHandler` — ラッパー型が保持されること
- `nonErrorAny_noStack` — slog.Any で非エラー型 → "stack" なし（型アサーション正確性）

### 2. `cc-tunnel/internal/api/mapping_test.go` (新規)
**対象**: SSE イベント型の JSON シリアライズ、`dbMsgToAPI`、`dbConvToAPI`

**選定理由**:
- SSE の `type` フィールドは Frontend が type narrowing で使うため、値の変更はクライアント破壊バグになる
- `dbMsgToAPI` / `dbConvToAPI` は DB → API の変換ロジックで UUID パース・nil 判定がある

**追加テスト (SSE)**:
- `SSETextEvent_typeField` — type="text"
- `SSEThinkingEvent_typeField` — type="thinking"
- `SSEDoneEvent_typeField` — type="done", session_id フィールド
- `SSEErrorEvent_typeField` — type="error", message フィールド
- `SSEInitEvent_typeField` — type="init"
- `SSEToolUseStartEvent_typeField` — type="tool_use_start", tool_name フィールド
- `SSEToolResultEvent_typeField` — type="tool_result"
- `SSECostEvent_typeField` — type="cost"

**追加テスト (mapping)**:
- `DbMsgToAPI_basicFields` — UUID・Role・MessageData が正しくマップされる
- `DbMsgToAPI_emptyMessageData_nilPointer` — 空 map → MessageData フィールドが nil ポインタ
- `DbMsgToAPI_invalidUUID_zeroValue` — 不正 UUID → ゼロ値 UUID（パニックしない）
- `DbMsgToAPI_assistantRole` — Role 文字列 → MessageRole 型変換
- `DbConvToAPI_basicFields` — UUID・Title・Model が正しくマップされる
- `DbConvToAPI_withSystemPrompt` — SystemPrompt ポインタが正しく伝播する
- `DbConvToAPI_invalidUUID_zeroValue` — 不正 UUID → ゼロ値 UUID

### 3. `cc-remote-agent/internal/claude/executor_test.go` (新規)
**対象**: `buildArgs`、`buildFallbackPrompt`、`containsAny`、`containsResumeError`

**選定理由**: 純粋関数でテスト適性が高い。特に `buildArgs` は claude CLI の引数構築ロジックで、`--resume` / `--model` 等の組み合わせバグが実行時まで見えない。`containsResumeError` は resume 失敗検知のフォールバックトリガーで誤検知・未検知が深刻。

**追加テスト**:
- `buildArgs_baseFlags` — 必須フラグ (-p, --output-format, --verbose, --dangerously-skip-permissions)
- `buildArgs_withSessionID_includesResume` — SessionID → --resume フラグ付与
- `buildArgs_isFallback_omitsResume` — isFallback=true → --resume 省略
- `buildArgs_withoutSessionID_noResume` — SessionID="" → --resume なし
- `buildArgs_model_included` — Model → --model フラグ
- `buildArgs_emptyModel_omitted` — Model="" → --model 省略
- `buildArgs_systemPrompt_included` — SystemPrompt → --system-prompt フラグ
- `buildArgs_allowedTools_eachToolHasFlag` — AllowedTools 各ツール → --allowedTools フラグ
- `buildArgs_permissionMode_included` — PermissionMode → --permission-mode フラグ
- `buildArgs_includePartialMessages_flag` — true → --include-partial-messages
- `buildArgs_includePartialMessages_false_omitted` — false → フラグなし
- `buildArgs_includeHookEvents_flag` — true → --include-hook-events
- `buildArgs_promptIsLastAfterDoubleDash` — プロンプトが -- の直後の最終引数
- `buildFallbackPrompt_includesUserMessages` — [User]: ... 形式
- `buildFallbackPrompt_includesAssistantMessages` — [Assistant]: ... 形式
- `buildFallbackPrompt_includesSystemMessages` — [System]: ... 形式
- `buildFallbackPrompt_includesCurrentPrompt` — 現在のプロンプトが末尾に含まれる
- `buildFallbackPrompt_conversationOrderPreserved` — 順序保持
- `containsAny` — テーブル駆動: 完全一致・部分一致・不一致・空文字・空リスト
- `containsResumeError` — テーブル駆動: 各エラーパターン・正常行・空行

## テスト追加しなかった箇所とその理由

| 箇所 | 理由 |
|------|------|
| `cc-tunnel/internal/api/handler.go` HTTP ハンドラ群 | DB・remoteclient 依存が多くモックが無意味な単体テストになる。E2E は家老担当 |
| `cc-tunnel/internal/db/repository.go` | PostgreSQL 実装のため実 DB なしでは意味がない |
| `cc-tunnel/internal/remoteclient/client.go` | 外部 HTTP 依存があり単体テスト不適 |
| `cc-remote-agent/internal/auth/manager.go` | PTY・外部プロセス依存 |
| `cc-remote-agent/internal/claude/StreamToWriter`・`runStream` | `exec.Command("claude", ...)` を呼ぶため実行環境依存 |
| `cc-remote-agent/internal/logging/handler.go` | cc-tunnel と同一実装のため省略（同パッケージテスト追加は cc-tunnel 側で済） |
| TS 側 (frontend) | テストフレームワーク (vitest) 導入済みだが既存 1 件のみで型テストは不要 |

## テスト実行結果

```
cc-tunnel/internal/logging:
PASS  7 tests

cc-tunnel/internal/api:
PASS  17 tests (既存 2 + 新規 15)

cc-remote-agent/internal/claude:
PASS  22 tests (新規)
```

### mise run check (全パッケージ)
```
[test:cc-tunnel]      ok internal/api  ok internal/logging
[test:cc-remote-agent] ok internal/claude
[lint:cc-tunnel]      0 issues.
[lint:cc-remote-agent] 0 issues.
[test:frontend]       1 passed (1)
[lint:frontend]       0 issues.
Finished in ~10s
```

全テスト PASS・lint エラー 0。
