# cmd_cctunnel_provider_impl 変更ログ

**コマンドID**: cmd_cctunnel_provider_impl
**作業日**: 2026-04-23
**記録者**: 足軽2号 (Ashigaru2)

---

## 経緯・背景

### 前コマンドからの流れ

前コマンド（cmd_cctunnel_docker_gce_design / cmd_cctunnel_sandbox_option_design）において、cc-tunnel の実行基盤として以下の3方式の詳細設計書を作成した:

- **local**: 現在の実装（同一 Docker Compose 内の cc-remote-agent を HTTP で呼ぶ方式）
- **cloud_run_sandbox**: Cloud Run Sandbox を使う方式
- **docker_gce**: GCE 上の Docker コンテナを使う方式

本コマンド（cmd_cctunnel_provider_impl）はその設計をもとに、**Provider パターンによる実行基盤の抽象化**を実装フェーズとして実施する。

### 実装の目的

- cc-remote-agent の実行基盤を `ExecutionProvider` インターフェースで抽象化
- 現在の単一インスタンス方式を `local` provider として整理
- `cloud_run_sandbox` と `docker_gce` は mock provider として実装し、将来の本実装への土台を構築

---

## 設計上の判断ポイント（予定）

### ExecutionProvider インターフェースの設計

Go の interface として `ExecutionProvider` を定義する。設計書（`docs/` 以下の SessionProvider 設計）を参考に、以下の操作を含める:

- **セッション生成** (`CreateSession`): プロバイダ側でセッションに対応するリソースを確保
- **メッセージ実行** (`Execute` 相当): セッションに対してメッセージを送信し、レスポンスを受け取る
- **セッション破棄** (`DeleteSession`): セッションに紐づくリソースを解放

認証（auth）と実行（execute）の責務を分離し、各 provider が独立してライフサイクルを管理できる設計とする。

### handler.go の変更最小化

既存のテストを維持するため、`handler.go` への変更は最小限に抑える。
`Server` 構造体の `remote` フィールドを `ExecutionProvider` インターフェース経由で呼び出すように変更する。
既存の `remoteclient.Client` の動作は `local` provider に移植し、インターフェースを実装させる。

### openapi.yaml への execution_mode 追加とコード再生成

会話作成時またはメッセージ送信時に provider を選択できるように、`openapi.yaml` に `execution_mode` パラメータを追加する。
変更後は `oapi-codegen` でコードを再生成し、`gen.go` を更新する。

### mock provider のレスポンス形式

`cloud_run_sandbox` と `docker_gce` の mock provider は、最小限の固定レスポンスを返す実装とする。例:
- `"This is a mock response from cloud_run_sandbox provider"`
- `"This is a mock response from docker_gce provider"`

実際の外部サービス呼び出しは行わず、インターフェースを満たすスタブ実装とする。

---

## 変更点一覧（予定）

### 新規追加ファイル

| ファイル | 内容 |
|----------|------|
| `internal/provider/provider.go` | `ExecutionProvider` インターフェース定義 |
| `internal/provider/local/local.go` | local provider 実装（現行の cc-remote-agent HTTP 呼び出しを移植） |
| `internal/provider/cloudrunsandbox/cloudrunsandbox.go` | Cloud Run Sandbox mock provider 実装 |
| `internal/provider/dockergce/dockergce.go` | Docker on GCE mock provider 実装 |

### 変更予定の既存ファイル

| ファイル | 変更内容 |
|----------|----------|
| `internal/api/handler.go` | `remote` フィールドを `ExecutionProvider` インターフェース経由に変更 |
| `internal/api/interfaces.go` | `ExecutionProvider` インターフェースの参照を追加 |
| `cmd/cc-tunnel/main.go` | provider 初期化・選択ロジックの追加、デフォルト = local |
| `openapi.yaml` | `execution_mode` パラメータの追加 |
| `internal/api/gen.go` | `oapi-codegen` 再生成による自動更新 |

### ドキュメント

| ファイル | 変更内容 |
|----------|----------|
| `docs/architecture.md` | provider アーキテクチャの反映・加筆修正（別タスクで対応済み） |

---

## 注意事項

| 項目 | 詳細 |
|------|------|
| **後方互換** | `local` provider は現行の cc-remote-agent HTTP 呼び出しと完全に同じ動作が必須。既存の動作を壊さないこと。 |
| **ビルド確認** | 実装後は `go build ./...` が成功することを確認すること。 |
| **テスト維持** | 既存のテストが通ること。`handler.go` の変更はインターフェース経由への置き換えに留める。 |
| **改行コード** | 全ファイル LF 改行（CRLF 厳禁） |
| **デフォルト** | `execution_mode` 未指定時は `local` provider を使用（既存動作の後方互換） |

---

## 参照資料

- `docs/docker-gce-design.md` — Docker on GCE 詳細設計書
- `docs/cloud-run-sandbox-design.md` — Cloud Run Sandbox 詳細設計書
- `design/session-isolation.md` — 3案比較・推奨根拠
- `internal/api/handler.go` — 現行の handler 実装（変更対象）
- `internal/api/interfaces.go` — 現行のインターフェース定義（変更対象）
- `internal/remoteclient/client.go` — 現行の cc-remote-agent クライアント（local provider への移植元）
