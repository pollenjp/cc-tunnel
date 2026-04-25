# cmd_cctunnel_docs_refresh 変更ログ

## 1. 背景・目的

直近の一連の変更（Provider パターン導入、DooD 統一、SessionManager、compose 分離）を
docs/ に反映するため、全ドキュメントの最新化を実施。

## 2. 変更したファイル

（各担当足軽が変更したファイルを記載）

### 足軽1号 (subtask_docs_refresh_001a)
- docs/architecture.md: ...

### 足軽2号 (subtask_docs_refresh_001b)
- docs/docker.md: ...
- docs/directory-structure.md: ...
- docs/auth.md: ...

### 足軽3号 (subtask_docs_refresh_001c)
- docs/api.md: 変更あり
- docs/database.md: 変更なし
- docs/frontend.md: 変更なし
- docs/sequence.md: 変更あり
- docs/cloud-run-sandbox-design.md: 変更あり
- docs/docker-gce-design.md: 変更あり

### 足軽4号 (subtask_docs_refresh_001d)
- docs/plantuml/c4_container.puml: ...
- docs/plantuml/c4_component.puml: ...
- docs/plantuml/chat-activity.puml: ...
- docs/plantuml/out/: SVG再生成

## 3. 主要変更内容（足軽3号担当分）

### docs/api.md
- Internal API `POST /execute` リクエストフィールド表に `conversation_id` を追加
  - `remoteclient.Request` 構造体に追加されたフィールド（per-session コンテナルーティング用）

### docs/sequence.md
- フロー 2「メッセージ送信」を全面更新
  - **旧**: SSE（Server-Sent Events）でフロントエンドにストリーミング送信
  - **新**: 202 Accepted で即時返却 → goroutine で非同期処理 → DB ポーリング方式
  - Provider パターン（ExecutionProvider → LocalDockerProvider → SessionManager → cc-remote-agent コンテナ）を図に反映
  - content_blocks バッチ保存（2秒間隔）フローを追加

### docs/cloud-run-sandbox-design.md
- セクション 8.3「SessionProvider インターフェース」を「ExecutionProvider インターフェース」に更新
  - 旧設計の `SessionProvider`（conversationID を引数に持つ形式）を削除
  - 現在の実装に合わせた `ExecutionProvider` インターフェース定義に更新
    ```go
    type ExecutionProvider interface {
        Execute(ctx context.Context, req remoteclient.Request, onEvent func(remoteclient.StreamEvent)) (string, error)
    }
    ```
  - `LocalDockerProvider`・`SandboxProvider` の実装例を現行コードに合わせて修正
  - `handler.go` の `Server` 構造体を `executionProvider provider.ExecutionProvider` に更新

### docs/docker-gce-design.md
- セクション 8.1「handler.go の変更」を現在の実装に合わせて更新
  - `Server` 構造体の `sessionMgr sessionManager` → `executionProvider provider.ExecutionProvider` に更新
  - `SendMessage()` の呼び出しを `ExecutionProvider.Execute()` 経由の現行実装に更新
  - `LocalDockerProvider.Execute()` 内部フロー（`GetOrCreate` → `client.Execute`）を記載

## 4. 変更しなかったファイル（確認のみ）

- **docs/database.md**: スキーマ（conversations/messages、マイグレーション 001-004）は現在の実装と一致。変更不要。
- **docs/frontend.md**: DB ポーリング方式（useConversationPoller、content_blocks、ToolCallCard）の記述は現在の実装と一致。変更不要。
