# cmd_cctunnel_sandbox_option_design 変更ログ

**コマンドID**: cmd_cctunnel_sandbox_option_design
**作業日**: 2026-04-23
**記録者**: 足軽4号 (Ashigaru4)

---

## 経緯・背景

### 前 cmd との関係

前コマンド（cmd_cctunnel_docker_gce_design）にて、cc-tunnel のセッション隔離方式として
**案1（Docker on GCE）** の詳細設計が完了した。成果物として以下が作成済み:

- `docs/docker-gce-design.md` — Docker on GCE 詳細設計書
- `docs/plantuml/docker_gce_*.puml` — アーキテクチャ図・シーケンス図一式
- `docs/architecture.md` — Docker on GCE セクション追記済み

### 殿の直命

Docker on GCE 設計の完了後、殿より追加の直命が下された:

> GoogleCloudPlatform/cloud-run-sandbox を活用した第2の隔離方式も設計し、
> ユーザーが用途に応じて選択できるようにせよ。

これが本コマンド（cmd_cctunnel_sandbox_option_design）の起点である。

### cloud-run-sandbox の主要特徴

`~/ghq/github.com/GoogleCloudPlatform/cloud-run-sandbox/` に配置された OSS プロジェクト。
以下の特徴を cc-tunnel に活用することが期待されている:

| 機能 | 概要 |
|------|------|
| **gVisor 隔離** | Cloud Run 上で `runsc` (gVisor) ベースのセキュアなサンドボックスを提供 |
| **WebSocket API** | WebSocket 経由でコマンド実行 (`exec`) が可能 |
| **Checkpoint/Restore** | サンドボックスの完全な状態（メモリ+ファイルシステム）を GCS に保存し、別インスタンスで復元可能 |
| **Filesystem Snapshot** | サンドボックスのファイルシステムスナップショットを作成し、そこから新しいサンドボックスを高速に生成可能 |
| **セッション単位の隔離** | `--concurrency=1 --session-affinity` でセッション単位の隔離を実現 |
| **クライアントライブラリ** | Python / TypeScript クライアントライブラリあり |

---

## 設計上の重要な判断ポイント（予定）

### Docker on GCE vs Cloud Run Sandbox の適用シナリオ分け

2方式のトレードオフに基づき、用途に応じた推奨シナリオを設計予定:

| シナリオ | 推奨方式 |
|----------|----------|
| 重い作業・長時間セッション・カスタム環境 | Docker on GCE |
| 軽量・高速起動・短時間セッション・高セキュリティ | Cloud Run Sandbox |

### SessionProvider 共通インターフェースによる切り替え設計

既存の Docker on GCE 設計との整合性を保つため、共通の `SessionProvider` インターフェースを
設け、実装を差し替え可能にする設計を予定。cc-tunnel の handler.go / main.go への影響を
最小化しつつ、両方式に対応できるアーキテクチャとする。

### checkpoint/restore の cc-tunnel への統合方法

cloud-run-sandbox の checkpoint/restore 機能を活用し、長時間セッションの状態を
GCS に保存・復元できる設計を検討予定。cc-remote-agent プロセスを含めた
完全な状態復元が可能かどうかは、軍師による実ソース調査で確認予定。

### execution_mode パラメータの API 設計

会話作成 API（`POST /conversations`）に `execution_mode` パラメータを追加し、
ユーザーが `docker_gce` / `cloud_run_sandbox` を選択できる設計を予定。
デフォルト値の選定（コスト・UX の観点）も設計書で明示する。

---

## 変更点一覧（予定）

### 新規作成ファイル（予定）

| ファイル | 内容 |
|----------|------|
| `docs/cloud-run-sandbox-design.md` | Cloud Run Sandbox 方式の詳細設計書 |
| `docs/plantuml/sandbox_architecture.puml` | Cloud Run Sandbox 方式の全体アーキテクチャ図 |
| `docs/plantuml/sandbox_session_start.puml` | セッション開始〜Sandbox 生成〜メッセージ処理のシーケンス図 |
| `docs/plantuml/sandbox_idle_cleanup.puml` | Idle 検知〜Sandbox 破棄のシーケンス図 |
| `docs/plantuml/sandbox_selection_flow.puml` | Docker on GCE 方式と Cloud Run Sandbox 方式の選択フロー図 |

### 既存修正ファイル（予定）

| ファイル | 変更内容 |
|----------|----------|
| `docs/architecture.md` | Cloud Run Sandbox オプションの追記（選択可能なアーキテクチャとして概説） |

### 本変更ログ（確定）

| ファイル | 内容 |
|----------|------|
| `logs/2026-04-23T13:41:57+00:00_cmd_cctunnel_sandbox_option_design/report.md` | 本ファイル（経緯・思考・変更点・ポイントを記録） |

---

## 注意事項・リスク

| リスク | 影響度 | 対策・備考 |
|--------|--------|------------|
| **cloud-run-sandbox の WebSocket API を Go から呼び出す実装** | 高 | Python/TypeScript ライブラリはあるが Go 向けは未確認。軍師が実ソース確認後に設計。 |
| **Cloud Run の `--concurrency=1` + `--session-affinity` 設定** | 高 | セッション単位の隔離に必須。Cloud Run の仕様制限（最大インスタンス数等）を確認が必要。 |
| **checkpoint/restore は GCS バケットへのアクセス権限が必要** | 中 | IAM 設定・サービスアカウント設計が必須。運用コスト（GCS ストレージ費用）も見積もり必要。 |
| **gVisor (runsc) の互換性** | 中 | cc-remote-agent が使用する linux システムコールが gVisor でサポートされているか確認が必要。 |
| **設計内容は軍師の実ソース調査結果に依存** | — | 本ファイルは「予定」「期待」として記述。軍師の設計書完成後に実績値で更新すること。 |

---

## 参照資料

- `~/ghq/github.com/GoogleCloudPlatform/cloud-run-sandbox/` — OSS プロジェクト本体
- `design/session-isolation.md` — 3案比較・推奨根拠（案1採用の根拠文書）
- `docs/docker-gce-design.md` — 前 cmd の Docker on GCE 詳細設計書（本 cmd の比較対象）
- `queue/shogun_to_karo.yaml` 内 `cmd_cctunnel_sandbox_option_design` セクション — 殿の直命・acceptance_criteria
