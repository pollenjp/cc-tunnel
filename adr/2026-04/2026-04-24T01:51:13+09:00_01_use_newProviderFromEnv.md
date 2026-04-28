# cmd_cctunnel_use_newProviderFromEnv 変更ログ

## 1. 経緯・背景

- 前 cmd（provider_switch_strict）で `newProviderFromEnv` を定義したが、`main()` では使わず `switch` 文を直接書く重複状態だった
- 殿の直接指摘により即刻修正
- 重複ロジックの排除と保守性向上が目的

## 2. 変更内容

- `newProviderFromEnv` のシグネチャ拡張:
  - 旧: `(provider.ExecutionProvider, error)`
  - 新: `(provider.ExecutionProvider, *remoteclient.Client, error)`
- `main()` の `switch` 文を削除し、`newProviderFromEnv()` 呼び出しに置き換え
- `main_test.go`: `remote` 戻り値の検証を追加（nil/non-nil チェック）

## 3. 設計ポイント

- `remote` は `local` プロバイダ時のみ non-nil（他プロバイダは nil）
- `main()` がシンプルになり、provider 選択ロジックの唯一の実装が `newProviderFromEnv` に集約
- `api.NewHandler(repo, remote, execProvider)` に `remote` をそのまま渡せる

## 4. 品質確認

- mise run check: PASS
