# cc-tunnel シーケンス図

## フロー 1: 会話セッション作成

```mermaid
sequenceDiagram
    participant F as Frontend (Browser)
    participant T as cc-tunnel (API Server)
    participant D as PostgreSQL

    F->>T: POST /conversations<br/>{title, model, system_prompt}
    T->>D: INSERT conversations
    D-->>T: conversation row
    T-->>F: 201 Created (Conversation JSON)
```

## フロー 2: メッセージ送信 + ストリーミング受信（メインフロー）

```mermaid
sequenceDiagram
    participant F as Frontend (Browser)
    participant T as cc-tunnel (API Server)
    participant A as cc-remote-agent
    participant D as PostgreSQL
    participant C as claude CLI

    F->>T: POST /conversations/{id}/messages<br/>{content}
    T->>D: SELECT conversation (存在確認)
    D-->>T: conversation row
    T->>D: SELECT messages (履歴取得)
    D-->>T: messages[]
    Note over T: 最新 assistant メッセージの<br/>metadata["session_id"] を取得
    T->>D: INSERT user message
    D-->>T: ok

    T->>A: POST /execute<br/>{prompt, session_id, model, conversation_history}
    A->>C: claude -p --output-format=stream-json<br/>--verbose --resume <session_id> -- <prompt>
    C-->>A: stream-json lines (ndjson)
    A-->>T: ndjson stream (chunked)

    loop 各 text チャンク
        T-->>F: SSE data: {"type":"text","content":"..."}
    end

    T-->>F: SSE data: {"type":"done","session_id":"...","cost_usd":...}

    T->>D: INSERT assistant message<br/>(metadata: {session_id: new_session_id})
    T->>D: UPDATE conversations.updated_at
```

## フロー 3: 会話一覧取得

```mermaid
sequenceDiagram
    participant F as Frontend (Browser)
    participant T as cc-tunnel (API Server)
    participant D as PostgreSQL

    F->>T: GET /conversations
    T->>D: SELECT conversations ORDER BY updated_at DESC
    D-->>T: conversations[]
    T-->>F: 200 OK (Conversation[])
```

## フロー 4: --resume フォールバック分岐

```mermaid
sequenceDiagram
    participant A as cc-remote-agent
    participant C as claude CLI

    A->>C: claude -p --output-format=stream-json<br/>--verbose --resume <session_id> -- <prompt>

    alt resume 成功
        C-->>A: stream-json lines (ndjson)
        Note over A: 正常にストリーム転送
    else resume 失敗 ("session not found" 検知)
        C-->>A: エラー行 (session not found)
        Note over A: SessionID をクリア<br/>会話履歴を prompt に埋め込み<br/>(prompt stuffing)
        A->>C: claude -p --output-format=stream-json<br/>--verbose -- <stuffed_prompt>
        C-->>A: stream-json lines (ndjson)
        Note over A: フォールバックストリームを転送
    end
```
