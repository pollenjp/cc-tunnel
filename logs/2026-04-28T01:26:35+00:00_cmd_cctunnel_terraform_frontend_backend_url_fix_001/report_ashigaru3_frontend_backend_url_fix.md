# 足軽3号 report: subtask_terraform_frontend_backend_url_fix_001

## 現状実装確認

### 1. frontend.tf (Terraform)
`terraform/modules/cc-tunnel/frontend.tf` の `fe_cloud_run` リソースに以下 env が既存コミット済み:

```hcl
env {
  name  = "API_UPSTREAM"
  value = google_cloud_run_v2_service.cloud_run.uri
}
env {
  name  = "BACKEND_URL"
  value = "/api"
}
```

- `API_UPSTREAM`: cc-tunnel Cloud Run URI を参照（片方向依存、circular dependency なし）
- `BACKEND_URL`: "/api" — フロントエンドコード (`src/api/client.ts`) が `window.__ENV__?.BACKEND_URL ?? '/api'` を使用するため維持必須

### 2. nginx.conf.template の /api/ reverse proxy
`apps/frontend/nginx.conf.template` に /api/ location ブロックが実装済み（コミット済み）:

```nginx
# SSE ストリーミング専用（conversations/{id}/messages）
location ~ ^/api/conversations/[^/]+/messages {
    rewrite ^/api(.*)$ $1 break;
    proxy_pass $API_UPSTREAM;
    proxy_http_version 1.1;
    proxy_buffering off;
    proxy_cache off;
    ...
    proxy_read_timeout 300s;
}

# 通常 API リクエスト
location /api/ {
    proxy_pass $API_UPSTREAM/;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

### 3. docker-entrypoint.sh（envsubst テンプレート方式）
Dockerfile:
- `nginx.conf.template` → `/etc/nginx/templates/nginx.conf.template`（nginx envsubst で展開）
- `docker-entrypoint.sh` → env.js 生成 → nginx entrypoint 委譲

採用パターン: **パターンA（envsubst テンプレート方式）**
- 既存 Dockerfile が `nginx.conf.template` + `docker-entrypoint.sh` を使用済み
- `ENV NGINX_ENVSUBST_OUTPUT_DIR=/etc/nginx` で `$API_UPSTREAM` を展開

## 変更ファイルリスト（今回タスク）

| ファイル | 操作 | 変更内容 |
|----------|------|---------|
| `docs/frontend.md` | 更新 | nginx reverse proxy セクション追記 |
| `docs/terraform-setup.md` | 更新 | frontend env 注入テーブル追記 |

注: Terraform (`frontend.tf`) と nginx (`nginx.conf.template`) は前タスクで実装・コミット済みのため本タスクでの変更なし。

## 検証結果

### terraform fmt
```
terraform fmt terraform/modules/cc-tunnel/frontend.tf → OK (変更なし)
```

### terraform validate
```
terraform -chdir=terraform/modules/cc-tunnel validate
→ Success! The configuration is valid.
```

### Docker build
```
docker build --no-cache -t frontend-test apps/frontend/
→ BUILD SUCCESS (multi-stage: node:24-slim builder + nginx:stable-alpine)
```

### LF 改行確認
| ファイル | 結果 |
|----------|------|
| `terraform/modules/cc-tunnel/frontend.tf` | LF only OK |
| `apps/frontend/nginx.conf.template` | LF only OK |
| `apps/frontend/Dockerfile` | LF only OK |
| `apps/frontend/docker-entrypoint.sh` | LF only OK |
| `docs/frontend.md` | LF only OK |
| `docs/terraform-setup.md` | LF only OK |

### circular dependency 確認
```
grep -n "fe_|frontend" terraform/modules/cc-tunnel/main.tf → (no output)
```
main.tf は `fe_` リソースを一切参照しない。frontend → cc-tunnel 片方向参照のみ ✓

## 品質要件チェック

| 要件 | 状態 |
|------|------|
| API_UPSTREAM env が frontend Cloud Run に注入 | ✓ 実装済み（frontend.tf） |
| nginx /api/ reverse proxy が API_UPSTREAM を参照 | ✓ 実装済み（nginx.conf.template） |
| BACKEND_URL = "/api" 維持（フロントエンドコード使用中） | ✓ 維持済み |
| CORS 対応不要（同オリジン化による） | ✓ |
| circular dependency なし | ✓ 確認済み |
| terraform validate PASS | ✓ |
| LF 改行のみ | ✓ 全ファイル確認済み |
| git 操作ゼロ | ✓ |
| logs/ 変更ログ作成済み | ✓ |

## 殿の apply + 動作確認手順

```bash
cd terraform/live/local/cc-tunnel
terragrunt plan   # API_UPSTREAM/BACKEND_URL の差分確認
terragrunt apply  # 殿の許可必須

# 動作確認（apply 後）
# frontend URL は output に表示される
curl -s https://<frontend_url>/api/conversations | jq .
# → cc-tunnel API からのレスポンス（CORS エラーなし）

# SSE ストリーミング確認
curl -N https://<frontend_url>/api/conversations/<id>/messages
# → SSE イベントストリームが返れば成功
```
