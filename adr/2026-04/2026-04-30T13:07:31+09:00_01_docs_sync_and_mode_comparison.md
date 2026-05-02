# ADR: docs 同期・local vs docker_gce モード差異図化（cmd_cctunnel_docs_sync_and_check_fix）

作成: 2026-04-30  
タスク群: subtask_dsac_001a / 002a / 002b / 003a / 005a

---

## 背景と経緯

2026-04 中旬以降の一連の実装変更（SSE PTY ストリーム導入・Bearer 認証統一・credentials 機能追加・local/docker_gce プロバイダー分離・cc-login 廃止）により、`docs/*` と `docs/plantuml/*` が実装から大きく乖離した。

具体的に累積した乖離:

1. **PTY 認証経路の変更**: `GET /auth/output?since=N`（DB ポーリング方式）→ `GET /auth/pty/stream`（SSE text/event-stream + xterm.js fan-out Subscribe）
2. **Bearer 認証の統一**: `mock-token-<uuid>` → 32 バイト乱数 hex 64 文字、全 endpoint に `Authorization: Bearer <token>` 必須化
3. **credentials 機能の追加**: `credentials` テーブル（migrations/007）・`CredentialGuard`・relogin フロー (`/credentials/relogin/start`, `/credentials/relogin/finalize`) が docs 未記載
4. **cc-login / cc-remote-agent-auth の廃止**: `docs/directory-structure.md` 等に削除済みコンポーネントの言及が残存
5. **local vs docker_gce プロバイダー分離**: `EXECUTION_PROVIDER` 環境変数による切り替えがドキュメント未記載
6. **plantuml 図の陳腐化**: 3 枚の puml が旧ポーリング経路のまま、local プロバイダーの起動フロー図が存在しなかった

`cmd_cctunnel_docs_sync_and_check_fix` はこれらの乖離を一括解消するために発令された。

---

## 修正ファイル一覧

### docs/* （10 ファイル、+464 行 / -55 行）

| ファイル | 変更内容 |
|---|---|
| `docs/api.md` | Bearer Token 認証方式一括説明章追加、`/auth/output` → `/auth/pty (SSE)` 置換、`/credentials/relogin/start`, `/credentials/relogin/finalize`, `/auth/finalize-credentials` 章新設 |
| `docs/auth.md` | cc-remote-agent-auth 言及削除、`POST /auth/finalize-credentials` Internal API 章追加、SSE フロー図更新 |
| `docs/architecture.md` | Bearer 認証フロー新章追加（32 バイトトークン・sessionStorage・middleware 自動付与）、PTY + SSE fan-out Subscribe データフロー更新、stripANSI 削除明記 |
| `docs/frontend.md` | mock-token-<uuid> → 32byte hex 形式更新、CredentialGuard・GET /credentials/status・dual-trigger 完了ボタン記載追加 |
| `docs/database.md` | credentials テーブル schema 章追加（migrations/007 由来: id/username/encrypted_data/nonce/key_version/is_valid/last_validated） |
| `docs/sequence.md` | credential relogin sequence diagram 追加（ChatPage → CredentialGuard → relogin/start → PTY → finalize） |
| `docs/screen-navigation.md` | CredentialGuard の `/chat/:id` 入室ガードフロー・dual-trigger 6 ステップ追記 |
| `docs/directory-structure.md` | cc-login・cc-remote-agent-auth・credential_login_sequence.puml 言及削除 |
| `docs/docker.md` | `EXECUTION_PROVIDER` プロバイダー選択（local/docker_gce/cloud_run_sandbox）比較表・docker_gce 環境変数例追加 |
| `docs/terraform-setup.md` | GCE ネットワークタグ `cc-tunnel-agent`・Firewall TCP 2375・VPC 限定セキュリティ注記追加 |

### docs/plantuml/* （7 ファイル: 5 修正 + 2 新規）

| ファイル | 変更内容 |
|---|---|
| `docs/plantuml/auth_flow.puml` | SSE `/auth/pty/stream` Subscribe + xterm.js renderer 置換（旧 polling 停止） |
| `docs/plantuml/credential_session_start_sequence.puml` | `GET /auth/pty/stream` + SubscribePTYStream + SSE event stream 置換 |
| `docs/plantuml/usecase.puml` | PTY ストリーム購読 `GET /auth/pty/stream (SSE)` 置換 |
| `docs/plantuml/docker_gce_component.puml` | 先頭コメントに役割明記（論理コンポーネント図、cross-ref to multi_container） |
| `docs/plantuml/docker_gce_multi_container.puml` | 先頭コメントに役割明記（デプロイメント図、cross-ref to component） |
| `docs/plantuml/local_provider_session_start.puml` | **新規 85 行**: User → cc-tunnel → LocalDockerProvider → SessionManager → DockerRunner (DooD) → コンテナ起動 → POST /init フロー |
| `docs/plantuml/local_vs_docker_gce_comparison.puml` | **新規 108 行**: 上下二段 deployment 図 + 8 軸ノート + 4 観点差分テーブル |

SVG 生成確認:
- `local_provider_session_start.svg`: 37,099 bytes
- `local_vs_docker_gce_comparison.svg`: 39,459 bytes
- 修正済み 5 puml の SVG も自動更新済み

---

## local vs docker_gce モードの差異整理

### アーキテクチャ概要

| 観点 | local (`LocalDockerProvider`) | docker_gce (`DockerGCEProvider`) |
|---|---|---|
| **用途** | ローカル開発 | GCP 本番 |
| **実行ホスト** | cc-tunnel と同一マシン | GCE VM プール（動的生成） |
| **Docker 接続** | `/var/run/docker.sock` (DooD) | `tcp://<vm-ip>:2375`（VPC + Firewall 限定） |
| **コンテナ命名** | `cctunnel-session-{convID-prefix8}` | `session-{conversationID}` |
| **VM 選択** | 不要（単一ホスト） | `db.GetAvailableVMInstance` + 空き無し時に `createGCEVM` |
| **ネットワーク** | `apps_default` compose ネットワーク内通信 | GCE Internal IP + ホストポート割当 |

### メタデータ保管

| 観点 | local | docker_gce |
|---|---|---|
| **セッション情報** | in-memory `sessions map[convID]*session` | DB テーブル `vm_instances` + `session_endpoints` |
| **並行制御** | `sync.Mutex` | `singleflight.Group`（convID 単位重複排除） |
| **再起動耐性** | プロセス再起動でセッション喪失 | DB 永続化のため復元可能 |

### アイドル/クリーンアップ

| 観点 | local | docker_gce |
|---|---|---|
| **トリガ** | コンテナ単位 `idleTimer`（15 分デフォルト） | `IdleChecker` + `VMScaler` goroutine |
| **削除対象** | アイドルコンテナのみ | アイドル session_endpoints + アイドル VM（GCE DeleteInstance） |
| **Orphan cleanup** | 停止状態の `cctunnel-session-*` 削除 | idle endpoint + idle VM の 2 段階削除 |

### credentials / 認証（両モード共通）

両モードともインターフェース設計の意図通り、以下のフローは完全共通:
- `PrepareForRelogin(convID)` → credentials 無しでセッションコンテナ起動
- フロントから PTY 認証 → `PullCredentialsFromSession` → `FinalizeCredentials`（内部 `credentials.json` 読み出し）
- cc-tunnel が AES-256-GCM で暗号化して `credentials` テーブルに UPSERT

---

## mise run check 状況

実行コマンド: `cd ~/ghq/github.com/pollenjp/cc-tunnel/apps && mise run check`

| サブタスク | 状態 | 備考 |
|---|---|---|
| test:cc-remote-agent | ✅ PASS | SKIP=0 |
| test:cc-tunnel | ✅ PASS | SKIP=0 |
| lint:cc-remote-agent | ✅ PASS | 0 issues |
| lint:cc-tunnel | ⚠️ 既知 race（1 回目 NG / 2 回目 OK） | 並列 golangci-lint キャッシュ race。過去 cmd でも同一現象 |
| lint:frontend | ✅ PASS | eslint 完走 |
| test:frontend | ✅ PASS | 134 tests / 22 files / SKIP=0 |

**判断**: `lint:cc-tunnel` の単発失敗は並列 golangci-lint キャッシュ race。機能上の問題なし。
恒久対策（`mise.toml` で lint を逐次化 or `--cache-dir` 分離）は任意の `subtask_dsac_004a` として別立て。
本 cmd では「既知問題として温存」を正式判断とする。

---

## 主要な設計判断

### 1. PTY 認証を SSE ストリームに統一

**決定**: `GET /auth/output?since=N`（DB ポーリング）を廃止し、`GET /auth/pty/stream`（text/event-stream）に一本化。

**理由**: DB ポーリングは 250ms ごとに DB アクセスが発生しリアルタイム性も低い。SSE + in-memory fan-out（`AuthManager.Subscribe`）により DB 負荷削減とリアルタイム PTY 表示を両立できる。xterm.js への ANSI エスケープ直通（stripANSI 削除）も前提。

### 2. local / docker_gce プロバイダー共通インターフェース

**決定**: `ExecutionProvider` インターフェースで両モードを抽象化し、`EXECUTION_PROVIDER` 環境変数で切り替え。cloud_run_sandbox はモック実装として予約枠。

**理由**: ローカル開発と GCP 本番の実行基盤差異をアプリケーションコードから隠蔽。credentials フローは両モード共通化し、インフラ差異を provider 層に閉じ込める。

### 3. docker_gce_component.puml と docker_gce_multi_container.puml の役割分担

**決定**: 統廃合せず、役割をコメントで明記して並存。component 図（論理構成）と multi_container 図（デプロイメント）として位置づけを明確化。

**理由**: 統廃合は情報損失リスクがあり、図の目的が異なるため。廃止判断は将来の cmd に委ねる。

### 4. mise check lint race の温存

**決定**: `lint:cc-tunnel` の並列 golangci-lint キャッシュ race を今回の cmd では恒久対策しない。

**理由**: 本 cmd の主目的は docs 同期とモード差異図化。lint race は機能影響なし、再実行で解消。恒久対策は独立 subtask（dsac_004a）として家老の判断に委ねる。

---

## 関連ファイル

- local provider: `apps/cc-tunnel/internal/provider/local/docker_provider.go`
- local session: `apps/cc-tunnel/internal/docker/session_manager.go`
- docker_gce provider: `apps/cc-tunnel/internal/provider/dockergce/provider.go`
- ExecutionProvider 抽象: `apps/cc-tunnel/internal/provider/provider.go`
- credentials migration: `apps/cc-tunnel/db/migrations/007_create_credentials.sql`
- mise tasks: `apps/mise.toml`
