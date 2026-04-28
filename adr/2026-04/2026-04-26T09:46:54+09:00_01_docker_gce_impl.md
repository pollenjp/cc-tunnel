# cmd_cctunnel_docker_gce_impl 変更ログ

## 概要
docker_gce Provider の mock → 本格実装。GCE VM ライフサイクル管理、
セッション管理、Execute 実装、IdleChecker によるアイドルクリーンアップ。

## Phase 別実装内容

| Phase | 担当 | 内容 |
|-------|------|------|
| Phase 0 | 足軽1号 | Docker API 接続方式検証（TCP採用確定）|
| Phase 1a | 足軽1号 | GCE クライアント層（internal/gce/）|
| Phase 1b | 足軽3号 | DB マイグレーション + Repository |
| Phase 2 | 足軽1号 | DockerGCEProvider 本体 + main.go 更新 |
| Phase 3 | 足軽3号 | IdleChecker goroutine + VMScaler goroutine |
| Phase 4 | 足軽2号 | docs 更新 + 変更ログ |

## 主要成果物

| ファイル | 変更内容 |
|---------|---------|
| internal/gce/client.go | GCEClient interface 定義 |
| internal/gce/mock_client.go | テスト用 MockGCEClient |
| internal/gce/sdk_client.go | cloud.google.com/go/compute/apiv1 実装 |
| internal/db/migrations/005_create_vm_instances.sql | VM インスタンス管理テーブル |
| internal/db/migrations/006_create_session_endpoints.sql | セッションエンドポイント管理テーブル |
| internal/db/repository.go | VMInstance/SessionEndpoint CRUD 追加 |
| internal/provider/dockergce/provider.go | DockerGCEProvider 本格実装 |
| internal/provider/dockergce/idle_checker.go | IdleChecker goroutine |
| internal/provider/dockergce/vmscaler.go | VMScaler goroutine |
| internal/docker/sdk_runner.go | NewSDKRunnerWithTCP 追加 |
| cmd/cc-tunnel/main.go | docker_gce ケースを本格実装に切り替え |
| docs/docker-gce-design.md | 実装状況反映・SSH→TCP更新・パッケージ構成追加 |
| docs/architecture.md | docker_gce 実装済み表記・実装ファイル一覧追加 |

## 設計決定事項

| 決定 | 内容 |
|------|------|
| Docker API | TCP (port 9091) 方式採用（distroless に SSH バイナリなし） |
| コンテナ起動方式 | GCE VM startup-script で直接 docker run（SSH トンネル不要） |
| VM OS | Container-Optimized OS (COS) |
| イメージ | Google Artifact Registry |
| 認証情報 | Secret Manager 経由 |
| ネットワーク | デフォルト VPC + ファイアウォールルール |
| GCE SDK | cloud.google.com/go/compute/apiv1 |
| 並行制御 | singleflight.Do（getOrCreateEndpoint） |
| VM 起動待機 | 2段階（GCE API RUNNING → cc-remote-agent health check） |
| アイドル管理 | IdleChecker（IdleCheckInterval 間隔）+ VMScaler（5分後 VM 削除） |

## 環境変数

| 変数名 | デフォルト | 説明 |
|--------|-----------|------|
| GCE_PROJECT_ID | 必須 | GCP プロジェクト ID |
| GCE_ZONE | us-central1-a | GCE ゾーン |
| GCE_MACHINE_TYPE | e2-medium | VM マシンタイプ |
| GCE_AGENT_IMAGE | 必須 | cc-remote-agent の Artifact Registry URL |
