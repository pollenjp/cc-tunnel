# cmd_cctunnel_local_docker_per_session 変更ログ

## 1. 経緯・背景
- 旧 local provider は常駐 cc-remote-agent 1台（全会話共有）
- 会話セッション:コンテナ=1:1 方式に変更し、状態分離とスケーラビリティを実現
- cc-tunnel は distroless ベースのため Docker SDK 一択（CLI 不可）
- 軍師事前設計レビュー（gunshi_local_docker_design）に基づく実装

## 2. 設計決定（軍師レビュー結果）
- Auth: cc-remote-agent-auth（専用常駐コンテナ）→ nil remote 問題も根本解消
- Docker 操作: Docker SDK (github.com/docker/docker v28.5.2+incompatible)
- ネットワーク: compose 同一ネットワーク (apps_default) + DNS 接続
- ライフサイクル: Lazy start + 15分 idle timeout + 3層クリーンアップ
- インターフェース: ExecutionProvider 拡張なし、Request.ConversationID 追加のみ
- ボリューム: 単一共有 claude-sessions

## 3. 変更内容

### 3.1 インフラ
- apps/prepare.compose.yaml 新規作成（cc-remote-agent イメージビルド専用）
- apps/compose.yaml 変更:
  - cc-remote-agent → cc-remote-agent-auth（build削除、pre-built image使用）
  - cc-tunnel: docker.sock マウント追加、EXECUTION_PROVIDER: local 追加
  - name: apps 追加（ネットワーク名固定）

### 3.2 Go 実装
- internal/remoteclient/client.go: Request に ConversationID フィールド追加
- internal/api/handler.go: SendMessage で ConversationID をセット
- internal/docker/runner.go: DockerRunner interface 定義
- internal/docker/sdk_runner.go: SDKRunner（Docker SDK 実装）
- internal/docker/session_manager.go: SessionManager（コンテナライフサイクル管理）
- internal/provider/local/docker_provider.go: LocalDockerProvider（per-session 実装）
- cmd/cc-tunnel/main.go:
  - newProviderFromEnv シグネチャ変更（agentURL 削除、LocalDockerProvider 生成）
  - auth remote を独立生成（remote = remoteclient.NewClient(*agentURL)）
  - SIGTERM ハンドラで graceful shutdown
  - 起動時孤児コンテナクリーンアップ

## 4. テスト追加
- internal/docker/session_manager_test.go: SessionManager TDD（5テスト）
- internal/provider/local/docker_provider_test.go: LocalDockerProvider TDD（4テスト）
- cmd/cc-tunnel/main_test.go: 新シグネチャ対応更新

## 5. 品質確認
- mise run check: PASS（SKIP=0, FAIL=0）
- lint: 0 issues
