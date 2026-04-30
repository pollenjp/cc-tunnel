# Authentication

cc-tunnel の認証は per-session container 上の Claude CLI に対して行う。会話ごとに動的生成されるセッションコンテナ（`cc-remote-agent`）内で PTY プロセスとして `claude /auth` を起動し、フロントエンドの xterm.js TUI から操作する。`/auth/*` エンドポイントはすべて `conversationId` で特定したセッションコンテナにルーティングされる。

2 つの認証方式をサポートし、フロントエンドは認証状態に応じて UI を切り替える。

## 認証方式

### API キー方式

`ANTHROPIC_API_KEY` 環境変数をセッションコンテナに設定する。Claude CLI がこの環境変数を自動的に読み取るため、追加の操作は不要。

### claude.ai OAuth 方式

ブラウザ経由で claude.ai アカウントにログインする方式。per-session container 内で PTY プロセスとして `claude /auth` を起動し、フロントエンドの xterm.js TUI から操作する。

## AuthManager（per-session container）

`apps/cc-remote-agent/internal/auth/manager.go` の `AuthManager` 構造体が認証状態を管理する。

```go
type AuthManager struct {
    mu           sync.Mutex
    loginCmd     *exec.Cmd
    ptyFd        *os.File    // PTY ファイルディスクリプタ（読み書き両用）
    outputBuf    []byte      // PTY の生出力バッファ（追記のみ、クリアしない）
    loginPending bool
    cancelFunc   context.CancelFunc
}
```

**状態管理**:

- `loginPending`: ログインフローが進行中かどうか
- `outputBuf`: PTY 出力の累積バッファ。カーソルベースで差分取得できる
- `ptyFd`: PTY への入力送信に使用

**主要メソッド**:

| メソッド                  | 説明                                                                                             |
| ------------------------- | ------------------------------------------------------------------------------------------------ |
| `GetStatus(ctx)`          | `claude auth status --json` を実行し、`loginPending` フィールドを付加して返す                    |
| `StartLogin(ctx, method)` | `claude /auth` を PTY で起動（タイムアウト 10 分）。ログイン済みまたは進行中の場合は早期リターン |
| `GetOutput(since)`        | `since` カーソル位置以降の PTY 出力を base64 エンコードして返す                                  |
| `SubmitInput(input)`      | 生のキーストロークを PTY stdin に書き込む（例: `"\x1b[A"` ↑矢印、`"\r"` Enter）                  |
| `CancelLogin()`           | PTY プロセスを強制終了し、状態をクリアする                                                       |
| `Logout(ctx)`             | ログイン中の場合はキャンセル後、`claude auth logout` を実行する                                  |

`GetStatus` が返す `AuthStatus`:

```go
type AuthStatus struct {
    LoggedIn         bool   `json:"loggedIn"`
    AuthMethod       string `json:"authMethod"`
    ApiProvider      string `json:"apiProvider,omitempty"`
    Email            string `json:"email,omitempty"`
    OrgName          string `json:"orgName,omitempty"`
    SubscriptionType string `json:"subscriptionType,omitempty"`
    ApiKeySource     string `json:"apiKeySource,omitempty"`
    LoginPending     bool   `json:"loginPending"`
    LoginUrl         string `json:"loginUrl,omitempty"`
}
```

## PTY 認証フロー

```
フロントエンド                cc-tunnel               session container
     |                           |                    (cc-remote-agent)
     |                           |                           |
     | POST /auth/login          |                           |
     | (conversationId 必須)     |                           |
     |-------------------------->|  POST /auth/login         |
     |                           |  (conversationId で特定)  |
     |                           |-------------------------->|
     |                           |  StartLogin()             |
     |                           |  claude /auth (PTY起動)   |
     |                           |<--------------------------|
     |<--------------------------|                           |
     |   { message: "Login started" }                        |
     |                           |                           |
     | GET /auth/pty/stream      |                           |
     | (conversationId 必須)     |                           |
     |-------------------------->|  GET /auth/pty/stream     |
     |                           |-------------------------->|
     |                           |  Subscribe (fan-out)      |
     |                           |  SSE: base64(PTY bytes)   |
     |                           |<--------------------------|
     |<--------------------------|                           |
     |  data: <base64>\n\n       |                           |
     |  (xterm.js が Uint8Array  |                           |
     |   → term.write() で描画   |                           |
     |   ANSI エスケープはそのまま通過)                       |
     |                           |                           |
     | POST /auth/pty/input      |                           |
     | { input: "\r",            |                           |
     |   conversationId: ... }   |  POST /auth/pty/input     |
     |-------------------------->|-------------------------->|
     |                           |  SubmitInput("\r")        |
     |                           |  PTY stdin に書き込み     |
     |                           |<--------------------------|
     |<--------------------------|                           |
     |                           |                           |
     | POST /auth/cancel (任意)  |                           |
     |-------------------------->|  POST /auth/cancel        |
     |                           |-------------------------->|
     |                           |  CancelLogin()            |
     |                           |  PTYプロセス強制終了      |
     |                           |<--------------------------|
     |<--------------------------|                           |
```

### API エンドポイント一覧（all endpoints require conversationId）

| エンドポイント | メソッド | 説明                                                           |
| -------------- | -------- | -------------------------------------------------------------- |
| `/auth/status` | GET      | 現在の認証状態を取得（`?conversationId={uuid}` 必須）          |
| `/auth/login`  | POST     | ログインフロー開始。body に `conversationId` と `method`       |
| `/auth/pty/stream` | GET  | PTY 出力を SSE でストリーミング。`?conversationId=...` 必須    |
| `/auth/pty/input`  | POST | PTY stdin への入力送信。body に `conversationId` 必須          |
| `/auth/cancel` | POST     | 進行中のログインを強制キャンセル。`conversationId` 必須        |
| `/auth/logout` | POST     | Claude CLI からログアウト。`conversationId` 必須               |

## フロントエンドの認証状態

`useAuth` フック（`apps/frontend/src/hooks/useAuth.ts`）が認証状態を管理し、`AuthGuard` コンポーネントが状態に応じた UI を描画する。

### 状態遷移

| 状態                | 条件                                       | UI                           |
| ------------------- | ------------------------------------------ | ---------------------------- |
| **loading**         | `isLoading && !status`                     | スピナー                     |
| **authenticated**   | `status.loggedIn === true`                 | 子コンポーネントを描画       |
| **login_pending**   | `status.loginPending === true`             | AuthTerminal（xterm.js TUI） |
| **unauthenticated** | `!status.loggedIn && !status.loginPending` | ログインボタン               |

### useAuth フック

```
初期マウント → fetchStatus() → isLoading = false
                              ├─ loggedIn → 通常描画
                              ├─ loginPending → 3秒ポーリング開始
                              └─ unauthenticated → ボタン表示

login() → initiateLogin() → fetchStatus()
        ├─ loginPending → 3秒ポーリング（3000ms setInterval）
        └─ loggedIn → ポーリング停止
```

**ポーリング停止条件**: `status.loginPending === false` になった時点で `clearInterval` する。

## POST /auth/finalize-credentials（Internal API）

cc-tunnel が cc-remote-agent に対して呼び出す内部エンドポイント。PTY ログイン完了後、セッションコンテナ内の tmpfs（`/home/user/.claude/`）に書き込まれた `credentials.json` を読み取り、その内容を返す。

cc-tunnel は受け取った credentials JSON を AES-256-GCM で暗号化し、`credentials` テーブルにユーザー単位で UPSERT する。

```
フロントエンド                   cc-tunnel                  session container
     |                              |                       (cc-remote-agent)
     |                              |                               |
     | POST /credentials/relogin/finalize                           |
     | (conversationId)             |                               |
     |----------------------------->|  POST /auth/finalize-credentials
     |                              |------------------------------>|
     |                              |  tmpfs の credentials.json を読む
     |                              |<------------------------------|
     |                              |  { "credentialsJson": "..." } |
     |                              |                               |
     |                              |  AES-256-GCM 暗号化           |
     |                              |  credentials テーブルに UPSERT|
     |<-----------------------------|                               |
     |  { registered: true, isValid: true }                        |
```

**Response 200**

```json
{
  "credentialsJson": "{\"claudeAiOauth\":{...}}"
}
```

**Response 202**: credentials.json がまだ書き込まれていない（PTY ログイン未完了）

---

## AuthGuard と AuthTerminal（xterm.js TUI）

`AuthGuard.tsx` は `useAuth` の状態を受け取り、`loginPending` 時に `AuthTerminal` を表示する。

`AuthTerminal` は xterm.js（`@xterm/xterm`）を使用して PTY 出力をブラウザ上に描画する。

**ターミナル設定**:

- サイズ: 80 列 × 24 行（cc-remote-agent 側の PTY サイズと一致）
- テーマ: Tokyo Night Dark（背景 `#1a1b26`）
- フォント: SF Mono / Fira Code / monospace、13px

**動作**:

1. マウント時に `GET /auth/pty/stream?conversationId=<id>` で SSE 接続を確立する
2. cc-remote-agent 内の `Subscribe()` で fan-out チャネルを取得し、PTY バイト列を受信するたびに `data: <base64>\n\n` を送信する（以前の DB ポーリングと `GetOutput(since)` は除去済み）
3. base64 デコード → `Uint8Array` → `term.write(bytes)` でそのまま書き込み（ANSI エスケープはストリップせずそのまま通過。xterm.js が描画を担う）
4. 出力テキストから URL を正規表現で抽出し、「認証URLを開く」ボタンとして表示
5. キーボード入力は `term.onData` → `submitAuthInput(data)` で PTY に転送
6. クリップボード貼り付けボタン・Enter ボタンも提供

**アンマウント時**: `clearInterval` + `term.dispose()` でリソースを解放。
