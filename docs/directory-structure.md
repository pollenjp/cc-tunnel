# ディレクトリ構成

生成ファイル (gen.go, schema.d.ts, node_modules, dist) は省略。

```
cc-tunnel/
├── .github/
│   └── workflows/
│       └── ci.yml                    # CI パイプライン (lint / test / build)
├── README.md                         # プロジェクト概要・セットアップ手順
├── logs/                             # cmd 実行ログ記録ディレクトリ (git-ignored)
├── design/
│   └── architecture.md               # 設計検討ドキュメント
├── docs/
│   ├── api.md                        # API リファレンス (External/Internal)
│   ├── architecture.md               # アーキテクチャ概要・データフロー・技術スタック
│   ├── database.md                   # DB スキーマ・マイグレーション・Repository パターン
│   ├── directory-structure.md        # 本ファイル
│   ├── docker.md                     # Docker 運用ガイド
│   ├── frontend.md                   # フロントエンド開発ガイド
│   ├── plantuml/                     # PlantUML アクティビティ図
│   │   ├── chat-activity.puml        # フロントエンド チャット機能全体フロー
│   │   └── useConversationPoller-activity.puml  # useConversationPoller 内部フロー
│   └── sequence.md                   # シーケンス図
└── apps/
    ├── compose.yaml                  # Docker Compose 定義 (デフォルト: postgres + cc-remote-agent-auth, profiles: full で cc-tunnel + frontend 追加)
    ├── prepare.compose.yaml          # cc-remote-agent イメージビルド専用 (compose.yaml 使用前に実行)
    ├── mise.toml                     # monorepo タスクランナー設定
    ├── openapi/
    │   ├── openapi.yaml              # cc-tunnel REST API スキーマ定義
    │   ├── oapi-codegen.yaml         # oapi-codegen 設定 (Go サーバー生成)
    │   └── README.md                 # OpenAPI ワークフロー説明
    │
    ├── cc-remote-agent/              # Claude CLI ラッパーサービス (Go)
    │   ├── Dockerfile
    │   ├── go.mod
    │   ├── go.sum
    │   ├── mise.toml                 # サービス固有タスク (build / run)
    │   ├── cmd/
    │   │   └── cc-remote-agent/
    │   │       └── main.go           # エントリーポイント: HTTP サーバー起動・ルーティング
    │   └── internal/
    │       ├── api/
    │       │   └── handler.go        # HTTP ハンドラー (/execute, /health, /auth/*)
    │       ├── auth/
    │       │   └── manager.go        # PTY 認証フロー管理 (creack/pty, outputBuf)
    │       ├── claude/
    │       │   └── executor.go       # claude CLI exec・ndjson ストリーム処理
    │       └── logging/
    │           └── handler.go        # ErrorStackHandler: error 属性検出時にスタックトレース付与
    │
    ├── cc-tunnel/                    # API ゲートウェイ兼会話管理サービス (Go)
    │   ├── Dockerfile
    │   ├── go.mod
    │   ├── go.sum
    │   ├── .golangci.yml             # golangci-lint 設定
    │   ├── mise.toml                 # サービス固有タスク (build / generate / run)
    │   ├── README.md
    │   ├── docs/
    │   │   └── architecture.md       # cc-tunnel アプリ固有アーキテクチャ設計
    │   ├── cmd/
    │   │   └── cc-tunnel/
    │   │       ├── main.go           # エントリーポイント: DB 接続・ExecutionProvider 選択・HTTP サーバー起動
    │   │       └── main_test.go      # エントリーポイントテスト
    │   └── internal/
    │       ├── api/
    │       │   ├── handler.go        # HTTP ハンドラー (SendMessage: 202 即時返却 + goroutine 実行)
    │       │   ├── mapping.go        # DB→API 変換コンストラクタ (newConversation / newMessage / newConversationDetail)
    │       │   ├── gen.go            # oapi-codegen 生成コード (ServerInterface / HandlerFromMux)
    │       │   └── handler_test.go   # ハンドラーユニットテスト
    │       ├── db/
    │       │   ├── db.go             # pgx 接続プール・goose マイグレーション実行
    │       │   ├── repository.go     # 会話・メッセージ CRUD (Repository)
    │       │   └── migrations/
    │       │       ├── 001_create_conversations.sql       # conversations テーブル
    │       │       ├── 002_create_messages.sql            # messages テーブル
    │       │       ├── 003_add_conversation_status.sql    # conversations.status カラム追加
    │       │       └── 004_add_message_status.sql         # messages.status / updated_at カラム追加
    │       ├── docker/               # Docker デーモン操作の抽象化レイヤー
    │       │   ├── runner.go         # DockerRunner interface + ContainerCreateOpts / ContainerInfo 型定義
    │       │   ├── sdk_runner.go     # SDKRunner: Docker SDK を使った DockerRunner 実装
    │       │   ├── session_manager.go    # SessionManager: per-session cc-remote-agent コンテナの生成・管理
    │       │   └── session_manager_test.go  # SessionManager ユニットテスト
    │       ├── logging/
    │       │   └── handler.go        # ErrorStackHandler: error 属性検出時にスタックトレース付与
    │       ├── provider/             # ExecutionProvider 抽象化レイヤー
    │       │   ├── provider.go       # ExecutionProvider interface 定義
    │       │   ├── provider_test.go  # インターフェース適合テスト
    │       │   ├── local/            # LocalDockerProvider: SessionManager 経由の per-session 実行
    │       │   │   ├── local.go      # LocalDockerProvider 公開コンストラクタ
    │       │   │   ├── docker_provider.go   # LocalDockerProvider 実装 (Execute メソッド)
    │       │   │   ├── docker_provider_test.go  # LocalDockerProvider テスト
    │       │   │   └── local_test.go  # ローカルプロバイダーテスト
    │       │   ├── cloudrunsandbox/  # CloudRunSandboxProvider (モック実装)
    │       │   │   ├── mock.go
    │       │   │   └── mock_test.go
    │       │   └── dockergce/        # DockerGCEProvider (モック実装)
    │       │       ├── mock.go
    │       │       └── mock_test.go
    │       └── remoteclient/
    │           └── client.go         # cc-remote-agent HTTP クライアント (Execute / Auth*)
    │
    └── frontend/                     # React SPA (Vite + Tailwind CSS v4)
        ├── Dockerfile
        ├── nginx.conf.template       # 本番 nginx 設定 (/api/* → cc-tunnel プロキシ)
        ├── docker-entrypoint.sh      # 環境変数を env.js に書き出してから nginx 起動
        ├── package.json
        ├── vite.config.ts            # Vite 設定 (@tailwindcss/vite プラグイン)
        ├── index.html
        ├── tsconfig.app.json
        └── src/
            ├── main.tsx              # React アプリエントリーポイント
            ├── App.tsx               # ルートコンポーネント・URL ルーティング・会話一覧管理
            ├── App.css               # CSS カスタムプロパティ (カラートークン)
            ├── index.css             # Tailwind CSS インポート (@import "tailwindcss")
            ├── env.d.ts              # 環境変数型定義
            ├── api/
            │   ├── client.ts         # openapi-fetch ベース API クライアント
            │   ├── client.test.ts    # API クライアントユニットテスト
            │   └── schema.d.ts       # openapi-typescript 生成型 (コミット済み)
            ├── components/
            │   ├── AuthGuard.tsx     # 認証状態に応じた Chat UI / AuthTerminal 切り替え
            │   ├── AuthTerminal.tsx  # @xterm/xterm ベース PTY 認証ターミナル
            │   ├── ChatView.tsx      # 会話ビュー (メッセージ一覧・送信・ポーリングを自己完結)
            │   ├── MessageBubble.tsx # メッセージ表示 (react-markdown / シンタックスハイライト)
            │   ├── MessageInput.tsx  # テキスト入力フォーム (Shift+Enter で改行)
            │   ├── Sidebar.tsx       # 会話リスト・新規作成・ログアウトボタン
            │   ├── ToolCallCard.tsx  # ツール使用状況表示カード
            │   └── TypingIndicator.tsx  # 進行中インジケータ (typing-shimmer アニメーション)
            └── hooks/
                ├── useAuth.ts                    # 認証状態管理フック (ポーリング・login / logout)
                ├── useConversationPoller.ts      # 会話ポーリングフック (1秒間隔・completed で停止)
                └── useConversationListPoller.ts  # 会話一覧ポーリングフック (running 中 3秒間隔)
```
