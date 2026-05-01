# Report: subtask_frontend_openresty_id_token_001

実行者: ashigaru3
タスクID: subtask_frontend_openresty_id_token_001
親コマンド: cmd_cctunnel_terraform_frontend_backend_url_fix_001
実行日時: 2026-04-28T02:37:19+00:00

## 変更内容

### apps/frontend/Dockerfile

ランタイムステージのベースイメージを nginx → OpenResty に変更。

```diff
-FROM mirror.gcr.io/library/nginx:stable-alpine
+FROM mirror.gcr.io/openresty/openresty:alpine-fat
```

`openresty:alpine-fat` は `lua-resty-http` が標準同梱済み。

### apps/frontend/nginx.conf.template

#### main ブロック（events 前）: env 宣言追加

```nginx
env CC_TUNNEL_AUDIENCE;
```

#### http ブロック: Lua 設定追加

```nginx
lua_shared_dict id_tokens 1m;
lua_ssl_trusted_certificate /etc/ssl/certs/ca-certificates.crt;
lua_ssl_verify_depth 5;
```

#### /api/ location + SSE location: access_by_lua_block 追加

両 location に同一の Lua ブロックを追加。proxy_pass の前に実行される。

## Lua ブロックの動作説明

1. `ngx.shared.id_tokens` (shared dict) からキャッシュを確認
2. キャッシュミス時: GCE メタデータサーバ (`metadata.google.internal`) に ID token をリクエスト
   - URL: `/computeMetadata/v1/instance/service-accounts/default/identity?audience=<CC_TUNNEL_AUDIENCE>`
   - ヘッダー: `Metadata-Flavor: Google`
3. 取得成功: cache に保存（TTL 3000秒 = 50分）
4. `Authorization: Bearer <token>` ヘッダーを upstream リクエストに注入
5. エラー時: CC_TUNNEL_AUDIENCE 未設定 → 500、メタデータ取得失敗 → 502

## docker build 結果

```
SUCCESS
- stage-1 (OpenResty): mirror.gcr.io/openresty/openresty:alpine-fat
- 全 COPY ステップ完了
- イメージ sha256:ed9f5404... 生成
```

## ローカル制約

メタデータサーバ (`metadata.google.internal`) はローカル環境では到達不可。
`/api/` アクセス時に 502 が返るのは想定動作。Cloud Run 上でのみ正常動作する。

## CRLF 確認

- Dockerfile: LF only OK
- nginx.conf.template: LF only OK

## git 操作

一切なし（禁止ルール遵守）。

## 殿の apply + 動作確認手順

1. `cd ~/ghq/github.com/pollenjp/cc-tunnel`
2. `git add apps/frontend/Dockerfile apps/frontend/nginx.conf.template`
3. `git commit -m "feat: switch frontend to OpenResty+Lua for ID token injection"`
4. Cloud Run に deploy し、`CC_TUNNEL_AUDIENCE` 環境変数に backend Cloud Run の URL を設定
5. `curl -H "Authorization: Bearer ..."` で `/api/` へのリクエストが通ることを確認
   （実際は Lua が自動で Authorization ヘッダーを注入するため、frontend → backend 間の認証が通る）
