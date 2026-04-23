# apps/openapi

cc-tunnel の API 定義。OpenAPI 3.0 仕様で記述されている。

## ファイル

| ファイル | 説明 |
|---------|------|
| `openapi.yaml` | 外部 API 定義 (Server B: cc-tunnel) |
| `oapi-codegen.yaml` | 外部 API サーバーコード生成設定 |

## 外部 API エンドポイント (Server B)

| メソッド | パス | 説明 |
|---------|------|------|
| `POST` | `/sessions` | セッション作成 (claude_code / multi_agent_shogun) |
| `GET` | `/sessions` | セッション一覧 |
| `POST` | `/sessions/{sessionId}/input` | 入力送信 (paneIndex 指定可) |
| `GET` | `/sessions/{sessionId}/output` | ペイン出力取得 (paneIndex 指定可) |
| `GET` | `/sessions/{sessionId}/outputs` | 全ペイン出力の一括取得 |
| `POST` | `/sessions/{sessionId}/resize` | ウィンドウリサイズ |
| `DELETE` | `/sessions/{sessionId}` | セッション終了 |

内部 API (Server A) も同一のエンドポイント構成。

## セッションタイプ

- `claude_code` — Claude Code CLI による単一会話セッション
- `multi_agent_shogun` — shogun + multiagent の 2 セッション構成 (ペイン 10 個)

## Go コード生成

`oapi-codegen` を使って Go のサーバーインターフェース、モデル型、HTTP クライアントを生成する。

```bash
# 外部 API サーバー
cd apps/cc-tunnel && go generate ./internal/api/
```

### API 定義の変更フロー

1. `openapi.yaml` を編集
2. `go generate` で対応する `gen.go` を再生成
3. `handler.go` の実装を新しいインターフェースに合わせて更新
4. `go build ./...` でビルド確認
