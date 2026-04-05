# apps/openapi

cc-tunnel の API 定義。OpenAPI 3.0 仕様で記述されている。

## ファイル

| ファイル | 説明 |
|---------|------|
| `openapi.yaml` | OpenAPI 定義 (Single Source of Truth) |
| `oapi-codegen.yaml` | oapi-codegen の設定ファイル |

## エンドポイント一覧

| メソッド | パス | 説明 |
|---------|------|------|
| `POST` | `/sessions` | セッション作成 + Claude Code 起動 |
| `GET` | `/sessions` | セッション一覧 |
| `POST` | `/sessions/{sessionId}/input` | テキスト入力送信 |
| `GET` | `/sessions/{sessionId}/output` | 画面出力取得 |
| `DELETE` | `/sessions/{sessionId}` | セッション終了 |

## Go コード生成

`oapi-codegen` を使って Go のサーバーインターフェースとモデル型を生成する。

### oapi-codegen のインストール

```bash
go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
```

### 設定ファイルを使う場合

```bash
oapi-codegen -config apps/openapi/oapi-codegen.yaml apps/openapi/openapi.yaml
```

### go generate を使う場合

```bash
cd apps/cc-tunnel
go generate ./internal/api/
```

### API 定義の変更フロー

1. `openapi.yaml` を編集
2. `go generate` (または `oapi-codegen` 直接実行) で `gen.go` を再生成
3. `handler.go` の実装を新しいインターフェースに合わせて更新
4. `go build ./...` でビルド確認
