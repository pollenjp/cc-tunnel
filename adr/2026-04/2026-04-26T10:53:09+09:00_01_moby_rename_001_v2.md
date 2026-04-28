# cmd_cctunnel_moby_rename_001_v2 変更ログ

## 概要

subtask_moby_rename_001_v2: sdk_runner.go の docker/docker → moby/moby 移行（再適用）。
前回（subtask_moby_rename_001）の成果が巻き戻されていたため、再適用した。

## 変更内容

### apps/cc-tunnel/internal/docker/sdk_runner.go

インポートの変更:
- 削除: `github.com/docker/docker/api/types/container`
- 削除: `github.com/docker/docker/api/types/filters`
- 削除: `github.com/docker/docker/api/types/mount`
- 削除: `github.com/docker/docker/api/types/network`
- 削除: `github.com/docker/docker/client`
- 追加: `github.com/moby/moby/api/types/container`
- 追加: `github.com/moby/moby/api/types/mount`
- 追加: `github.com/moby/moby/api/types/network`
- 追加: `dockerclient "github.com/moby/moby/client"`

API 変更:
- `NewClientWithOpts(FromEnv, WithAPIVersionNegotiation)` → `New(FromEnv)`
- `ContainerCreate` の引数: 6引数 → `ContainerCreateOptions` struct
- `ContainerStart` の戻り値: `error` → `(ContainerStartResult, error)`
- `ContainerStop` の戻り値: `error` → `(ContainerStopResult, error)`
- `ContainerRemove` の戻り値: `error` → `(ContainerRemoveResult, error)`
- `ContainerInspect` の引数・戻り値: `(ctx, id)` → `(ctx, id, ContainerInspectOptions{})`, `ContainerInspectResult`
  - `resp.ID/Name/State.Status` → `resp.Container.ID/Name/State.Status`
- `ContainerList` の戻り値: `[]container` → `ContainerListResult` (.Items でアクセス)
  - `filters.NewArgs` → `make(dockerclient.Filters).Add("name", ...)`

追加:
- `NewSDKRunnerWithTCP(tcpEndpoint string)` 関数 (TCP エンドポイント経由の接続)

### go.mod (変更なし)

go.mod には既に以下が存在しており変更不要:
- `github.com/moby/moby/api v1.54.2`
- `github.com/moby/moby/client v0.4.1`

go mod tidy 実行済み（変更なし）。

## 検証結果

- `go build ./...`: PASS
- `mise run test:cc-tunnel`: 全 PASS (SKIP=0, FAIL=0)
- `mise run lint:cc-tunnel`: 0 issues
- `docker/docker` 参照残留チェック: 0件（完全除去確認）
