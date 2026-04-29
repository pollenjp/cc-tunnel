# Credential Management — セッションコンテナ統合方式（Phase 1 改訂版）

> 改訂版 設計ドキュメント（subtask_cclogin_002a 軍師起草）
> 関連 cmd: `cmd_cctunnel_cc_login_design_impl`
> 旧版（cc-login 独立サービス前提）は本ファイルで全面置換。
> 改訂理由: 殿の裁定 2026-04-29 — Cloud Run concurrency=80 セキュリティリスクへの対応

## 1. 改訂の動機（Cloud Run concurrency リスク）

### 旧設計の問題

旧版（v1）では `cc-login` を独立 HTTP サービスとして apps/cc-login/ に新設し、
PTY による `claude /auth` 実行と credentials 暗号化・DB 保存を担わせる方針だった。

しかし Cloud Run はデフォルト concurrency=80（同一インスタンスが複数リクエストを並行処理）。
cc-login が Cloud Run 上で動作した場合：

- ユーザーAの `claude /auth` フローが書き込んだ `~/.claude/.credentials.json` が
- 同一インスタンスで処理されているユーザーBの finalize 処理から読まれる
- → **ユーザー隔離が破綻し、credentials の他者漏えいリスク**

これは cc-remote-agent-auth（既存の認証専用常駐コンテナ）も抱える同質の問題じゃ。

### 改訂方針

**credentials のライフサイクルを「セッション毎の Docker コンテナ」内に閉じ込める**。

cc-tunnel は既に会話毎にセッションコンテナ（cc-remote-agent 派生）を起動する仕組みを
持っており、各コンテナは tmpfs:/home/user/.claude を持つ独立空間。
このコンテナの中で「認証も会話実行も両方行う」ことで、
**Docker コンテナ単位の隔離が credentials のユーザー隔離に流用できる**。

→ cc-login を独立アプリとして作る必要は無くなる。

## 2. 新アーキテクチャ

```
┌────────────────────────────────────────────────────────────────────────┐
│                              User Browser                              │
└───────────────┬───────────────────────────────────────────────────────┘
                │ HTTP
                ▼
        ┌───────────────┐                       ┌───────────────────┐
        │   Frontend    │ ─── /api/* ───▶       │     cc-tunnel     │
        │ (React+nginx) │                       │  (会話＋credential) │
        └───────────────┘                       └─────────┬─────────┘
                                                          │
                                       (1)Existing /auth/*│  (2)New /auth/finalize-credentials
                                          PTY proxy       │     pull from container
                                                          ▼
                                              ┌───────────────────────┐
                                              │ Session Container     │
                                              │ (cc-remote-agent      │
                                              │  per conversationID)  │
                                              │                       │
                                              │  tmpfs:/home/user/    │
                                              │   .claude/            │
                                              │   .credentials.json   │
                                              │  ┌─────────────────┐  │
                                              │  │ claude CLI      │  │
                                              │  │ /auth (PTY) or  │  │
                                              │  │ execute (HTTP)  │  │
                                              │  └─────────────────┘  │
                                              └─────────┬─────────────┘
                                                        │
                                                        ▼
                                               ┌────────────────┐
                                               │ Anthropic API  │
                                               └────────────────┘

       ┌──────────────────────────────────────┐
       │ PostgreSQL                            │
       │  credentials (暗号化済み)              │
       │   - cc-tunnel.CredentialService が読む │
       │   - 暗号化のみ cc-tunnel 内で完結       │
       └──────────────────────────────────────┘
```

**消えるもの**:
- `apps/cc-login/` ディレクトリ全体
- `apps/compose.yaml` の `cc-login` サービス
- `cc-remote-agent-auth` 共有常駐コンテナ（Phase 2 で廃止予定 — cmd_cctunnel_cc_remote_agent_auth_retire で廃止済み）

**残るもの**:
- `apps/cc-tunnel/internal/credential/` (encryptor / repository / service)
- `apps/cc-tunnel/internal/db/migrations/007_create_credentials.sql`
- `apps/cc-remote-agent/internal/api/handler.go` の `POST /init` endpoint
- `apps/cc-tunnel/internal/docker/session_manager.go` の tmpfs + /init 呼び出し

**追加するもの**:
- `cc-remote-agent` の `POST /auth/finalize-credentials` endpoint（コンテナ内 credentials.json を読んで返す）
- `cc-tunnel` の `RemoteClient` に `FinalizeCredentials` メソッド
- `cc-tunnel` API の再ログイン finalize endpoint（フロントが「ログイン完了」を通知 → cc-tunnel が pull → 暗号化 → DB 保存）

## 3. 設計判断: 「pull 方式 (cc-tunnel 主導)」を採用

### 選択肢比較

| 方式 | 概要 | 採否 |
|------|------|------|
| **選択肢1 (採用)**: cc-tunnel 主導 / pull | cc-tunnel が既存 `/auth/*` PTY proxy を中継し、完了後に新 endpoint で credentials を pull する | ✅ |
| 選択肢2: cc-remote-agent 主導 / push | cc-remote-agent が自律的に再ログインし、cc-tunnel に webhook 通知 | ✗ |

### 選択肢2 を不採用とした理由

1. **`claude /auth` は対話型 TUI**: OAuth 認可コードをユーザーが貼り付ける必要がある。
   ヘッドレス自動再ログインは原理的に不可能。
2. **cc-remote-agent からの push にはサービスディスカバリと認証が必要**:
   GCE VM 上のコンテナから cc-tunnel API を呼ぶには内部 URL の解決と相互認証が要る。
   一方 pull なら cc-tunnel が自身の RemoteClient を使うだけで済む（既存パターン）。
3. **既存の `/auth/*` PTY エンドポイント群が再利用可能**:
   `GetAuthStatus`, `InitiateLogin`, `SubmitAuthInput`, `GetAuthOutput`, `CancelLogin`, `Logout`
   が cc-tunnel の `RemoteClient` に既にある。改修箇所が局所化される。

## 4. ライフサイクル詳細

### 4.1 ケースA: 新規会話開始（credentials 既存・有効）

```
1. POST /conversations/{id}/messages
2. cc-tunnel.handler.SendMessage:
     username 解決 → CredentialService.FetchAndDecrypt
     → 復号成功
3. ExecutionProvider.Execute(req with credentials)
     → SessionManager.GetOrCreate(conversationID, credentials_json)
        → ContainerCreate (tmpfs:/home/user/.claude)
        → POST /init {credentialsJson} （既存 endpoint）
4. POST /execute → claude CLI 実行（既存フロー）
```

### 4.2 ケースB: 新規会話開始（credentials 未登録）

```
1. POST /conversations/{id}/messages
2. cc-tunnel.handler.SendMessage:
     CredentialService.FetchAndDecrypt → ErrNotFound
3. cc-tunnel returns 401:
     {error: "credentials_required",
      redirect: "/login/credentials?reason=missing&conversationId=..."}
4. Frontend: ユーザーをログイン画面へ誘導 → ケースD（再ログインフロー）へ
```

### 4.3 ケースC: 新規会話開始（credentials 失効）

```
1. POST /conversations/{id}/messages
2. cc-tunnel.handler.SendMessage:
     CredentialService.FetchAndDecrypt → ErrCredentialsInvalid
3. cc-tunnel returns 401:
     {error: "credentials_invalid",
      redirect: "/login/credentials?reason=expired&conversationId=..."}
4. Frontend: ケースD へ
```

### 4.4 ケースD: 再ログインフロー（セッションコンテナ統合方式の核心）

```
1. Frontend: POST /api/credentials/relogin/start
     body: {conversationId: "..."} ← この会話用のセッションコンテナを起こす起点
   cc-tunnel:
     SessionManager.GetOrCreate(conversationID, credentials=nil)
       → 認証用に空のセッションコンテナを起動（credentials なし、tmpfs 空）
     → 200 {ready: true}

2. Frontend: 既存の /auth/login → cc-tunnel が remoteClient.InitiateLogin を呼ぶ
     remoteClient は当該会話のセッションコンテナを向いている
     コンテナ内で `claude /auth` が PTY 起動
   cc-tunnel: 200 {message, loginUrl?}

3. Frontend ↔ cc-tunnel ↔ session container（PTY 中継）:
     POST /auth/input  ← ユーザー入力
     GET  /auth/output ← PTY 出力
   ※ 既存の RemoteClient 経路をそのまま使う

4. Frontend が「ログイン完了」を検知 → POST /api/credentials/relogin/finalize
     body: {conversationId: "..."}
   cc-tunnel:
     a) remoteClient.FinalizeCredentials(ctx)
        → 当該コンテナの POST /auth/finalize-credentials を呼ぶ
        → 戻り値: {credentialsJson: "<生 JSON>"}
     b) Encryptor.Seal(credentialsJson, AAD=username) → ciphertext, nonce
     c) CredentialRepository.Upsert(...)
        → ON CONFLICT で既存行を上書き、is_valid=TRUE に再活性化
     d) cc-remote-agent に「credentials.json を削除して」とは指示しない
        （セッションコンテナは会話終了とともに破棄されるため tmpfs 上の credentials も自動消滅）
   cc-tunnel returns 200 {registered: true, isValid: true}

5. Frontend: 当該会話画面に戻り、メッセージ送信を再試行
   → ケースA に合流
```

### 4.5 ケースE: 実行中に Anthropic API が 401（credentials 失効を実行時検出）

```
1. SendMessage 進行中、claude CLI が 401 Unauthorized
2. ExecutionProvider が捕捉、CredentialService.MarkInvalid(username)
3. 次回送信は ケースC で 401 redirect → ケースD へ
```

## 5. 認証用コンテナと会話用コンテナの統合

旧 v1 では「会話セッション」と「認証セッション」を区別する余地があったが、本改訂では
**両者を同一の Docker コンテナ（会話 ID 単位）に統合する**。

### 統合の利点

1. Cloud Run concurrency 問題が原理的に発生しない（コンテナ毎に独立 OS namespace）
2. SessionManager の既存ライフサイクル（idle 15 分で停止、会話終了で破棄）がそのまま流用できる
3. tmpfs:/home/user/.claude 上の credentials は会話終了で自動消滅
4. cc-remote-agent-auth 共有常駐コンテナの存在意義が消滅 → Phase 2 で廃止済み（cmd_cctunnel_cc_remote_agent_auth_retire）

### 統合に伴う設計上の留意点

- **同時並行ログインは別コンテナ**: ユーザーAとユーザーBが同時にログインしても、
  それぞれ別の会話 ID に紐づく別コンテナで /auth が走る。互いに credentials.json を
  目視・読取できない。
- **セッションコンテナの生存期間**: 通常は 15 分でアイドル停止だが、再ログイン中は
  ユーザー操作が継続するためアイドルとはみなされない（remoteClient 操作で
  `last_activity` 更新）。
- **認証専用コンテナという概念は消失**: ログインのために「会話を作る」ことになるが、
  これはユーザー視点で違和感が無い（メッセージ送信時にログイン画面に飛ばされて、
  そのままログインしてその会話に戻る、という自然な動線）。

## 6. データモデル（変更なし）

`007_create_credentials.sql` の credentials テーブルはそのまま。

```sql
-- 既存 migration（変更なし）
CREATE TABLE credentials (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    username        TEXT        NOT NULL UNIQUE,
    encrypted_data  BYTEA       NOT NULL,
    nonce           BYTEA       NOT NULL,
    key_version     INTEGER     NOT NULL DEFAULT 1,
    is_valid        BOOLEAN     NOT NULL DEFAULT TRUE,
    last_validated  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

## 7. 暗号化（変更なし）

AES-256-GCM + AAD=username + 環境変数鍵 `CC_LOGIN_ENCRYPTION_KEY`（名前は移行コスト
回避のためそのまま、用途は cc-tunnel の暗号化）。

旧 cc-login 側にあった `encryptor.go` の duplication は本改訂で自然解消する
（cc-login ディレクトリ自体が消えるため）。

## 8. API 仕様

### 8.1 cc-remote-agent — 新エンドポイント

#### POST /auth/finalize-credentials

```http
POST /auth/finalize-credentials
(no body, no auth) ※ 内部ネットワーク前提（既存 /auth/* と同水準のセキュリティ境界）
```

レスポンス:

```json
200 OK
{
  "credentialsJson": "<file content of /home/user/.claude/.credentials.json>"
}
```

```json
404 Not Found
{
  "error": "credentials file not found",
  "hint": "complete /auth/login flow first"
}
```

実装方針:
- `os.UserHomeDir()` → `~/.claude/.credentials.json` を読む
- ファイル不在なら 404
- 読み取り後にファイル削除はしない（コンテナ破棄まで残置）
- ※ 既存 `/init` endpoint（書き込み側）と対称的な構造

### 8.2 cc-tunnel — 新エンドポイント

#### POST /api/credentials/relogin/start

```http
POST /api/credentials/relogin/start
Authorization: Bearer <app-auth token>
Content-Type: application/json

{ "conversationId": "<UUID>" }
```

レスポンス:
```json
200 OK { "ready": true }
```

実装:
- token から username 解決
- `SessionManager.GetOrCreate(conversationId, credentials=nil)` で空コンテナ起動
- credentials 不在ゆえ /init はスキップ

#### POST /api/credentials/relogin/finalize

```http
POST /api/credentials/relogin/finalize
Authorization: Bearer <app-auth token>
Content-Type: application/json

{ "conversationId": "<UUID>" }
```

レスポンス:
```json
200 OK { "registered": true, "isValid": true }
```

実装:
- token から username 解決
- `remoteClient.FinalizeCredentials(ctx)` で当該会話のコンテナから credentials を pull
- `Encryptor.Seal` → `Repository.Upsert`（既存実装のまま使える）
- 失敗時は 502 Bad Gateway（コンテナ側の問題）または 500（暗号化／DB 問題）

#### GET /api/credentials/status (任意)

```http
GET /api/credentials/status
Authorization: Bearer <app-auth token>
```

レスポンス:
```json
200 OK { "registered": true, "isValid": true, "lastValidatedAt": "..." }
```

これは Phase 1 の利便性のため任意。CredentialGuard はこの endpoint で
fast-path 確認できる。

### 8.3 既存 endpoint（変更なし）

- `/auth/login`, `/auth/input`, `/auth/output`, `/auth/cancel`, `/auth/logout`, `/auth/status`
  すべて残置。RemoteClient は当該会話のセッションコンテナを向くよう注意。
- `/conversations/*`, `/app-auth/*` 変更なし
- cc-remote-agent の `/init`, `/execute`, `/auth/*` 変更なし

## 9. cc-tunnel 内部の主な変更

### 9.1 RemoteClient

`apps/cc-tunnel/internal/remoteclient/client.go`:

```go
// 新メソッド追加
func (c *Client) FinalizeCredentials(ctx context.Context) (string, error) {
    // POST <baseURL>/auth/finalize-credentials
    // 200 → response.credentialsJson を返す
    // 404 → ErrCredentialsNotReady を返す
}
```

### 9.2 Handler

`apps/cc-tunnel/internal/api/handler.go` に 2 ハンドラ追加:
- `PostReloginStart`
- `PostReloginFinalize`

### 9.3 SessionManager の credentials 引数の扱い

既存の `GetOrCreate(conversationID, credentials []byte)` は credentials が nil の場合に
`/init` 呼び出しをスキップするよう調整（既に nil-safe ならそのまま）。

### 9.4 既存テスト互換性

`SendMessage` の credService.FetchAndDecrypt 呼び出しと 401 redirect は v1 と同じ。
新 endpoint のテストを追加すれば既存テストには影響しない。

## 10. cc-remote-agent 内部の主な変更

`apps/cc-remote-agent/internal/api/handler.go`:
- `Init` ハンドラ（既存）はそのまま
- 新ハンドラ `FinalizeCredentials`:
  - `os.UserHomeDir()/.claude/.credentials.json` を `os.ReadFile`
  - 不在なら 404、その他エラーは 500
  - 200 で `{credentialsJson: <file content>}` を返す

`apps/cc-remote-agent/cmd/cc-remote-agent/main.go`:
- `mux.HandleFunc("/auth/finalize-credentials", handler.FinalizeCredentials)`

## 11. フロントエンドへの影響

旧 v1 と概念は同じだが向き先が変わる。

| 改修対象 | 旧 v1 | 改訂後 |
|---------|------|-------|
| API 呼び出し先 | cc-login (`http://cc-login:9092`) | cc-tunnel (`/api/credentials/*`) |
| /credentials/login | cc-login の独自 endpoint | 既存 `/auth/login` をそのまま |
| /credentials/finalize | cc-login の独自 endpoint | 新 `/api/credentials/relogin/finalize` |
| CredentialGuard | cc-login の status を確認 | cc-tunnel の `/api/credentials/status` を確認（任意） |

フロントエンドのコンポーネント名は名残で「Credentials」を含むまま（CredentialGuard,
CredentialsLoginPage）でよい。本改訂は backend 側の構造変更が主なので、フロントは
「呼び先 URL の変更」が中心となる。

## 12. Phase 1 と Phase 2 の境界（改訂後）

### Phase 1（本 cmd を含む）

- ✅ migration 007（既存）
- ✅ cc-tunnel encryptor / repository / service（既存）
- ✅ cc-tunnel SendMessage の credentials チェック（既存）
- ✅ tmpfs マウント＋/init endpoint（既存）
- ✅ cc-remote-agent `/auth/finalize-credentials` endpoint（新規）— 完了 002c
- ✅ cc-tunnel `RemoteClient.PullCredentialsFromSession`（新規）— 完了 002c
- ✅ cc-tunnel `/api/credentials/relogin/start`, `/relogin/finalize`（新規）— 完了 002c
- ✅ cc-login 削除（apps/cc-login/ 全削除、compose.yaml 削除、apps/mise.toml 削除）— 完了 002c
- ✅ ADR 改訂 — 完了 002e
- ✅ docs 改訂（本ファイル + puml 削除）— 完了 002e
- ✅ フロント改修（CredentialGuard, CredentialsLoginPage + compose CC_LOGIN_ENCRYPTION_KEY 注入）— 完了 002d

### Phase 2

- users テーブル新設・FK 移行
- ✅ cc-remote-agent-auth 共有常駐コンテナ廃止（cmd_cctunnel_cc_remote_agent_auth_retire で完了）
- KMS 連携
- 鍵ローテーション運用
- audit log
- 定期 credentials 有効性チェック goroutine
- PTY manager 単体テスト充実（cc-remote-agent 側）

## 13. 残課題・問いかけ（殿/家老向け）

| ID | 内容 | 軍師の見解 | 要判断 |
|----|------|----------|-------|
| Q1 | 旧 cc-login 関連コード（apps/cc-login/, compose.yaml, mise.toml）の削除はこの cmd で同時実施するか別 cmd か | 推奨：本 cmd 内で削除（足軽3号担当）→ Phase 1 完了の境界が明確になる | 殿/家老 |
| Q2 | docs/plantuml/credential_login_sequence.puml（旧 cc-login PTY 図）の扱い | 推奨：削除（実装と乖離する遺物） | 軍師判断で削除 |
| Q3 | cc-remote-agent-auth 廃止は Phase 2 とするか即時実施か | 推奨：Phase 2 へ。本 cmd では「廃止しても動く」状態まで持っていけば良い | 殿 |
| Q4 | 認証中もアイドルタイマーが回るリスクへの対処 | 推奨：`/auth/*` 操作で last_activity を更新する小修正を Phase 1 に含める | 軍師判断で実装ガイドに含める |
| Q5 | concurrency=80 でも cc-tunnel API ハンドラ自体は共有プロセス上で動く点 | コンテナ単位の隔離は credentials の保管・展開フェーズで担保される。cc-tunnel ハンドラは memory 上で credentials を扱うが、関数局所スコープゆえユーザー間でリークしない（goroutine ローカル） | 説明済 |

## 14. 実装サマリ（完了 2026-04-29）

| 足軽 | 担当 subtask | 内容 |
|------|-------------|------|
| 足軽1号 | subtask_cclogin_002b | cc-remote-agent 改修＋ cc-tunnel RemoteClient/API 実装 |
| 足軽2号 | subtask_cclogin_002c | cc-login 削除＋ compose/mise 整理＋ cc-tunnel API 追加 |
| 足軽3号 | subtask_cclogin_002d | frontend CredentialGuard + CredentialsLoginPage + compose 注入 |
| 足軽3号 | subtask_cclogin_002e | ADR 改訂＋旧 puml 削除＋ E2E テスト追加＋最終 check |
