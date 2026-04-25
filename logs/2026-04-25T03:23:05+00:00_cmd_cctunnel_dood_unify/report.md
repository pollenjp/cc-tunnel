# cmd_cctunnel_dood_unify 変更ログ

## 1. 経緯・背景

cmd_cctunnel_local_dev_host で追加した HostMode（CC_TUNNEL_HOST_MODE=true による
ランダムポート公開 + localhost 接続）を廃止。

docker.sock マウント（DooD）により cc-tunnel がコンテナ内でも docker 操作可能であるため、
HostMode による分岐は不要。URL 生成を常に Docker ネットワーク DNS 方式に統一。

## 2. 変更内容

### 2.1 runner.go
- ContainerCreateOpts から ExposePorts []string フィールドを削除
- ContainerInfo から HostPort string フィールドを削除

### 2.2 sdk_runner.go
- nat import 削除（github.com/docker/go-connections/nat）
- ContainerCreate から exposedPorts/portBindings 構築ブロック削除
- ContainerInspect から HostPort 取得コード削除

### 2.3 session_manager.go
- SessionManagerConfig から HostMode bool フィールドを削除
- GetOrCreate から HostMode 分岐（ExposePorts 設定 + ContainerInspect + localhost URL）削除
- URL 生成を常に DNS 方式に統一:
  `containerURL = "http://" + containerName + ":" + sm.config.ContainerPort`

### 2.4 main.go
- SessionManagerConfig の HostMode: os.Getenv("CC_TUNNEL_HOST_MODE") == "true" を削除

### 2.5 session_manager_test.go（削除テスト）
- TestContainerCreateOpts_ExposePorts_passthrough
- TestContainerInfo_HostPort
- TestContainerInfo_HostPort_empty_when_not_exposed
- TestSessionManager_HostMode_usesLocalhostURL
- TestSessionManager_HostMode_setsExposePorts

### 2.6 mise.toml
- CC_TUNNEL_HOST_MODE=true 削除
- DOCKER_NETWORK="" → DOCKER_NETWORK="${DOCKER_NETWORK:-apps_default}" に変更

### 2.7 docs/architecture.md
- HostMode セクション削除
- DooD 方式の説明に更新

## 3. テスト結果

- go build ./...: PASS
- go test ./internal/docker/... -count=1 -v: 6 tests PASS（SKIP=0, FAIL=0）
- go test ./cmd/cc-tunnel/... -count=1: PASS
- golangci-lint run ./...: 0 issues

## 4. 品質確認

- HostMode 分岐: 全ファイルから削除済み
- CC_TUNNEL_HOST_MODE: 全ファイルから削除済み
- ExposePorts / HostPort: 使用箇所なし、削除済み
- DNS 方式 URL 生成のみ残存
- LF 改行のみ（CRLF=0）
