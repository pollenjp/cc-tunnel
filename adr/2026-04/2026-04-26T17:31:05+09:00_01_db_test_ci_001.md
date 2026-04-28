# 変更ログ: subtask_db_test_ci_impl_001

## 変更ファイル

- `.github/workflows/ci.yml`

## 変更内容

`test-cc-tunnel` ジョブに以下を追加:

- **services**: `postgres:18-alpine` コンテナを起動し、ヘルスチェック付きで 5432 ポートを公開
- **env**: `DATABASE_URL=postgres://cctunnel:cctunnel_dev@localhost:5432/cctunnel?sslmode=disable`

既存の `steps`（actions/checkout, actions/setup-go, go test）および `defaults.run.working-directory` は変更なし。

## 根拠

軍師設計レビュー（gunshi_db_test_ci_design）DD001-DD006 に基づく推奨案 B（GHA services）の実装。
`repository_gce_test.go` の `testDatabaseURL()` は `DATABASE_URL` 環境変数を優先取得するため、
CI での services 追加のみで 4 テストが PASS するようになる。

## 品質確認

- YAML 構文: `python3 -c "import yaml; yaml.safe_load(...)"` → OK
- 改行コード: LF only（CRLF なし）
- Go コード変更: なし（go.mod / go.sum / *.go すべて手付かず）

## ローカル開発への影響

なし。`mise run dev:up` + `mise run test:cc-tunnel` はそのまま動作する。
（CI では postgres が自動起動、ローカルでは `dev:up` で postgres を手動起動する必要がある）
