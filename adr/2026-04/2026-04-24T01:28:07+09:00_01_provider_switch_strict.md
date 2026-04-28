# cmd_cctunnel_provider_switch_strict 変更ログ

## 1. 経緯・背景

- cmd_cctunnel_provider_tdd（TDD テスト充実）完了後、更なる品質向上として実施
- 現状: EXECUTION_PROVIDER 未設定時に default で local が選択される（暗黙的）
- 問題: 本番環境で環境変数を忘れると意図せず local が使われる可能性
- 目的: 明示的な設定を強制し、設定ミスを実行時に即座に検知

## 2. 変更内容

- default → log.Fatal/os.Exit によるエラー終了
- case "local" を明示的に追加
- agentURL（remoteclient.NewClient）は case "local" 内のみで使用
- provider 選択ロジックを newProviderFromEnv() 関数に切り出し（テスタビリティ向上）

### 変更前

```go
remote := remoteclient.NewClient(*agentURL)

// Select execution provider via EXECUTION_PROVIDER env var (default: local).
var execProvider provider.ExecutionProvider
switch os.Getenv("EXECUTION_PROVIDER") {
case "cloud_run_sandbox":
    execProvider = cloudrunsandbox.New()
case "docker_gce":
    execProvider = dockergce.New()
default:
    execProvider = localprovider.New(remote)
}
```

### 変更後

```go
// provider 選択ロジックを関数に切り出し（テスタビリティ向上）
func newProviderFromEnv(envVal, agentURL string) (provider.ExecutionProvider, error) {
    switch envVal {
    case "local":
        // agentURL を使うのはここだけ
        remote := remoteclient.NewClient(agentURL)
        return localprovider.New(remote), nil
    case "cloud_run_sandbox":
        return cloudrunsandbox.New(), nil
    case "docker_gce":
        return dockergce.New(), nil
    default:
        return nil, fmt.Errorf("unknown EXECUTION_PROVIDER: %q", envVal)
    }
}
```

main() 内:

```go
execProvider, err := newProviderFromEnv(os.Getenv("EXECUTION_PROVIDER"), *agentURL)
if err != nil {
    slog.Error("unknown EXECUTION_PROVIDER", "value", os.Getenv("EXECUTION_PROVIDER"))
    os.Exit(1)
}
```

## 3. テスト追加

- main_test.go: TestNewProviderFromEnv_xxx（local/cloud_run_sandbox/docker_gce/unknown/empty）

| テストケース | envVal | agentURL | 期待結果 |
|---|---|---|---|
| TestNewProviderFromEnv_local | "local" | "http://localhost:9091" | *local.Provider（エラーなし） |
| TestNewProviderFromEnv_cloudRunSandbox | "cloud_run_sandbox" | "" | *cloudrunsandbox.MockProvider（エラーなし） |
| TestNewProviderFromEnv_dockerGce | "docker_gce" | "" | *dockergce.MockProvider（エラーなし） |
| TestNewProviderFromEnv_empty | "" | "" | error |
| TestNewProviderFromEnv_unknown | "unknown" | "" | error |

## 4. 設計ポイント

- agentURL は local provider のみが必要。他の provider では使用しない
- エラーメッセージに EXECUTION_PROVIDER の値を含め、デバッグしやすく
- newProviderFromEnv() 関数に切り出すことで main() のテストが困難な問題を回避
- 本番運用では EXECUTION_PROVIDER の明示的設定が必須となる

## 5. 品質確認

- mise run check: PASS / FAIL（実行後更新）
