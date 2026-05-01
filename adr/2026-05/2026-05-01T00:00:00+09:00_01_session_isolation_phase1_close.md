# ADR: session-isolation Phase 1 クローズ条件と現状監査

- **Date**: 2026-05-01
- **Status**: Accepted
- **関連設計**: `design/session-isolation.md` §11 (実装フェーズ)
- **関連 ADR**:
  - `2026-04-23T18:30:46+09:00_01_docker_gce_design.md`
  - `2026-04-24T18:08:05+09:00_01_local_docker_per_session.md`
  - `2026-04-26T09:46:54+09:00_01_docker_gce_impl.md`
  - `2026-04-29T12:55:24+09:00_01_cc_remote_agent_auth_retire.md`
  - `2026-04-29T15:55:56+09:00_01_docker_gce_multi_container.md`
  - `2026-04-29T20:01:36+09:00_01_session_auth_gate.md`

---

## 背景

`design/session-isolation.md` §11 は session-isolation の実装を Phase 1〜4 に分けたが、
クローズ条件（acceptance criteria）が文章として定義されていない。

実際には Phase 1 の対象範囲は概ね 1〜2 週間で実装が進み、Phase 2・3 の項目も
`cmd_cctunnel_docker_gce_*` および `cmd_cctunnel_cc_login_*` 系のタスクで
前倒しで完了している。一方で「Phase 1 が完了したと言えるのか」「残作業は何か」が
ADR に集約されておらず、レビュー時に複数の ADR を横串で読まなければ把握できない
状態にある。

本 ADR は以下を行う:

1. Phase 1 のクローズ条件（AC）を**定量・テスト可能な形**で定義する。
2. 現コードを各 AC に突き合わせ、done / partial / missing を確定する。
3. 残った未充足 AC に対する最小クローズ作業を列挙する。
4. design 文書（`design/session-isolation.md`）と実装の乖離点を明文化し、
   今後 Phase 2 以降を再開する際の正しいベースラインを残す。

---

## 決定事項

### 1. Phase 1 のスコープ再定義

`design/session-isolation.md` §11 における Phase 1 の元定義:

> - DB マイグレーション (session_endpoints, vm_instances)
> - SessionManager 基本実装 (GetOrCreateEndpoint, ReleaseEndpoint)
> - GCE VM プロビジョニング (Compute Engine API)
> - Docker コンテナ管理 (SSH 経由)

実装上の対応関係を以下に固定する。

| 設計上の名称 | 実装上の対応 |
|---|---|
| SessionManager | `internal/provider/dockergce.DockerGCEProvider`（独立パッケージは作らない） |
| GCE VM プロビジョニング | `internal/gce.SDKGCEClient` + `DockerGCEProvider.GetOrCreateEndpoint` |
| Docker コンテナ管理 | `internal/dockerhost.ContainerManager`（SSH ではなく **TCP 2375**, ADR `2026-04-29_docker_gce_multi_container` で正式に上書き決定） |
| ローカル開発相当 | `internal/provider/local.LocalDockerProvider` + `internal/docker.SessionManager` |

「SessionManager」を独立パッケージとして切る案は採用しない。`ExecutionProvider`
インターフェースが既にセッションの取得・実行・relogin・credentials 取出しを
網羅しており、屋上屋を架す形になるため。

### 2. Phase 1 クローズ条件 (AC)

以下 8 件全てを満たした時点を Phase 1 完了とする。

| ID | AC | 検証方法 |
|----|----|----------|
| AC-1 | `session_endpoints` / `vm_instances` テーブルが migrations にあり、`status` / `last_activity` / `idle_since` カラムを含む | `internal/db/migrations/005_create_vm_instances.sql`, `006_create_session_endpoints.sql`, `008_session_endpoints_unique_vm_port.sql` の存在 |
| AC-2 | `ExecutionProvider.Execute / GetSessionClient / PrepareForRelogin / PullCredentialsFromSession` がインターフェースとして固定されている | `internal/provider/provider.go` |
| AC-3 | 1 GCE VM 上で 2 セッション以上が異なるコンテナ・異なるポートで同時稼働できる | `internal/provider/dockergce/e2e_test.go::TestDockerGCEProvider_MultiContainerIntegration`（testcontainers + Mock GCE） |
| AC-4 | コンテナ起動時にポート競合を検出して 3 回までリトライする（DB の `UNIQUE(vm_instance_id, port)` ベース） | migration `008_session_endpoints_unique_vm_port.sql` + `DockerGCEProvider.allocatePort` のロジック |
| AC-5 | `GET /api/conversations/{id}/auth/*` が `executionProvider.GetSessionClient(convID)` 経由で per-session ルーティングされる | `internal/api/auth_handler.go`（refactor 後）+ `internal/api/session_auth_gate_e2e_test.go` |
| AC-6 | 起動時に孤児コンテナをクリーンアップする (`CleanupOrphans`) | `cmd/cc-tunnel/main.go` の `orphanCleaner` ブロック + `dockergce.DockerGCEProvider.CleanupOrphans` |
| AC-7 | アイドルコンテナ・アイドル VM を周期的にクリーンアップする goroutine が稼働する | `dockergce.IdleChecker` + `dockergce.VMScaler`（`NewDockerGCEProvider` 内で `IdleCheckInterval > 0` のとき自動起動） |
| AC-8 | ローカル開発で同等の per-session 動作を `EXECUTION_PROVIDER=local` で再現できる | `internal/provider/local.LocalDockerProvider` + `apps/compose.yaml` |

### 3. 現状マッピング

| AC | 状態 | 補足 |
|----|------|------|
| AC-1 | ✅ done | migrations 005/006/008 存在 |
| AC-2 | ✅ done | `provider.go` で固定済 |
| AC-3 | ✅ done | e2e は Docker daemon 必須のためローカル実行は条件付き |
| AC-4 | ✅ done | `MAX(port)+1` 戦略 + UNIQUE 制約。ADR `01_docker_gce_multi_container` 参照 |
| AC-5 | ✅ done | `cc_remote_agent_auth_retire` で完了 |
| AC-6 | ✅ done | `main.go:67-74` で provider を `orphanCleaner` にダウンキャストして呼び出し |
| AC-7 | ✅ done | `IdleChecker` / `VMScaler` 共に実装。CleanupOrphans が両者の主処理 |
| AC-8 | ✅ done | `local_docker_per_session` ADR 参照 |

すべての AC が満たされている。**Phase 1 は完了とみなす。**

### 4. design 文書との既知の乖離

`design/session-isolation.md` から実装が外れている点を以下に明示する。
今後 Phase 2 以降の議論ベースとして使う際は、この差分を頭に入れた上で参照すること。

| 項目 | design 文書の記述 | 実装 | 経緯 |
|---|---|---|---|
| Docker daemon 接続 | SSH トンネル経由 | TCP 2375 + VPC Connector からのみ許可するファイアウォール | `2026-04-29_docker_gce_multi_container` で COS 採用に伴い変更（COS は SSH バイナリなし） |
| SessionManager パッケージ | `internal/sessionmanager/` を新設 | `internal/provider/dockergce/`（および `internal/provider/local/`）に内包 | `ExecutionProvider` インターフェースが SessionManager の責務を包含するため |
| Phase 3 認証案 | 「案A: 認証専用コンテナ」を推奨 | 廃止（per-session container 内 PTY） | `cc_remote_agent_auth_retire` で全面切替。共有 `cc-remote-agent-auth` および `claude-sessions` volume は削除済 |

`design/session-isolation.md` 自体の本文書は今回は書き換えない（履歴ドキュメント
としての価値を残すため）。代わりに本 ADR をエントリポイントとして参照する。

### 5. Phase 2-4 の再ベースライン

Phase 1 と並行して Phase 2 / 3 の項目も大半が完了している。残作業を改めて
以下に固定する。これより先のフェーズ計画を検討する際は本表を起点とすること。

| 元 Phase | 項目 | 状態 | 残作業の有無 |
|---|---|---|---|
| 2 | handler.go の per-session routing | ✅ done | なし |
| 2 | main.go の SessionManager 初期化 | ✅ done | なし |
| 2 | IdleChecker + VMScaler goroutine | ✅ done | なし |
| 3 | 認証フロー再設計（per-session credentials） | ✅ done | なし |
| 3 | Secret Manager 統合（`CC_LOGIN_ENCRYPTION_KEY` + DATABASE_URL） | ✅ done | terraform 側に集約済 |
| 3 | ファイアウォールルール / VPC Connector | ✅ done | terraform 側に集約済 |
| 4 | Warm pool（最低 1 VM 常駐 or 0 VM 完全アイドル設定） | ❌ 未実装 | Phase 4 の主課題として残置 |
| 4 | Cloud Monitoring ダッシュボード / アラート（VM 数 / コンテナ数 / エラー率） | ❌ 未実装 | terraform 側に追加 |
| 4 | ローカル開発環境の整備 | ✅ done（`compose.yaml` + `prepare.compose.yaml`） | なし |

Phase 4（運用課題）のみが残置事項である。Phase 4 は本 ADR のスコープ外とする。

---

## 不採用案

### 案A: design 文書本体を書き換えて Phase 定義を更新する

`design/session-isolation.md` §11 を直接書き換え、現状を反映する案。

**不採用理由**:
- design 文書は「決定の根拠を残す履歴」として価値がある（Phase 分割が当初どう
  考えられていたかを後から追えること自体が ADR の役目）
- 書き換えると、過去の ADR が参照する記述と齟齬が生じる
- 代わりに ADR に集約することで、design は不変のまま、ADR が現状を述べる構造になる

### 案B: Phase 1 終了を宣言せず Phase 4 まで一括で扱う

「Warm pool まで揃って初めて完了」とする案。

**不採用理由**:
- Warm pool はコスト最適化（運用課題）であって、隔離性・スケーラビリティ・
  セキュリティの達成とは独立。Phase 1 の本来目的（セッション間の独立計算リ
  ソース）は既に達成されている
- 「クローズ済」と「運用課題」の境界が曖昧なままだと、次の優先順位付けが
  難しくなる

---

## 影響範囲

### 新規追加

| ファイル | 内容 |
|---|---|
| `adr/2026-05/2026-05-01T00:00:00+09:00_01_session_isolation_phase1_close.md` | 本 ADR |

### 書き換えなし

- `design/session-isolation.md`: 履歴として保持
- 既存 ADR 群: いずれも書き換えない

---

## 今後

- Phase 4（Warm pool / 監視 / アラート）は別 ADR で着手判断する。
- Phase 4 着手時には、本 ADR §5 の表が「現状の正」として参照される前提で計画する。
- 新たに session-isolation 関連の判断を下す ADR を作る場合は、本 ADR を必ず
  Related ADR として引用する。
