# cmd_cctunnel_local_dev_host 変更ログ

## 1. 経緯・背景
- 旧構成: 全コンポーネントを Docker compose で起動
- 新構成: postgres + cc-remote-agent-auth のみ compose、cc-tunnel/frontend はホスト起動
- 動機: ローカル開発時の docker build 待ち時間の削減、DinD 不要化

## 2. 設計決定（軍師レビュー: gunshi_local_dev_host_design）
- ホスト接続方式: 案A（bridge + ランダムポート公開）採用
  - macOS/Linux/WSL2 クロスプラットフォーム対応
  - 127.0.0.1:RANDOM → container:9091 でポート公開
- SessionManagerConfig.HostMode bool 追加
- compose.yaml: profiles: ["full"] で cc-tunnel/frontend を分離
- AF001 修正: volumes.claude-sessions.name: claude-sessions でボリューム名不整合解消

## 3. 変更内容

### 3.1 Docker レイヤー
- ContainerCreateOpts.ExposePorts []string 追加
- ContainerInfo.HostPort string 追加
- SDKRunner.ContainerCreate: ExposePorts → PortBindings 変換
- SDKRunner.ContainerInspect: HostPort 取得

### 3.2 SessionManager
- SessionManagerConfig.HostMode bool 追加
- GetOrCreate: HostMode 時の ExposePorts 設定 + localhost:{HostPort} URL 生成
- Env: []string{"PORT=" + sm.config.ContainerPort} を ContainerCreateOpts に追加

### 3.3 main.go
- CC_TUNNEL_HOST_MODE=true 時に SessionManagerConfig.HostMode=true

### 3.4 インフラ（compose.yaml）
- cc-tunnel/frontend を profiles: ["full"] で分離
- postgres: ports 127.0.0.1:5432:5432 追加
- cc-remote-agent-auth: ports 追加
- volumes.claude-sessions.name: claude-sessions 追加（AF001修正）

### 3.5 mise タスク
- cc-tunnel/mise.toml: cc-tunnel:dev:up を正しいフラグ・環境変数に修正
- apps/mise.toml: dev:up にインフラ起動ガイド追加

## 4. テスト
- TestSessionManager_HostMode_usesLocalhostURL
- TestSessionManager_HostMode_setsExposePorts
- TestSessionManager_compose_mode_unchanged
- ExposePorts/HostPort 関連テスト 3件
- 計 11テスト全 PASS（SKIP=0, FAIL=0）

## 5. 品質確認
- mise run check: PASS（全パッケージ SKIP=0, FAIL=0）
- lint: 0 issues
