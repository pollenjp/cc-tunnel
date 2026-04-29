# docker_gce マルチコンテナ実装ガイド（一時ファイル）

> **このファイルは subtask_dcgmc_001a で軍師が策定した足軽向け実装ガイドである。**
> **完了後（cmd 全体クローズ時）に削除すること。**

## 0. 目的

殿の期待仕様「**1 GCE インスタンス上で最大 N（=10）個の cc-remote-agent コンテナをプール管理し、
満杯時に新 GCE を自動起動**」を満たすよう、現行 `docker_gce` provider を改修する。

## 1. 現状調査結果（コードを読んで確認した事実）

### 1.1 既に実装されている要素（再利用可）

- DB スキーマは既にマルチコンテナ前提で設計済み
  - `vm_instances.active_containers` カウンタ
  - `vm_instances.idle_since` (active=0 で NOW() に設定)
  - `GetAvailableVMInstance(maxContainers)`: `WHERE active_containers < N ORDER BY active_containers DESC` ←bin-pack 動作OK
  - `IncrementVMActiveContainers / DecrementVMActiveContainers`
  - `ListIdleVMInstances` / `ListIdleSessionEndpoints`
- `DockerGCEConfig.MaxContainers` フィールドは存在（`provider.go:43`）
- `DockerGCEProvider.Execute` → `getOrCreateEndpoint` → `GetAvailableVMInstance(MaxContainers)` 順の制御フロー
- `singleflight` による同一 conversation_id の重複作成防止（AF002）
- `waitForVMReady` 2 段階（GCE RUNNING → /auth/status 200）
- `IdleChecker` / `VMScaler` goroutine スケルトン
- `CleanupOrphans` で idle endpoint と idle VM をまとめて削除

### 1.2 重大な乖離（**実装が「1 VM = 1 コンテナ」しか動作しない**）

| # | 乖離項目 | 現行コード | 期待仕様 | 影響度 |
|---|---------|-----------|---------|-------|
| G1 | **コンテナ起動経路** | `buildStartupScript()` が VM 起動時に **1 個だけ** `docker run` する。2 個目以降を起動するロジック不在 | セッション要求毎に `docker run` を VM 上で発行 | **致命的** |
| G2 | **コンテナ名** | 全 VM 共通で `cc-remote-agent` 固定 (`provider.go:268`, `CreateSessionEndpoint` も `"cc-remote-agent"` 固定 `provider.go:173`) | `session-{conv_id}` 等で会話毎ユニーク | **致命的** |
| G3 | **ホストポート** | startup-script で `-p {AgentPort}:{AgentPort}`、DB にも `cfg.AgentPort` を直書き | 9091〜9200 の動的割当（`docker-gce-design.md` §3.4） | **致命的** |
| G4 | **2 セッション目の動作** | `GetAvailableVMInstance` で既存 VM が見つかると、新たな `docker run` をせずに **同じコンテナ・同じポート** に session_endpoint を作る。複数会話が同一 claude プロセスを共有 | 会話毎に隔離コンテナ | **致命的（隔離崩壊）** |
| G5 | **MaxContainers 設定** | `cmd/cc-tunnel/main.go:158-165` の `DockerGCEConfig` に `MaxContainers` を渡していないため、`provider.go:83-85` のデフォルト `1` で確定 | env から取得（既定 10） | **大** |
| G6 | **IdleCheckInterval 設定** | `main.go` で渡していない → `IdleChecker` 永久未起動 (`provider.go:105`) | 60s で起動 | **大** |
| G7 | **VMScaler 起動** | `vmscaler.go` は実装済みだが `NewDockerGCEProviderWithClientFactory` が `IdleChecker` しか起動しない（`provider.go:106-109`）。VMScaler は **誰からも `Start` されていない** | アイドル VM 削除間隔で常駐 | **大** |
| G8 | **Docker daemon API 呼出** | 一切なし。VM 上の Docker と通信する Go コードが存在しない | TCP 2375 経由 or HTTP API クライアントで `containers.create` / `start` / `stop` / `rm` | **致命的** |
| G9 | **コンテナ削除** | `CleanupOrphans` は session_endpoint を DB から消すのみ。Docker コンテナへの `docker stop/rm` は **発行されない**。VM 全体を `gce.DeleteInstance` で消す経路だけ存在 | 個別コンテナ停止 + VM が空になったら VM 削除 | **大** |
| G10 | **リソース制限** | `docker run` に `--memory --cpus` なし | `--memory=512m --cpus=0.5`（design §2.3） | **中** |
| G11 | **ANTHROPIC_API_KEY 注入** | startup-script に環境変数注入なし | Secret Manager から取得しコンテナ env に | **中**（先行 cclogin で credentials.json 注入経路があるため、認証は別経路で動く可能性高い。要再確認） |
| G12 | **ネットワーク隔離** | `--network=bridge` 明示なし（default は bridge なので実質 OK） | bridge 明示で意図を残す | **小** |
| G13 | **Terraform env** | `terraform/modules/cc-tunnel/main.tf` に `GCE_MAX_CONTAINERS` env 定義なし | Cloud Run env として注入 + `variables.tf` に variable 追加 | **大**（本番が default=1 のままになる） |

### 1.3 副次的観察

- `apps/cc-tunnel/internal/provider/dockergce/idle_checker_test.go` は既存。現行 IdleChecker 自体のロジックは正しい。
- `provider_test.go::TestDockerGCEProvider_Execute_ExistingSession` は **同じ VM・同じポート再利用** をテストしている。マルチコンテナ実装後は「同じ VM・別ポート・別コンテナ」になるので **既存テストの assertion 自体が変わる**。
- `CleanupOrphans` の VM 削除は「全 idle VM を毎回削除」する強気な実装。Warm pool を入れるとここで競合する。本実装ガイドでは Warm pool は対象外。
- `cc_remote_agent_auth_retire`（subtask_ccauth_001b〜001d）が **同時並行で進行中**。`handler.go`・`provider.go`（インタフェース）・`cmd/main.go` を編集する。**RACE-001 を避けるため本タスク群はそれらに触ってはならぬ**。

## 2. 設計判断（軍師の見解）

### 2.1 Docker daemon との通信方式

| 案 | 概要 | 採否 |
|----|------|------|
| **A. Docker daemon TCP (2375)** | VM startup-script で `daemon.json` に `tcp://0.0.0.0:2375` を設定（既に実装済）、cc-tunnel が `github.com/docker/docker/client` で接続 | **採用** |
| B. SSH 経由 docker exec | COS は SSH バイナリなし | 不採用（`docker-gce-design.md` §3.3） |
| C. cc-remote-agent に「コンテナ管理 API」を生やす | 親 cc-remote-agent が子 cc-remote-agent を docker run する | 親子関係が複雑、ただし TCP 2375 を VPC 内で開ける必要がない利点。**fallback 案として保留**。 |

**採用: A（Docker over TCP）**。design doc が既に A を選択しており、startup-script も TCP リスナーを既に設定している。

> **セキュリティ補足**: TCP 2375 は **暗号化なし**。VPC 内（VPC Connector 経由でしか到達不可）+ ファイアウォールで cc-tunnel SA のサブネットからのみ許可することで保護する。本番有効化前にファイアウォールルールの明示が必須（足軽が Terraform 改修時に追加）。

### 2.2 ポート割当方式

| 案 | 採否 |
|----|------|
| **動的割当（DB で `MAX(port)+1` を計算 / 9091-9200）** | **採用**（design §3.4 と一致） |
| 固定範囲を pre-allocate | 不採用（複雑化、DB と整合の難しさ） |

```sql
-- 同一 VM 上の使用中ポート最大値+1。MAX(port)=9090 のとき 9091
SELECT COALESCE(MAX(port), 9090) + 1
FROM session_endpoints
WHERE vm_instance_id = $1 AND status = 'running';
```

**注意**: ポート上限到達（9200 まで使い切り）の判定が必要。MaxContainers=10 なので通常は 9091-9100 までしか使わないが、削除→再作成で穴が空いた状態を考慮し、`MAX(port)` 方式は MaxContainers と整合させて使う。

実装簡略化のため、**`9091 + active_containers` ではなく** トランザクション内で `MAX(port)+1` を採るのが正しい（穴埋めはせず単純加算でも、MaxContainers 上限以下なら 9100 を超えない）。

### 2.3 ネットワーク隔離

design 仕様通り `--network=bridge` を採用。Docker bridge は既定でコンテナ間通信が L2 で隔離されない（`172.17.0.0/16` 内で互いに ping 可能）が、cc-remote-agent は外向き Anthropic API しか叩かないので **同一 VM 内コンテナ間の HTTP 攻撃リスクは低い**。本タスクではここまで。

将来拡張として `docker network create --internal session-{conv_id}` で per-container network を切る案を ADR に記録するに留める。

### 2.4 GCE スケールアウト判断方式

DB 管理（`active_containers < MaxContainers`）を採用。現行の `GetAvailableVMInstance` がそのまま使える。**変更不要**。

### 2.5 cc-remote-agent イメージは既存をそのまま使用

cc-remote-agent コンテナ側のコード変更は **不要**。port 9091 で listen するロジックを変更しないが、ホスト側ポート（VM の `9091+N`）→ コンテナ内ポート（9091）にマッピングするだけ。

## 3. 実装フェーズ分割（足軽 subtask 提案）

### Phase A（必須・先行）: Docker daemon クライアント層と provider 改修

`cc_remote_agent_auth_retire` の **完了後** に着手することを強く推奨（main.go と provider.go interface を後者が触るため）。

#### subtask_dcgmc_002a: Docker daemon クライアント層
**担当推奨**: 足軽4または5（idle）
**ファイル新規**:
- `apps/cc-tunnel/internal/dockerhost/client.go` ← Docker daemon TCP 接続クライアント
- `apps/cc-tunnel/internal/dockerhost/client_test.go` ← unit test (mock httptest)

```go
// インタフェース提案
package dockerhost

type Client interface {
    // RunAgentContainer は VM 上で cc-remote-agent コンテナを起動する
    RunAgentContainer(ctx context.Context, req RunRequest) error
    StopContainer(ctx context.Context, name string) error
    RemoveContainer(ctx context.Context, name string) error
}

type RunRequest struct {
    VMHost        string  // e.g. "10.128.0.5"
    DockerPort    int     // 2375
    Image         string
    ContainerName string  // "session-{conv_id}"
    HostPort      int     // 9091..9100
    AgentPort     int     // 9091
    EnvVars       map[string]string
    MemoryLimit   string  // "512m"
    CPULimit      string  // "0.5"
}
```

実装は `github.com/docker/docker/client` をベースに `client.NewClientWithOpts(client.WithHost("tcp://10.128.0.5:2375"))` で接続。TDD でモック HTTP サーバーを使ったテストを書く。

#### subtask_dcgmc_002b: DockerGCEProvider マルチコンテナ対応
**担当推奨**: 足軽1または2（cclogin/ccauth で土地勘がある）
**前提**: subtask_dcgmc_002a 完了後
**前提**: subtask_ccauth_001b〜001d 完了後（main.go・provider.go interface・handler.go の競合回避）
**ファイル変更**:
- `apps/cc-tunnel/internal/provider/dockergce/provider.go` （主改修）
- `apps/cc-tunnel/internal/provider/dockergce/provider_test.go` （TDD）

**主な変更点**:
1. `DockerGCEConfig` に追加:
   ```go
   ContainerNamePrefix string  // default "session"
   PortRangeStart int          // default 9091
   PortRangeEnd   int          // default 9200
   MemoryLimit    string       // default "512m"
   CPULimit       string       // default "0.5"
   DockerHostPort int          // default 2375
   ```
2. `getOrCreateEndpoint` のフロー変更（マルチコンテナ対応）:
   ```
   既存 endpoint あり? → return（変更なし）
   ↓ なし
   利用可 VM 検索（GetAvailableVMInstance(MaxContainers)）
   ├ 既存 VM hit
   │   ├ 同一 VM 上で次の空きポート選択（MAX(port)+1 をリポジトリに追加）
   │   ├ dockerhost.Client.RunAgentContainer(VMホスト, 動的ポート, コンテナ名=session-{conv})
   │   ├ remoteclient で /auth/status を待機（agent ready 確認）
   │   ├ CreateSessionEndpoint(conv, vm.ID, "session-"+conv, port)
   │   └ IncrementVMActiveContainers
   └ VM なし
       ├ createGCEVM（既存）→ waitForVMReady（既存）
       ├ ※ startup-script で **コンテナを起動しない**（後述）
       ├ dockerhost.Client.RunAgentContainer（新 VM 上）
       └ 同じ session_endpoint 登録
   ```
3. `buildStartupScript` 変更: **コンテナ起動を削除**し、Docker daemon TCP リスナー設定のみ残す:
   ```bash
   #!/bin/bash
   mkdir -p /etc/docker
   echo '{"hosts":["tcp://0.0.0.0:2375","unix:///var/run/docker.sock"]}' > /etc/docker/daemon.json
   systemctl restart docker 2>/dev/null || true
   sleep 5
   docker pull <image> || true   # 任意（先プル）
   ```
4. `waitForVMReady` 変更: Stage 2（agent ready）は VM 起動時には実行できなくなる（コンテナがまだない）。Stage 1（GCE RUNNING + IP）+ **Docker daemon 到達確認**（TCP 2375 ping）で完了とする。Stage 2 はコンテナ起動後に移す。
5. `CleanupOrphans` 変更:
   - idle endpoint 検出 → `dockerhost.Client.StopContainer / RemoveContainer` を呼ぶ（**現状欠落**）
   - その後 DB 削除 + DecrementVMActiveContainers
   - VM が空になったら従来通り `gce.DeleteInstance`
6. `NewDockerGCEProviderWithClientFactory` 変更:
   - `dockerhost.Client` ファクトリを config / 引数で受け取る
   - `VMScaler` を `IdleChecker` と並んで起動

**リポジトリ追加**:
- `apps/cc-tunnel/internal/db/repository.go` に `GetMaxPortOnVM(ctx, vmID) (int, error)` を追加（または `ListSessionEndpointsByVM`）。

**テスト**:
- 同一 VM に 3 セッション作成 → 3 個の異なるポート + 異なるコンテナ名で `RunAgentContainer` が 3 回呼ばれる
- 4 セッション目（MaxContainers=3 の場合）→ 新 VM 作成
- アイドル endpoint → `StopContainer/RemoveContainer` が呼ばれる
- VM idle → VMScaler が GCE delete を発行
- singleflight 同時呼出での重複防止（既存テスト）

#### subtask_dcgmc_002c: 設定経路 + Terraform
**担当推奨**: 足軽3
**前提**: subtask_ccauth_001b 完了（main.go の他改修との競合回避）
**ファイル変更**:
- `apps/cc-tunnel/cmd/cc-tunnel/main.go` の `newProviderFromEnv("docker_gce")`:
  ```go
  cfg := dockergce.DockerGCEConfig{
      ProjectID:           gceProjectID,
      Zone:                getEnvOrDefault("GCE_ZONE", "us-central1-a"),
      MachineType:         getEnvOrDefault("GCE_MACHINE_TYPE", "e2-medium"),
      AgentImage:          agentImage,
      AgentPort:           9091,
      MaxContainers:       getEnvIntOrDefault("GCE_MAX_CONTAINERS", 10),
      IdleTimeout:         15 * time.Minute,
      IdleCheckInterval:   60 * time.Second,
      DockerHostPort:      2375,
  }
  ```
- `terraform/modules/cc-tunnel/variables.tf`: `gce_max_containers` (default 10) を追加
- `terraform/modules/cc-tunnel/main.tf`: Cloud Run env に `GCE_MAX_CONTAINERS` を注入
- `terraform/modules/cc-tunnel/cc-remote-agent.tf` 等のファイアウォール定義: タグ `cc-tunnel-agent` への TCP 2375 を VPC Connector サブネットからのみ許可（**未存在なら新規作成**）

#### subtask_dcgmc_002d: E2E 検証 + ドキュメント整理
**担当推奨**: 軍師（E2E テストは家老が実施でも可。本件は cc-tunnel の単体 E2E）
**前提**: 002a〜002c 完了
**作業**:
- 既存 `docs/docker-gce-design.md` の「実装済み」表記が 002b 後に正しくなる箇所を更新
- `docs/plantuml/docker_gce_session_start.puml` 等を新フローに合わせて更新
- 新 ADR 作成: `adr/2026-04/{ts}_01_docker_gce_multi_container.md`
- `_docker_gce_multi_container_impl_guide.md` を削除（cmd 完了時）

### 3.1 cc_remote_agent_auth_retire との依存関係

| Phase | 依存 | 並行可否 |
|------|-----|---------|
| **subtask_dcgmc_002a (dockerhost クライアント)** | なし | **完全並行可** |
| subtask_dcgmc_002b (provider 改修) | provider.go interface（auth_retire が `GetSessionClient` を追加するが、既存の `GetSessionClient` メソッドは既に provider.go:301 にある）| auth_retire の **完了後**（main.go・provider.go interface の競合回避） |
| subtask_dcgmc_002c (設定 + Terraform) | main.go（auth_retire が触る） | auth_retire の **完了後** |
| subtask_dcgmc_002d (E2E + docs) | 002a-c 全部 | 直列 |

**結論**: **subtask_dcgmc_002a だけ即座並行投入可。002b〜002d は ccauth_001b〜001d 完了後**。

## 4. TDD サイクル手順（足軽向け）

各 subtask 共通:

1. 任務読み取り → 実装ガイド本書を熟読
2. 既存テストを `go test ./...` で実行し緑であることを確認
3. **失敗するテストを先に書く**（Red）
4. 実装（Green）
5. リファクタ（Refactor）
6. `mise run check` 全パス確認（SKIP は絶対値で報告）
7. `git diff --stat` で変更ファイル一覧を採取し報告書に記載

## 5. 想定される躓きどころ

| 躓き | 対処 |
|------|-----|
| Docker daemon TCP 2375 がリッスンしない | startup-script の `systemctl restart docker` 後に `nc -z 10.128.0.5 2375` で疎通確認するまで待つ。`waitForVMReady` Stage1 を「RUNNING + IP + TCP 2375 reachable」に拡張せよ |
| `docker/docker/client` のバージョン整合 | go.mod に既存があれば再利用。なければ `client/v25` 系を入れる。`API version negotiation` を有効化 |
| ポート競合 | 必ず `MAX(port)+1` を **DB トランザクション内で取得**（FOR UPDATE）し、同一 VM 上の同時作成で衝突しないようにする |
| singleflight キーが conversationID なので、同一 VM への並行作成は singleflight で防げない | **VM 単位の追加 mutex** を `DockerGCEProvider` に持つか、DB で UNIQUE 制約 `(vm_instance_id, port)` を活用 |
| `cc-remote-agent` イメージ内部の `cc-tunnel` 認証 endpoint ルーティングが ccauth_retire で変わる | dockergce.provider が認証 endpoint に直接触らないため影響少。注意点として `getOrCreateEndpoint` 内で `client.GetAuthStatus` を ready 判定に使っている部分（provider.go:245）→ ccauth_retire 後も `/auth/status` 自体は残るので OK。ただし conversationId クエリ必須化された場合は ready 判定を別 endpoint（`/init` 等）に切替必要 |
| credentials.json は session コンテナの `~/.claude/.credentials.json` に乗る（cclogin v2 で実装済） | マルチコンテナ化後も同じ。コンテナ起動時に env としてコンテナへ流れる経路を `dockerhost.Client.RunAgentContainer` の `EnvVars` で確保 |
| MaxContainers=1 時の互換性 | 現行テスト（既存 1 セッションのみ）が引き続き緑であること |
| 旧 startup-script のコンテナ起動が残ると 9091 占有 | 002b で startup-script を必ず変更すること（新コンテナ起動の docker run と衝突する） |

## 6. 完了基準

- 1 GCE VM 上で 10 個の cc-remote-agent コンテナが並行稼働できる
- 11 個目の要求で新 VM が自動起動する
- 全コンテナ idle で VM が削除される
- `mise run check` 全パス（SKIP=0、または環境スキップは根拠付き）

## 7. 参考ファイル一覧（read 必須）

- `apps/cc-tunnel/internal/provider/dockergce/provider.go`
- `apps/cc-tunnel/internal/provider/dockergce/idle_checker.go`
- `apps/cc-tunnel/internal/provider/dockergce/vmscaler.go`
- `apps/cc-tunnel/internal/db/repository.go`
- `apps/cc-tunnel/cmd/cc-tunnel/main.go`
- `docs/docker-gce-design.md` （仕様書）
- `terraform/modules/cc-tunnel/main.tf` （Cloud Run env）
- `terraform/modules/cc-tunnel/variables.tf`

---
作成: 2026-04-29 軍師（subtask_dcgmc_001a）
