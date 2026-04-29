# ADR: docker_gce マルチコンテナ・プール管理実装

**日時**: 2026-04-29T15:55:56+09:00  
**ステータス**: 採用済み  
**関連タスク**: cmd_cctunnel_docker_gce_multi_container (subtask_dcgmc_002a〜002d)

---

## 背景

### 設計乖離の発見

`docker_gce` provider の既存実装を調査したところ、設計書（`docs/docker-gce-design.md`）が定義する「1 GCE VM 上に最大 N 個の cc-remote-agent コンテナを動的管理する」仕様と、実際のコードの間に重大な乖離が確認された。

主な乖離点（軍師 subtask_dcgmc_001a の調査結果）:

| ID | 問題 | 影響度 |
|----|------|--------|
| G1 | startup-script が VM 起動時に1個の `docker run` のみ発行。セッション要求毎のコンテナ起動ロジックなし | 致命的 |
| G2 | コンテナ名が `cc-remote-agent` 固定（会話ごとにユニークになっていない） | 致命的 |
| G3 | ホストポートが `AgentPort` 直書き。動的割り当てなし | 致命的 |
| G4 | 2セッション目以降が同じコンテナ・同じポートを共有（セッション隔離崩壊） | 致命的 |
| G8 | Docker daemon API 呼び出しロジックが一切なし | 致命的 |

---

## 決定事項

### 1. Docker daemon 接続方式: TCP 2375

**採用**: GCE VM の Docker daemon に `tcp://0.0.0.0:2375` でアクセス。

- COS (Container-Optimized OS) は SSH バイナリなし → SSH 経由不可
- Docker daemon TCP リスナーは startup-script で有効化
- Go 実装: `internal/dockerhost/` パッケージ（`ContainerManager` インターフェース）

**セキュリティ**: TCP 2375 は暗号化なし。VPC Connector サブネットからのみ許可するファイアウォールルール（`allow-cc-tunnel-docker-api`）が必須。

### 2. コンテナ起動タイミング: セッション要求時

startup-script ではコンテナを起動しない。Docker daemon の TCP リスナー有効化のみ。
コンテナ起動は `getOrCreateEndpoint` 内で `dockerhost.ContainerManager.RunAgentContainer()` を通じて行う。

### 3. ポート動的割り当て: `MAX(port) + 1` 方式

同一 VM 上の使用中ポート最大値 + 1 を次のポートとして割り当てる。
DB の `UNIQUE (vm_instance_id, port)` 制約 + 3 回リトライでポート衝突を防ぐ。

```sql
SELECT COALESCE(MAX(port), 0) FROM session_endpoints
WHERE vm_instance_id = $1 AND status = 'running'
```

デフォルトのポートレンジ: 9091〜9200（`PortRangeStart` 設定、`MaxContainers=10` 時は 9091〜9100）。

### 4. GCE VM スケールアウト: DB 管理方式

`GetAvailableVMInstance(maxContainers)` が `active_containers < maxContainers` の VM を返す。
空き VM なし → 新 VM プロビジョン。`MaxContainers` デフォルト: 10。

### 5. コンテナ命名: `session-{conversation_id}`

会話 ID ベースのユニークな名前で Docker コンテナを管理。
同一会話の再 Execute でコンテナを一意に特定可能。

### 6. ネットワーク: `--network=bridge`（デフォルト）

Docker bridge ネットワークを明示。コンテナ間の意図しない通信を防ぐための将来拡張として `docker network create --internal` の per-container network 案を保留。

---

## バグ修正（実装中に発見）

E2E テスト実装時に、実 DB を使った統合テストで以下のバグを発見・修正した。

### BF-001: VM status が 'provisioning' のまま残る

**原因**: `waitForVMReady` 完了後に `UpdateVMInstanceStatus(ctx, vm.ID, "running")` が呼ばれていなかった。
**影響**: `GetAvailableVMInstance` が `status='running'` でフィルタするため、作成済み VM が「利用可能な VM」として検出されず、セッション毎に新 VM を作成してしまう。
**修正**: `waitForVMReady` 内の IP 更新直後に `UpdateVMInstanceStatus` を追加。

### BF-002: session_endpoint が 'provisioning' ステータスで作成される

**原因**: `CreateSessionEndpoint` の INSERT SQL がデフォルト status='provisioning' のまま。
**影響**: `GetMaxPortOnVM` は `status='running'` でフィルタするため、既存エンドポイントのポートが重複して選択され、DB の UNIQUE 制約違反が発生。
**修正**: `CreateSessionEndpoint` の INSERT に `status='running'` を明示指定。

---

## 影響範囲

### 新規追加

| ファイル | 内容 |
|---------|------|
| `apps/cc-tunnel/internal/dockerhost/interface.go` | `ContainerManager` インターフェース |
| `apps/cc-tunnel/internal/dockerhost/client.go` | Docker daemon TCP クライアント実装 |
| `apps/cc-tunnel/internal/dockerhost/mock.go` | テスト用 MockContainerManager |
| `apps/cc-tunnel/internal/provider/dockergce/e2e_test.go` | testcontainers + MockContainerManager 統合テスト |
| `adr/2026-04/2026-04-29T15:55:56+09:00_01_docker_gce_multi_container.md` | 本 ADR |

### 改修

| ファイル | 変更内容 |
|---------|---------|
| `apps/cc-tunnel/internal/provider/dockergce/provider.go` | マルチコンテナ対応・dockerhost 統合・BF-001 修正 |
| `apps/cc-tunnel/internal/db/repository.go` | `GetMaxPortOnVM` 追加・BF-002 修正（`CreateSessionEndpoint` に `status='running'`） |
| `apps/cc-tunnel/cmd/cc-tunnel/main.go` | `GCE_MAX_CONTAINERS` 環境変数読み込み追加 |
| `terraform/modules/cc-tunnel/variables.tf` | `gce_max_containers` variable 追加 |
| `terraform/modules/cc-tunnel/main.tf` | Cloud Run env に `GCE_MAX_CONTAINERS` 追加 |
| `docs/docker-gce-design.md` | 実装後の状態に更新（FW TCP 2375・マルチコンテナ説明） |
| `docs/plantuml/docker_gce_multi_container.puml` | 実際のフローを反映した完成版シーケンス図 |

### 削除

| ファイル | 理由 |
|---------|------|
| `docs/_docker_gce_multi_container_impl_guide.md` | 実装完了につき不要 |

---

## セキュリティ考慮事項

1. **Docker daemon TCP 2375**: 暗号化なし。ファイアウォールルール `allow-cc-tunnel-docker-api` で VPC Connector サブネット限定必須。
2. **コンテナリソース制限**: `--memory=512m --cpus=0.5` で単一セッションのリソース独占を防止。
3. **コンテナ分離**: bridge ネットワーク + per-conversation コンテナ名でセッション隔離を実現。

---

## 警告・注意事項

- BF-001/BF-002 は単体テスト（mockDBRepo）では露見しなかった。Mock の `CreateVMInstance` / `CreateSessionEndpoint` が status='running' を直接設定していたため。実 DB を使った E2E テストで発見。
- Terraform の `allow-cc-tunnel-docker-api` ファイアウォールルールを本番適用前に必ず追加すること。
- `MaxContainers` を超えた場合の新 VM 起動には GCE 起動時間（30〜60秒）がかかる。Warm pool 戦略（`warm_pool_size=1`）で初回レイテンシを緩和可能。
