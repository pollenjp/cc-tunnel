# cmd_cctunnel_moby_rename_001 変更ログ

## 調査結果

### 直接依存
- 変更前: `github.com/docker/docker v28.5.2+incompatible` (direct)
- 変更後: `github.com/moby/moby/api v1.54.2` + `github.com/moby/moby/client v0.4.1` (both direct)

### インポート箇所
- `apps/cc-tunnel/internal/docker/sdk_runner.go` の5つのインポート:
  - `github.com/docker/docker/api/types/container` → `github.com/moby/moby/api/types/container`
  - `github.com/docker/docker/api/types/filters` → 廃止 (新 client.Filters に統合)
  - `github.com/docker/docker/api/types/mount` → `github.com/moby/moby/api/types/mount`
  - `github.com/docker/docker/api/types/network` → `github.com/moby/moby/api/types/network`
  - `github.com/docker/docker/client` → `github.com/moby/moby/client`

### Transitive 依存の状況
- `github.com/docker/docker v28.5.2+incompatible` は go.mod から完全に削除
- `github.com/docker/go-connections` (v0.6.0→v0.7.0) と `github.com/docker/go-units` は別パッケージのため残存 (手出し不可)
- go.sum 内の transitive 参照に `docker/docker` が残る可能性あり (go mod tidy で自動管理)

## 注意: 単純リネームではなく API 移行が必要だった

`github.com/moby/moby v28.5.2+incompatible`（旧モノリス）はソース内で `docker/docker` パスを内部使用しており、
新しいサブモジュール `github.com/moby/moby/api v1.54.2` と組み合わせると go mod tidy でモジュールパス競合が発生。

解決策: 旧モノリスを使わず、新しいサブモジュールに完全移行した。
新 API では以下のメソッドシグネチャが変更されている:
- `ContainerCreate`: 6引数 → `ContainerCreateOptions` struct
- `ContainerStart/Stop/Remove`: `error` のみ → `(Result, error)`
- `ContainerInspect`: `types.ContainerJSON` → `ContainerInspectResult`
- `ContainerList`: `container.ListOptions{Filters: filters.Args}` → `ContainerListOptions{Filters: client.Filters}`
- `NewClientWithOpts` + `WithAPIVersionNegotiation` → `New` (deprecation 対応)

## 変更したファイル一覧

- `apps/cc-tunnel/internal/docker/sdk_runner.go` — import パス変更 + API 移行
- `apps/cc-tunnel/go.mod` — docker/docker 削除、moby/moby/api + moby/moby/client 追加
- `apps/cc-tunnel/go.sum` — go mod tidy による自動更新

## mise run check 結果

- `cc-tunnel:test`: 全 PASS (SKIP=0, FAIL=0)
- `cc-tunnel:lint`: 0 issues
