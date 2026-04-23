# cmd_cctunnel_docker_gce_design 変更ログ

**コマンドID**: cmd_cctunnel_docker_gce_design
**作業日**: 2026-04-23
**記録者**: 足軽4号 (Ashigaru4)

---

## 経緯・背景

### 現行の問題点

cc-remote-agent は単一コンテナとして稼働し、全会話セッションが同一プロセス空間を共有していた。これにより以下の問題が存在していた:

- **隔離性なし**: あるセッションの `claude` CLI 実行が他セッションに影響しうる（リソース競合、プロセス干渉）
- **スケーラビリティ制約**: 単一インスタンスの CPU/メモリが上限
- **セキュリティ**: セッション間でファイルシステムを共有（`/home/user/.claude`）

また、現行コードにも以下の制約があった:

| コンポーネント | 現行実装 | セッション隔離への影響 |
|---|---|---|
| `cc-tunnel/internal/remoteclient/client.go` | `Client.baseURL` が固定1つの cc-remote-agent URL | **変更必須**: セッションごとに異なるエンドポイントへのルーティングが必要 |
| `cc-tunnel/cmd/cc-tunnel/main.go` | `-agent-url` フラグで単一 URL を指定 | **変更必須**: セッションマネージャに置き換え |
| `cc-tunnel/internal/api/handler.go` | `Server.remote` が単一 `remoteClient` | **変更必須**: per-session client routing |

### 将軍の直命として案1（Docker on GCE）を正式採用した経緯

セッション隔離の実現方式として3案を比較検討した結果、将軍の直命により**案1（Docker on GCE）**が正式採用された。

- **前回の成果物との関係**: `design/session-isolation.md` において3案比較・推奨案の詳細アーキテクチャ・実装フェーズが文書化された。本コマンド（cmd_cctunnel_docker_gce_design）はその設計内容を詳細設計書・PlantUML図・アーキテクチャ文書として具現化することを目的とした。

---

## 設計上の重要な判断ポイント

### 3案比較の要点

| 評価軸 | 案1: Docker on GCE | 案2: GCE per session | 案3: GKE Autopilot |
|--------|:---:|:---:|:---:|
| **起動速度** | **1-3秒** (VM稼働時) | 30-60秒 | 5-120秒 |
| **コスト (10並行)** | **$1.61/day** | $8.16/day | $7.68/day |
| **隔離性** | コンテナ (十分) | **VM (最強)** | コンテナ (十分) |
| **運用複雑度** | 中 | **低〜中** | 高 |
| **アイドルコスト** | **$0** (VM削除) | **$0** (VM削除) | $72/月 (クラスタ管理料) |

**案1選択の根拠**: 起動速度が圧倒的に速く（VM稼働中はコンテナ起動 1-3秒）、コスト効率が最も高く（複数セッションを1 VM に集約）、隔離性は実用上十分（claude CLI は Anthropic 公式ツールで悪意あるコード実行リスクが低い）。

### Docker API アクセス方式（SSH トンネル vs TLS の選択根拠）

GCE VM 上の Docker デーモンへのアクセスには **SSH トンネル経由の Docker API** を採用した。

| 方式 | 採用 | 根拠 |
|------|------|------|
| Docker API over TLS (tcp:2376) | × | TLS 証明書管理が複雑 |
| SSH トンネル | **○** | 既存の SSH 認証を活用、追加設定最小 |
| gcloud compute ssh | △ | フォールバック用。gcloud CLI 依存のため主用途には不向き |

実装方式: `docker -H ssh://user@<vm-internal-ip>` 形式、または Go の `docker/client` ライブラリに SSH ダイアラーを設定。

### Warm Pool 戦略（e2-micro 1台 vs 完全ゼロのトレードオフ）

| 設定 | 値 | トレードオフ |
|---|---|---|
| `warm_pool_size = 0` (デフォルト) | 完全コスト最適化 | 初回の VM 起動で 30-60秒の待ちが発生 |
| `warm_pool_size = 1` (推奨) | e2-micro $5.76/月を常時待機 | 初回レイテンシを解消。コスト微増で UX 大幅改善 |

**推奨設定**: warm_pool_size = 1（e2-micro 1台常時待機）。月額 $5.76 の追加コストで初回コールドスタート問題を解消できる。

### 認証フロー（認証専用コンテナ案A vs APIキー案B の選択）

セッションごとに動的生成されるコンテナでは認証フローの再設計が必要であった。

| 方式 | 概要 | 採用 |
|------|------|------|
| **案A: 認証専用コンテナ** | 永続的な認証専用コンテナを1つ維持し、credentials を Secret Manager に保存。セッションコンテナ起動時にマウント。 | **○ 推奨** |
| **案B: API キー認証に統一** | Anthropic API Key を直接使用（OAuth フロー不要）。ユーザーが API Key を持っている前提。 | △ |

**採用: 案A** — 既存の OAuth フローを活かしつつ、credentials を共有する方式を推奨。

### IdleChecker / VMScaler の間隔設定根拠

```
SessionManager (cc-tunnel 内 goroutine)
  ├── IdleChecker (60秒間隔)  ← コンテナのアイドル検出・削除
  └── VMScaler (5分間隔)      ← VM の削除（全コンテナ削除後 5分で判定）
```

- **IdleChecker 60秒**: コンテナの idle 15分を超過したものを検出。1分間隔で十分な精度（最大 1分の誤差）を確保しつつ、API コール頻度を抑制。
- **VMScaler 5分**: コンテナがゼロになっても短時間で再利用される可能性（`vm_idle_grace_period = 5分`）を考慮し、5分後に VM 削除を判断。

---

## 変更点一覧

### 新規作成ファイル

| ファイル | 内容 |
|----------|------|
| `docs/docker-gce-design.md` | Docker on GCE 詳細設計書（SessionManager 設計・DB スキーマ・通信経路・フェーズ計画を含む） |
| `docs/plantuml/docker_gce_architecture.puml` | C4 Level 2 コンテナ図（Cloud Run・GCE VM・Docker コンテナ・PostgreSQL の関係） |
| `docs/plantuml/docker_gce_session_start.puml` | セッション開始シーケンス図（VM warm/cold 両ケース） |
| `docs/plantuml/docker_gce_idle_cleanup.puml` | アイドル 15分でのリソース自動削除フロー図 |
| `docs/plantuml/docker_gce_component.puml` | SessionManager 内部コンポーネント図 |

### 既存修正ファイル

| ファイル | 変更内容 |
|----------|----------|
| `docs/architecture.md` | Docker on GCE セクションを追記（案1採用の根拠・アーキテクチャ概要） |

### 将来の実装フェーズ（設計書に記載）

| フェーズ | 内容 | 工数目安 |
|----------|------|----------|
| **Phase 1: 基盤** | DB マイグレーション・SessionManager 基本実装・GCE VM プロビジョニング・Docker コンテナ管理 | 1-2週間 |
| **Phase 2: 統合** | handler.go 改修（per-session routing）・main.go 改修・IdleChecker/VMScaler goroutine | 1週間 |
| **Phase 3: 認証・セキュリティ** | 認証フロー再設計・Secret Manager 統合・ファイアウォールルール・VPC Connector 構成 | 1週間 |
| **Phase 4: 運用** | Warm pool 実装・Cloud Monitoring ダッシュボード・アラート設定・ローカル開発環境整備 | 1週間 |

---

## 注意事項・リスク

| リスク | 影響度 | 対策 |
|--------|--------|------|
| **VM コールドスタート（初回 30-60秒）** | 高（UX に直接影響） | Warm pool: e2-micro 1台を常時待機（$5.76/月）。長時間アイドル後の初回会話のみ遅延発生。 |
| **GCE クォータ制限（デフォルト 24 CPUs/リージョン）** | 中 | e2-standard-2（2 vCPU）であれば最大 12 VM まで。同時 100+ セッション規模になった時点でクォータ申請が必要。 |
| **Docker デーモン障害時の全セッション停止リスク** | 高（単一障害点） | VM ヘルスチェック + 自動再作成。複数 VM による冗長化（VMScaler が VM を複数管理）。 |
| **cc-remote-agent は変更不要** | — | コンテナイメージとしてそのまま再利用可能。`apps/cc-remote-agent/` 全体は変更対象外。 |
| **ポート枯渇（動的ポートマッピング 9091-9200）** | 低〜中 | 1 VM あたり最大 110 セッション。上限到達時に新 VM 作成ロジックで対応。 |

---

## 参照資料

- `design/session-isolation.md` — 3案比較・推奨根拠・詳細アーキテクチャ（本変更の根拠文書）
- `docs/docker-gce-design.md` — Docker on GCE 詳細設計書（本コマンドで新規作成）
