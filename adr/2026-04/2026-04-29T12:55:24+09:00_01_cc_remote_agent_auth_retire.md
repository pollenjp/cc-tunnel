# ADR: cc-remote-agent-auth 共有常駐コンテナの廃止

**タイムスタンプ**: 2026-04-29T12:55:24+09:00  
**ステータス**: Accepted  
**関連 cmd**: `cmd_cctunnel_cc_remote_agent_auth_retire`

---

## 背景

`cmd_cctunnel_cc_login_design_impl` Phase 1 改訂（subtask_cclogin_002b〜002e）にて、credentials のライフサイクルを per-session container 内に閉じ込める設計に移行した。この設計変更により、`/auth/*` PTY 群を会話ごとのセッションコンテナ（`cc-remote-agent`）内で動作させることが可能となり、`cc-remote-agent-auth` 共有常駐コンテナの存在意義が消滅した。

### 廃止前の不整合

`cmd_cctunnel_cc_login_design_impl` Phase 1 完了時点で、以下の重大な不整合が残存していた:

- `PrepareForRelogin` / `PullCredentialsFromSession` は per-session container を参照 ✅
- `/auth/*` PTY 群（login/input/output/cancel/status/logout）は共有 `cc-remote-agent-auth` コンテナを参照 ❌

これにより:
1. `PrepareForRelogin` で起こした空の per-session コンテナでは PTY が走っていない
2. `/auth/login` の PTY は共有コンテナで動作し、`claude-sessions` volume に credentials を書く
3. `PullCredentialsFromSession` は空の per-session コンテナを覗く → **404 (credentials not ready)**
4. 実 PTY を介した relogin フローが破綻

---

## 決定事項

### Stage 1: `/auth/*` の per-session ルーティング化（subtask_ccauth_001b）

- `OpenAPI` の `/auth/*` 6 エンドポイント全てに `conversationId` パラメータを追加（破壊的変更）
- `cc-tunnel handler.go` の `/auth/*` ハンドラを `executionProvider.GetSessionClient(conversationId)` 経由に変更
- `ExecutionProvider` インターフェースに `GetSessionClient(ctx, convID) (*remoteclient.Client, error)` を追加
- `handler.go` の `remote` フィールド（共有 `remoteClient`）を削除
- `frontend` の `/auth/*` 呼び出しに `conversationId` を付与

### Stage 2: compose / volume の削除（subtask_ccauth_001c）

- `apps/compose.yaml` から `cc-remote-agent-auth` サービスブロックを削除
- `claude-sessions` Docker volume を削除（per-session container は tmpfs を使用）
- `cc-tunnel main.go` の `-agent-url` フラグを削除
- `NewHandlerFull` シグネチャから `remote` 引数を削除

### Stage 3: docs / puml / ADR 改訂（subtask_ccauth_001d）

- 本 ADR の作成
- `docs/` 7 ファイル改訂（「cc-remote-agent-auth 常駐コンテナ」前提を「per-session container 内 PTY」前提に書換）
- `docs/plantuml/` 3 ファイル改訂
- 一時ガイドファイル削除

---

## 廃止対象一覧

| 種別 | 対象 | 対応 |
|------|------|------|
| compose service | `cc-remote-agent-auth` | 削除 |
| Docker volume | `claude-sessions` | 削除 |
| cc-tunnel フラグ | `-agent-url` | 削除 |
| handler フィールド | `Server.remote` / `remoteClient` インターフェース | 削除 |
| OpenAPI パラメータ | `/auth/*` の conversationId なし variant | 廃止（conversationId 必須化） |

---

## `/auth/*` per-session ルーティング化の判断根拠

1. **credentials 隔離の完結**: per-session container 内で `/auth` も実行することで、`tmpfs:/home/user/.claude` の credentials が他ユーザーに漏れない
2. **共有コンテナの同時並行リスク解消**: Cloud Run concurrency=80 環境で `cc-remote-agent-auth` が共有されると、複数ユーザーの PTY が同一プロセスに混入するリスクがある
3. **既存インタフェースの再利用**: `SessionManager.GetOrCreate` と `remoteclient.Client` がそのまま利用可能
4. **本番環境への影響軽微**: Cloud Run + DockerGCE 本番環境では既に `cc-remote-agent-auth` の独立常駐サービスが存在しなかった

---

## 影響範囲

| レイヤ | 変更内容 |
|--------|---------|
| compose.yaml | `cc-remote-agent-auth` サービス削除、`claude-sessions` volume 削除 |
| OpenAPI | `/auth/*` 全エンドポイントに `conversationId` 追加（破壊的変更） |
| cc-tunnel handler.go | `remote` フィールド削除、`GetSessionClient` 経由に変更 |
| cc-tunnel main.go | `-agent-url` フラグ削除 |
| cc-tunnel interfaces.go | `remoteClient` インターフェース削除 |
| cc-tunnel remoteclient | Auth 系メソッドは `GetSessionClient` で取得した client から呼ぶ |
| frontend | `/auth/*` 呼び出しに `conversationId` を渡すよう変更 |
| docs | auth.md / architecture.md / docker.md 等 7 ファイル改訂 |
| plantuml | c4_component.puml / auth_flow.puml / screen_navigation.puml 改訂 |

---

## 削除してはならなかったもの

`apps/cc-remote-agent/internal/api/handler.go` の `/auth/*` ハンドラ群（`AuthStatus`, `AuthLogin`, `AuthLogout`, `AuthInput`, `AuthOutput`, `AuthCancel`, `FinalizeCredentials`, `Init`）は **削除しなかった**。

これらは `cc-remote-agent` image 自体に含まれており、per-session container でも `/auth/login` を叩く必要があるため保持必須。廃止したのは「共有常駐としてのデプロイ」と「cc-tunnel の共有 remote 利用」のみ。

---

## relogin フロー修正の成果

Stage 1 完了後、実 PTY が per-session container で動作するようになり、以下のフローが正常化した:

1. `POST /api/credentials/relogin/start` → per-session container 起動
2. `POST /auth/login` → 当該 per-session container 内で `claude /auth` PTY 起動
3. `GET /auth/output` / `POST /auth/input` → per-session container の PTY と中継
4. `POST /api/credentials/relogin/finalize` → per-session container から credentials pull → DB 保存
5. 次回 `SendMessage` で credentials を per-session container に注入 → 会話実行成功

---

## Phase 2 残課題（影響なし）

本 cmd の廃止作業は Phase 2 の以下の課題に影響を与えない:

- users テーブル新設・FK 移行
- KMS 連携・鍵ローテーション
- audit log
- 定期 credentials 有効性チェック goroutine
