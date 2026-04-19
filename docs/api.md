# cc-tunnel API Reference

## Overview

cc-tunnel は2層構造のAPIを持つ。

```
Browser
  ↓ (External API: HTTP/SSE)
cc-tunnel (nginx → Go server)
  ↓ (Internal API: HTTP/ndjson)
cc-remote-agent
  ↓ (subprocess)
claude CLI
```

## スキーマ参照

REST エンドポイントの型定義は `apps/openapi/openapi.yaml` が正規ソース。
フロントエンド TypeScript 型は `apps/frontend/src/api/schema.d.ts`（`openapi-typescript` で自動生成）が単一ソース。

---

## 1. External API (Browser → cc-tunnel)

ベースURL: `/api` (nginx プロキシ経由)

### 認証系

#### GET /auth/status

Claude CLI の認証状態を返す。

**Response 200**

```json
{
  "loggedIn": true,
  "authMethod": "claude.ai",
  "loginPending": false,
  "apiProvider": "anthropic",
  "email": "user@example.com",
  "orgName": "My Org",
  "subscriptionType": "pro",
  "apiKeySource": null,
  "loginUrl": null
}
```

| フィールド         | 型      | 説明                             |
| ------------------ | ------- | -------------------------------- |
| `loggedIn`         | boolean | ログイン済みか                   |
| `authMethod`       | string  | `none` / `api_key` / `claude.ai` |
| `loginPending`     | boolean | ログイン処理が進行中か           |
| `apiProvider`      | string? | APIプロバイダ名                  |
| `email`            | string? | ログイン中のメールアドレス       |
| `orgName`          | string? | 組織名                           |
| `subscriptionType` | string? | サブスクリプション種別           |
| `apiKeySource`     | string? | APIキーのソース                  |
| `loginUrl`         | string? | OAuth URL（ログイン開始時のみ）  |

---

#### POST /auth/login

ログインフローを開始する。

**Request Body** (省略可)

```json
{
  "method": "claudeai"
}
```

| フィールド | 型      | 説明                                |
| ---------- | ------- | ----------------------------------- |
| `method`   | string? | `claudeai` (デフォルト) / `console` |

**Response 200**

```json
{
  "message": "Login initiated",
  "loginUrl": "https://claude.ai/oauth/...",
  "loggedIn": false
}
```

---

#### POST /auth/logout

ログアウトする。

**Response 200**: `AuthStatus` オブジェクト（`GET /auth/status` と同じ形式）

---

#### POST /auth/cancel

進行中のログインPTYプロセスをキャンセルする。

**Response 200**

```json
{
  "message": "Login cancelled"
}
```

---

#### POST /auth/input

ログインプロセスのstdinに入力を送信する（対話型認証用）。

**Request Body**

```json
{
  "input": "y"
}
```

| フィールド | 型     | 説明                                       |
| ---------- | ------ | ------------------------------------------ |
| `input`    | string | 送信する文字列（空文字列 = Enterキーのみ） |

**Response 200**

```json
{
  "message": "Input submitted"
}
```

**Response 409**: ログインプロセスが存在しない場合

---

#### GET /auth/output?since=N

ログインプロセスのPTY出力を取得する（ポーリング用）。

**Query Parameters**

| パラメータ | 型      | デフォルト | 説明                     |
| ---------- | ------- | ---------- | ------------------------ |
| `since`    | integer | 0          | カーソル位置（0 = 全件） |

**Response 200**

```json
{
  "data": "base64エンコードされたPTY出力バイト列",
  "cursor": 42
}
```

| フィールド | 型      | 説明                                                |
| ---------- | ------- | --------------------------------------------------- |
| `data`     | string  | Base64エンコードされたPTY出力（`since` 以降の差分） |
| `cursor`   | integer | 次回リクエスト用カーソル位置                        |

---

### 会話管理

#### POST /conversations

新しい会話を作成する。

**Request Body** (省略可)

```json
{
  "title": "My conversation",
  "model": "claude-sonnet-4-6",
  "system_prompt": "You are a helpful assistant."
}
```

| フィールド      | 型      | デフォルト            | 説明               |
| --------------- | ------- | --------------------- | ------------------ |
| `title`         | string  | `""`                  | 会話タイトル       |
| `model`         | string  | `"claude-sonnet-4-6"` | 使用モデル         |
| `system_prompt` | string? | null                  | システムプロンプト |

**Response 201**

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "title": "My conversation",
  "model": "claude-sonnet-4-6",
  "status": "idle",
  "system_prompt": "You are a helpful assistant.",
  "created_at": "2026-04-18T01:00:00Z",
  "updated_at": "2026-04-18T01:00:00Z"
}
```

**Response 500**: Internal server error

---

#### GET /conversations

全会話の一覧を返す。

**Response 200**: `Conversation[]` 配列

---

#### GET /conversations/{conversationId}

会話とメッセージ履歴を返す。`status` フィールドで CLI 実行状態を確認できる。
フロントエンドはセッション選択時に `status === 'running'` を検知して、DBポーリングを開始する。

**Path Parameters**: `conversationId` (UUID)

**Response 200**

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "title": "My conversation",
  "model": "claude-sonnet-4-6",
  "status": "idle",
  "system_prompt": null,
  "created_at": "2026-04-18T01:00:00Z",
  "updated_at": "2026-04-18T01:05:00Z",
  "messages": [
    {
      "id": "msg-uuid",
      "conversation_id": "550e8400-e29b-41d4-a716-446655440000",
      "role": "user",
      "message_data": { "content": "Hello" },
      "created_at": "2026-04-18T01:01:00Z"
    },
    {
      "id": "msg-uuid-2",
      "conversation_id": "550e8400-e29b-41d4-a716-446655440000",
      "role": "assistant",
      "message_data": { "session_id": "abc123", "content": "Hi! How can I help?" },
      "created_at": "2026-04-18T01:01:05Z"
    }
  ]
}
```

**Response 404**: 会話が存在しない

---

#### DELETE /conversations/{conversationId}

会話を削除する。

**Path Parameters**: `conversationId` (UUID)

**Response 200**

```json
{ "status": "ok" }
```

**Response 404**: 会話が存在しない

---

### メッセージ送信（SSEストリーミング）

#### POST /conversations/{conversationId}/messages

ユーザーメッセージを送信し、アシスタントの応答をServer-Sent Events (SSE)でストリーミングする。

**Path Parameters**: `conversationId` (UUID)

**Request Body**

```json
{
  "content": "Tell me about Go generics."
}
```

**Response 200** — `Content-Type: text/event-stream`

各イベントは `data: <json>\n\n` 形式で送信される。

#### SSEイベント一覧

**`init`** — セッション開始情報

```
data: {"type":"init","model":"claude-sonnet-4-6","session_id":"abc123"}
```

**`text`** — アシスタントのテキスト（完成ブロック単位）

```
data: {"type":"text","content":"Go generics allow..."}
```

**`text_delta`** — テキストの差分（リアルタイム表示用）

```
data: {"type":"text_delta","content":"allow"}
```

**`thinking`** — 思考ブロック（完成ブロック単位）

```
data: {"type":"thinking","content":"The user is asking about..."}
```

**`thinking_delta`** — 思考の差分（リアルタイム表示用）

```
data: {"type":"thinking_delta","content":"The user"}
```

**`tool_use_start`** — ツール使用開始

```
data: {"type":"tool_use_start","index":0,"tool_use_id":"toolu_01","tool_name":"bash"}
```

| フィールド | 型 | 説明 |
|------------|-----|------|
| `index` | integer | コンテンツブロックのインデックス |
| `tool_use_id` | string | ツール使用ID |
| `tool_name` | string | ツール名 |

**`tool_input_delta`** — ツール入力の差分

```
data: {"type":"tool_input_delta","index":0,"partial_json":"{\"command\":\"ls"}
```

| フィールド | 型 | 説明 |
|------------|-----|------|
| `index` | integer | コンテンツブロックのインデックス |
| `partial_json` | string | ツール入力JSONの差分断片 |

**`tool_result`** — ツール実行結果（アシスタント側1000文字、ユーザー側2000文字で切り捨て）

```
data: {"type":"tool_result","tool_use_id":"toolu_01","content":"file1.txt\nfile2.txt"}
```

| フィールド | 型 | 説明 |
|------------|-----|------|
| `tool_use_id` | string | 対応する `tool_use_start` のツール使用ID |
| `content` | string | ツール実行結果テキスト |

**`hook_event`** — Claude Code フックイベント

```
data: {"type":"hook_event","subtype":"hook_started","hook_id":"h1","hook_name":"PreToolUse","hook_event":"hook_started","session_id":"abc123"}
```

| フィールド    | 型      | 説明                                                         |
| ------------- | ------- | ------------------------------------------------------------ |
| `subtype`     | string  | イベント種別（下表参照）                                     |
| `hook_id`     | string? | フックの識別子                                               |
| `hook_name`   | string? | フック名（例: `PreToolUse`, `PostToolUse`）                  |
| `hook_event`  | string? | フックイベント種別（`hook_started` / `hook_response` / `notification` / `status`） |
| `session_id`  | string? | セッションID                                                 |

| `subtype`       | 説明             |
| --------------- | ---------------- |
| `hook_started`  | フック開始       |
| `hook_response` | フックからの応答 |
| `notification`  | 通知             |
| `status`        | ステータス更新   |

**`rate_limit`** — レート制限情報

```
data: {"type":"rate_limit","status":"limited","resets_at":1713398400,"rate_limit_type":"requests"}
```

**`cost`** — コスト情報（`done` の直前に送信）

```
data: {"type":"cost","total_cost_usd":0.001,"duration_ms":3200}
```

**`done`** — ストリーム終了

```
data: {"type":"done","session_id":"abc123","cost_usd":0.001}
```

**`error`** — エラー

```
data: {"type":"error","message":"execution failed"}
```

**Response 400**: リクエスト不正  
**Response 404**: 会話が存在しない

#### 会話ステータス (`status` フィールド)

`Conversation` オブジェクトには `status` フィールド（`idle` / `running` / `completed`）が含まれる。

| 値 | 意味 |
|----|------|
| `idle` | 初期状態。メッセージ送信待ち |
| `running` | CLI 実行中（`SendMessage` 処理中） |
| `completed` | CLI 実行完了 |

**SSE切断後の復帰フロー（DBポーリング方式）**:

1. ユーザーが `POST /conversations/{id}/messages` でメッセージ送信 → SSEストリーミング開始
2. ストリーミング中に別会話に切り替え（SSE切断） → バックエンドは `execCtx` で処理継続
3. 元の会話に戻る → フロントエンドが `GET /conversations/{id}` で `status` を確認
4. `status === 'running'` → `useConversationPoller` フックが 2 秒間隔で `GET /conversations/{id}` をポーリング
5. 新しいメッセージが届いたら差分表示（「処理中...」インジケータ付き）
6. `status === 'completed'` になったらポーリング停止

#### タイトルの自動更新

アシスタント応答完了後、`conversations.title` カラムが自動的に更新される。

- 生成元: アシスタント応答テキストの先頭 60 文字（Markdown 除去・改行→スペース変換後）
- 60 文字超過時は `...` を付加
- 応答テキストが空の場合は更新しない（既存タイトルを維持）
- サイドバーのセッション一覧に即時反映される（`GET /conversations` でタイトルを含む `Conversation[]` を返す）

---

#### アシスタントメッセージの message_data

アシスタント返信完了後、DB に保存されるメッセージの `message_data` フィールドには以下が格納される。

| キー | 型 | 説明 |
|------|----|------|
| `content` | string | アシスタント応答のテキスト全文 |
| `session_id` | string | Claude CLI セッションID（次回 `--resume` に使用） |
| `model` | string | 使用モデル名 |
| `cost_usd` | number | コスト（USD） |
| `duration_ms` | integer | 実行時間（ミリ秒） |
| `tool_calls` | ToolCallData[] | ツール呼び出しデータ一覧 |
| `thinking` | string[] | 思考ブロック一覧 |
| `hook_events` | object[] | フックイベント一覧 |
| `content_blocks` | object[] | コンテンツブロック一覧 |

##### ToolCallData

`metadata.tool_calls` の各要素。openapi.yaml の `ToolCallData` スキーマ参照。

| フィールド | 型 | 必須 | 説明 |
|------------|-----|:----:|------|
| `tool_use_id` | string | ✓ | ツール使用ID（SSE `tool_use_start` の `tool_use_id` と対応） |
| `tool_name` | string | ✓ | ツール名 |
| `input_json` | string | ✓ | ツール入力JSON文字列（`input_json_delta` を結合したもの） |
| `result` | string | - | ツール実行結果（`tool_result` イベント受信後に格納） |
| `is_running` | boolean | - | 実行中フラグ |

> **廃止フォーマット**: 旧バージョンでは `tool_calls` の各要素に `name`/`input` フィールドを使用していた。現行フォーマットでは `tool_name`/`input_json` に変更されている（後方互換性なし）。

---

## 2. Internal API (cc-tunnel → cc-remote-agent)

cc-tunnel が cc-remote-agent に直接 HTTP で通信する内部API。
ベースURL: `http://cc-remote-agent:8080`（Docker内部ネットワーク）

### 認証系

#### GET /auth/status

**Response 200**

```json
{
  "loggedIn": true,
  "authMethod": "claude.ai",
  "loginPending": false,
  "apiProvider": "anthropic",
  "email": "user@example.com",
  "orgName": "My Org",
  "subscriptionType": "pro",
  "apiKeySource": null,
  "loginUrl": null
}
```

---

#### POST /auth/login

**Request Body**

```json
{ "method": "claudeai" }
```

`method` は `"claudeai"` (デフォルト) または `"console"`。

**Response 200**

```json
{
  "message": "Login initiated",
  "loginUrl": "https://claude.ai/oauth/...",
  "loggedIn": false
}
```

---

#### POST /auth/logout

**Response 200**: AuthStatus オブジェクト

---

#### POST /auth/input

**Request Body**

```json
{ "input": "y" }
```

**Response 200**

```json
{ "message": "Input submitted" }
```

**Response 409**: ログインプロセスが存在しない

---

#### GET /auth/output?since=N

**Response 200**

```json
{
  "data": "base64エンコードPTY出力",
  "cursor": 42
}
```

---

#### POST /auth/cancel

**Response 200**

```json
{ "message": "Login cancelled" }
```

---

### 実行系

#### POST /execute

claude CLI を起動してndjsonをストリーミングする。
認証されていない場合は `401 Unauthorized` を返す。

**Request Body**

```json
{
  "prompt": "Tell me about Go generics.",
  "session_id": "abc123",
  "model": "claude-sonnet-4-6",
  "system_prompt": "You are a helpful assistant.",
  "conversation_history": [
    { "role": "user", "content": "Hello" },
    { "role": "assistant", "content": "Hi!" }
  ],
  "allowed_tools": ["bash", "read"],
  "permission_mode": "default",
  "max_budget_usd": 1.0,
  "include_partial_messages": true,
  "include_hook_events": true
}
```

| フィールド                 | 型        | 説明                                                   |
| -------------------------- | --------- | ------------------------------------------------------ |
| `prompt`                   | string    | **必須**。ユーザープロンプト                           |
| `session_id`               | string?   | `--resume` 用セッションID（省略時は新規セッション）    |
| `model`                    | string?   | 使用モデル                                             |
| `system_prompt`            | string?   | システムプロンプト                                     |
| `conversation_history`     | array?    | 会話履歴（`--resume` 失敗時のフォールバック用）        |
| `allowed_tools`            | string[]? | 許可ツール一覧                                         |
| `permission_mode`          | string?   | パーミッションモード                                   |
| `max_budget_usd`           | number?   | 最大コスト上限（USD）                                  |
| `include_partial_messages` | boolean?  | デルタイベントを有効化（`--include-partial-messages`） |
| `include_hook_events`      | boolean?  | フックイベントを有効化（`--include-hook-events`）      |

**Response 200** — `Content-Type: application/x-ndjson`

claude CLI の出力をndjson（1行1JSON）でストリーミングする。
各行は claude CLI の `--output-format=stream-json --verbose` 出力と同じ形式。

主なイベント行:

```jsonc
// type=system, subtype=init: セッション開始
{"type":"system","subtype":"init","session_id":"abc123","model":"claude-sonnet-4-6"}

// type=assistant: アシスタントメッセージ（完成ブロック）
{"type":"assistant","message":{"content":[{"type":"text","text":"Hello!"}]}}

// type=stream_event: ストリーミングデルタ（include_partial_messages=true 時）
{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}}

// type=system, subtype=hook_started: フックイベント（include_hook_events=true 時）
{"type":"system","subtype":"hook_started","hook_id":"h1","hook_name":"PreToolUse","session_id":"abc123"}

// type=rate_limit_event: レート制限
{"type":"rate_limit_event","rate_limit_info":{"status":"limited","resetsAt":1713398400,"rateLimitType":"requests"}}

// type=result: 実行完了
{"type":"result","session_id":"abc123","total_cost_usd":0.001,"duration_ms":3200}
```

**Response 400**: `prompt` が空  
**Response 401**: 未認証

---

#### GET /health

サービスのヘルスチェック。claude CLI の存在確認も行う。

**Response 200**

```json
{
  "status": "ok",
  "claude_version": "1.2.3"
}
```
