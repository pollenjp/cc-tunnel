# cmd_cctunnel_provider_tdd 変更ログ

## 1. 経緯・背景

- cmd_cctunnel_provider_impl で Provider パターンの基本構造を実装済み
- TDD アプローチでテストを充実させ、品質保証を行う
- provider_impl QC（軍師）: PASS（全 acceptance_criteria 充足、go build/test 成功）

### provider_impl で実装済みのファイル

| ファイル | 概要 |
|---------|------|
| `internal/provider/provider.go` | `ExecutionProvider` インターフェース定義 |
| `internal/provider/local/local.go` | local provider（`remoteclient.Client` ラッパー） |
| `internal/provider/cloudrunsandbox/mock.go` | Cloud Run Sandbox mock provider |
| `internal/provider/dockergce/mock.go` | Docker on GCE mock provider |
| `internal/api/handler.go` | `Server` に `executionProvider` フィールド追加 |
| `internal/api/interfaces.go` | `remoteClient` から `Execute` を削除 |
| `apps/openapi/openapi.yaml` | `execution_mode` フィールド追加 |
| `cmd/cc-tunnel/main.go` | `EXECUTION_PROVIDER` 環境変数による provider 選択 |

## 2. TDD 方針（twada 式）

- テスト先行（RED → GREEN → Refactor）
- 各 provider の interface 適合を型検証テストで保証
- mock providers の固定レスポンスを単体テストで検証
- local provider の Execute 委譲を integration 的テストで検証

### インターフェース定義（確認済み）

```go
// internal/provider/provider.go
type ExecutionProvider interface {
    Execute(ctx context.Context, req remoteclient.Request, onEvent func(remoteclient.StreamEvent)) (string, error)
}
```

## 3. 追加テスト一覧

| ファイル | 概要 |
|---------|------|
| `internal/provider/provider_test.go` | `ExecutionProvider` コンパイル時型検証 |
| `internal/provider/local/local_test.go` | `Execute` 委譲テスト |
| `internal/provider/cloudrunsandbox/mock_test.go` | 固定レスポンス検証 |
| `internal/provider/dockergce/mock_test.go` | 固定レスポンス検証 |

## 4. 設計上のポイント

### 型検証テスト

コンパイル時にインターフェース適合を保証するパターンを使用:

```go
var _ provider.ExecutionProvider = (*local.Provider)(nil)
var _ provider.ExecutionProvider = (*cloudrunsandbox.MockProvider)(nil)
var _ provider.ExecutionProvider = (*dockergce.MockProvider)(nil)
```

### local provider の委譲テスト

- `httptest.NewServer` を使用してモック HTTP サーバーを立ち上げ
- `remoteclient.Client` 経由の委譲が正しく行われることを検証
- コンテキストキャンセルや onEvent コールバックの呼び出しも確認

### mock テストの検証内容

- `onEvent` コールバックの呼び出し順序と内容を検証
  1. `type: "assistant"` イベント（テキストコンテンツを含む）
  2. `type: "result"` イベント（result: "success"）
- 戻り値の session ID が `"mock-session-"` プレフィックスを持つことを確認
- `uuid.New()` によるユニーク性を確認

## 5. 品質確認

- mise run check（全テスト + lint）: 実行後に更新予定

> ※ 完全な結果は足軽1号・2号の報告を受けてから更新すること（現時点は計画段階）。
