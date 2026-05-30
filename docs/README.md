# ドキュメント索引

cc-tunnel のドキュメント一覧です。目的別に各サブページへのリンクと概要をまとめています。
プロジェクト全体の概要・起動手順はリポジトリルートの [`/README.md`](../README.md) を参照してください。

## 全体像・構成

| ドキュメント | 概要 |
|---|---|
| [architecture.md](./architecture.md) | システム全体のアーキテクチャ概要。ローカル開発構成と本番（Cloud Run + GCE）構成のコンポーネント構成図、各コンポーネントの責務、VM reap（dual-path）などの実行時の流れ。**hook 駆動 agent 通信 (ADR 2026-05-16) の段階別ロードマップも記載**。 |
| [directory-structure.md](./directory-structure.md) | リポジトリのディレクトリ構成。各サービス（cc-tunnel / cc-remote-agent / container-manager / frontend）のファイルレイアウトと役割。cc-remote-agent 配下の `cc-hook-bridge` も収録。 |
| [sequence.md](./sequence.md) | 主要フローのシーケンス図（Mermaid）。会話セッション作成・メッセージ送受信（shadow write 含む）・hook 駆動 agent 通信（フロー 6, 段階 5-6 想定）など。 |

## フロントエンド

| ドキュメント | 概要 |
|---|---|
| [frontend.md](./frontend.md) | React SPA の設計。コンポーネント / ページ構成、state 管理（会話リストの Zustand store、ChatView のポーリング・ローディング表示）、ルーティング、OpenAPI 生成型の利用 |
| [screen-navigation.md](./screen-navigation.md) | 画面遷移・認証フロー設計。「アプリ認証」と「Agent 認証」の 2 つの認証概念と各ガード（AppAuthGuard / CredentialGuard 等）の関係 |

## バックエンド・API

| ドキュメント | 概要 |
|---|---|
| [api.md](./api.md) | cc-tunnel の REST API リファレンス。Bearer トークン認証、会話管理 / メッセージ送信（202 + ポーリング）/ credentials 系エンドポイント、内部 API |
| [database.md](./database.md) | PostgreSQL のデータ永続化層。テーブル定義（conversations / messages / vm_instances / session_endpoints / credentials / agent_dispatches / agent_outputs）、マイグレーション、repository インターフェース（4 分割） |
| [auth.md](./auth.md) | Claude CLI 認証の仕組み。API キー方式と PTY ログイン方式、`/auth/*` エンドポイント、credentials の finalize（dual-trigger） |
| [credential-management.md](./credential-management.md) | credentials（OAuth トークン等）の管理設計。セッションコンテナ統合方式、AES-256-GCM 暗号化と DB 保管 |

## インフラ・デプロイ

| ドキュメント | 概要 |
|---|---|
| [docker.md](./docker.md) | ローカル開発の Docker Compose 構成。`apps/compose.yaml` のサービス一覧・ポート・依存関係 |
| [terraform-setup.md](./terraform-setup.md) | GCP インフラ（Terraform / Terragrunt）のセットアップガイド。カスタム VPC・Firewall、Cloud Build / Cloud Run、Cloud SQL、HTTPS LB、IAP、Cloud Logging、VM reap（Cloud Scheduler）など |

## セッション隔離方式の設計

| ドキュメント | 概要 |
|---|---|
| [docker-gce-design.md](./docker-gce-design.md) | Docker on GCE 方式の詳細設計書。会話ごとに GCE VM 上でセッションコンテナを起動して隔離する方式（採用済み） |
| [cloud-run-sandbox-design.md](./cloud-run-sandbox-design.md) | Cloud Run Sandbox 方式の詳細設計書。Google の cloud-run-sandbox を活用した第 2 の実行方式の検討 |

## 図

| ディレクトリ | 概要 |
|---|---|
| [plantuml/](./plantuml/) | C4 図・シーケンス図・アクティビティ図などの PlantUML ソース（`*.puml`）と生成済み SVG（`out/`） |

### 主要な PlantUML 図 (生成済み SVG は `plantuml/out/`)

| ファイル | 概要 |
|---|---|
| `plantuml/c4_context.puml` | C4 Level 1 (System Context)。cc-tunnel と外部システム (ユーザー / Anthropic / Docker daemon)。 |
| `plantuml/c4_container.puml` | C4 Level 2 (Container)。frontend / cc-tunnel API / container-manager / cc-remote-agent / PostgreSQL とプロバイダ抽象。`agent_dispatches` / `agent_outputs` (ADR 2026-05-16) も注記。 |
| `plantuml/c4_component.puml` | C4 Level 3 (Component)。cc-tunnel API・cc-remote-agent (Handler / claude / cc-hook-bridge)・Frontend の主要コンポーネントと依存関係。 |
| `plantuml/agent_dispatch_shadow_write.puml` | **本 PR (ADR 2026-05-16 段階 1-3) のシーケンス**。`agent_dispatches` への shadow write が既存 `/execute` 経路と並走する様子。 |
| `plantuml/agent_dispatch_sequence.puml` | **段階 5-6 完了後の想定シーケンス**。PTY 常駐 `claude` + `cc-hook-bridge` 経由で `agent_outputs` が積み上がり、Stop hook が次 dispatch を block 注入する loop。 |
| `plantuml/docker_gce_architecture.puml` ほか | Docker on GCE 隔離方式の VM / container-manager / セッションコンテナ構成、idle cleanup、multi-container 等。 |
| `plantuml/sandbox_*.puml` | Cloud Run Sandbox 方式の検討用図 (採用未確定)。 |
| `plantuml/screen_navigation.puml` / `chat-activity.puml` / `useConversationPoller-activity.puml` | 画面遷移・チャット UI・ポーリングのアクティビティ図。 |
