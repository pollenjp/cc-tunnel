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

## フロー 2: メッセージ送信（非同期処理 + DB ポーリング）

```mermaid
sequenceDiagram
    participant F as Frontend (Browser)
    participant T as cc-tunnel (API Server)
    participant P as ExecutionProvider<br/>(LocalDockerProvider)
    participant SM as SessionManager
    participant A as cc-remote-agent (container)
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
    T->>D: INSERT assistant message (status='streaming')
    D-->>T: assistant message row
    T->>D: INSERT agent_dispatches (status='pending')
    Note over T: shadow write (ADR 2026-05-16 段階 1-4)。<br/>失敗は warn log のみ。reader はまだ wire されていない。
    T-->>F: 202 Accepted {message_id}

    Note over T: goroutine で非同期実行

    T->>D: UPDATE conversations SET status='running'
    T->>P: Execute(ctx, req, onEvent)
    P->>SM: GetOrCreate(conversationID)
    SM-->>P: remoteclient.Client (container endpoint)
    P->>A: POST /execute<br/>{prompt, session_id, conversation_id, model, conversation_history, ...}
    A->>C: claude -p --output-format=stream-json<br/>--verbose --resume <session_id> -- <prompt>
    C-->>A: stream-json lines (ndjson)
    A-->>P: ndjson stream

    loop content_blocks バッチ保存（2秒間隔）
        T->>D: UPDATE messages SET message_data (content_blocks, tool_calls)
    end

    A-->>P: ndjson stream 完了
    P-->>T: newSessionID
    T->>D: UPDATE messages (content_blocks, session_id, model, cost_usd, duration_ms, ...)
    T->>D: UPDATE messages SET status='completed'
    T->>D: UPDATE conversations SET title=<生成タイトル>
    T->>D: UPDATE conversations SET updated_at=NOW()
    T->>D: UPDATE conversations SET status='completed'
    T->>D: UPDATE agent_dispatches SET status='consumed' (shadow write の最終遷移)
    Note over T: 実行完了後、POST /auth/finalize-credentials 経由で<br/>cc-remote-agent コンテナから credentials を取得し<br/>AES-256-GCM 暗号化後に credentials テーブルに UPSERT

    loop DB ポーリング（1秒間隔）
        F->>T: GET /conversations/{id}
        T->>D: SELECT conversation + messages
        D-->>T: ConversationDetail (status='running'/'completed')
        T-->>F: ConversationDetail
    end
    Note over F: status='completed' でポーリング停止
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

## フロー 5: Credential Relogin（credentials 再取得フロー）

```mermaid
sequenceDiagram
    participant F as Frontend (Browser)
    participant T as cc-tunnel (API Server)
    participant P as ExecutionProvider
    participant A as cc-remote-agent (container)
    participant D as PostgreSQL

    Note over F: CredentialGuard が GET /credentials/status で<br/>registered=false または isValid=false を検知
    F->>T: GET /credentials/status<br/>(Authorization: Bearer <token>)
    T->>D: SELECT credentials WHERE username=<user>
    D-->>T: row (or not found)
    T-->>F: { registered: bool, isValid: bool }

    Note over F: /login/credentials?reason=missing|expired&conversationId=<id> にリダイレクト

    F->>T: POST /credentials/relogin/start<br/>{ conversationId }
    T->>P: PrepareForRelogin(conversationId)
    P-->>T: ok (コンテナ起動済み、credentials なし)
    T-->>F: { ready: true }

    F->>T: POST /auth/login<br/>{ conversationId }
    T->>A: PTY で claude /auth 起動
    T-->>F: 202 Accepted

    Note over F: AuthTerminal (xterm.js) が PTY 出力をレンダリング<br/>ユーザーが Claude OAuth またはキー入力で認証

    alt 自動検知（PTY 出力に "Login successful" を含む）
        Note over F: onTextOutput コールバックが自動で finalize 呼び出し
    else 手動ボタン（ユーザーが「完了」ボタンをクリック）
        Note over F: ユーザーが手動で finalize トリガー
    end

    F->>T: POST /credentials/relogin/finalize<br/>{ conversationId }
    T->>A: GET /finalize-credentials (内部 HTTP)
    A-->>T: credentials JSON
    T->>D: UPSERT credentials (AES-256-GCM 暗号化)
    D-->>T: ok
    T-->>F: { registered: true, isValid: true }

    Note over F: /chat/<conversationId> にリダイレクト
```

## フロー 6: hook 駆動 agent 通信 (段階 5-6 で実装予定, ADR 2026-05-16)

PTY 常駐 `claude` プロセスへ Claude Code hook 経由で I/O する将来構成。
本 PR (段階 1-4) では `agent_dispatches` への shadow write のみ実装されており、
**この sequence は段階 5-6 が wire された後に有効**。

```mermaid
sequenceDiagram
    participant F as Frontend (Browser)
    participant T as cc-tunnel (API Server)
    participant A as cc-remote-agent<br/>(container, /agent/*)
    participant H as cc-hook-bridge<br/>(in-container hook)
    participant CL as claude<br/>(long-lived, PTY)
    participant D as PostgreSQL

    Note over A,CL: 起動時に claude を PTY で常駐させる。<br/>SessionStart hook がここで一度発火。

    F->>T: POST /conversations/{id}/messages
    T->>D: INSERT messages (status='streaming')
    T->>D: INSERT agent_dispatches (status='pending')
    T->>A: POST /agent/kick (idempotent)
    T-->>F: 202 Accepted
    A->>CL: PTY stdin: <prompt>
    CL->>H: UserPromptSubmit hook 発火 (stdin: hook JSON)
    H->>D: INSERT agent_outputs (event_type='user_prompt_submit')

    loop ツール呼び出しの度に
        CL->>H: PreToolUse hook
        H->>D: INSERT agent_outputs (event_type='pre_tool_use')
        CL->>H: PostToolUse hook
        H->>D: INSERT agent_outputs (event_type='post_tool_use')
    end

    CL->>H: Stop hook (ターン完了)
    H->>D: INSERT agent_outputs (event_type='stop', status='final')
    H->>D: UPDATE agent_dispatches SET status='consumed'
    H->>D: SELECT pending dispatch FOR UPDATE SKIP LOCKED
    alt 新規 pending あり
        D-->>H: next dispatch row
        H->>D: UPDATE agent_dispatches SET status='delivered'
        H-->>CL: stdout: {"decision":"block","reason":<次の prompt>}
        Note over CL: claude が同一セッションで<br/>次ターンを実行
    else 55秒待っても無し
        H-->>CL: exit 0 (idle)
        Note over CL: container は idle 検知後に停止
    end

    loop fold worker (cc-tunnel, 2 秒間隔)
        T->>D: SELECT agent_outputs WHERE assistant_message_id = ?
        T->>D: UPDATE messages SET message_data (content_blocks)
    end

    loop frontend DB ポーリング (1 秒間隔)
        F->>T: GET /conversations/{id}
        T-->>F: ConversationDetail
    end
```
