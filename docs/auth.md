# Authentication

cc-tunnel の認証は `cc-remote-agent-auth` コンテナ上の Claude CLI に対して行う。`cc-remote-agent-auth` は `compose.yaml` のデフォルトサービスとして常時起動する**認証専用常駐コンテナ**であり、セッションごとに動的生成される実行用 `cc-remote-agent` コンテナとは別物である。

2 つの認証方式をサポートし、フロントエンドは認証状態に応じて UI を切り替える。

## 認証方式

### API キー方式

`ANTHROPIC_API_KEY` 環境変数を `cc-remote-agent-auth` コンテナに設定する。Claude CLI がこの環境変数を自動的に読み取るため、追加の操作は不要。

### claude.ai OAuth 方式

ブラウザ経由で claude.ai アカウントにログインする方式。`cc-remote-agent-auth` 内で PTY プロセスとして `claude /auth` を起動し、フロントエンドの xterm.js TUI から操作する。

## AuthManager（cc-remote-agent-auth）

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
フロントエンド                cc-tunnel               cc-remote-agent-auth
     |                           |                           |
     | POST /auth/login          |                           |
     |-------------------------->|  POST /auth/login         |
     |                           |-------------------------->|
     |                           |  StartLogin()             |
     |                           |  claude /auth (PTY起動)   |
     |                           |<--------------------------|
     |<--------------------------|                           |
     |   { message: "Login started" }                        |
     |                           |                           |
     | GET /auth/output?since=0  |                           |
     |-------------------------->|  GET /auth/output?since=0 |
     |                           |-------------------------->|
     |                           |  GetOutput(0)             |
     |                           |  base64(PTY出力)          |
     |                           |<--------------------------|
     |<--------------------------|                           |
     |  { data: "<base64>", cursor: N }                      |
     |  (xterm.js にデコード表示)                            |
     |                           |                           |
     | POST /auth/input          |                           |
     | { input: "\r" }           |  POST /auth/input         |
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

### API エンドポイント一覧（cc-tunnel が cc-remote-agent へプロキシ）

| エンドポイント | メソッド | 説明                                                     |
| -------------- | -------- | -------------------------------------------------------- |
| `/auth/status` | GET      | 現在の認証状態を取得                                     |
| `/auth/login`  | POST     | ログインフロー開始。`{"method":"claudeai"}` 等を指定可能 |
| `/auth/output` | GET      | PTY 出力をポーリング取得。`?since=N` で差分取得          |
| `/auth/input`  | POST     | PTY stdin への入力送信。`{"input":"<keystrokes>"}`       |
| `/auth/cancel` | POST     | 進行中のログインを強制キャンセル                         |
| `/auth/logout` | POST     | Claude CLI からログアウト                                |

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

## AuthGuard と AuthTerminal（xterm.js TUI）

`AuthGuard.tsx` は `useAuth` の状態を受け取り、`loginPending` 時に `AuthTerminal` を表示する。

`AuthTerminal` は xterm.js（`@xterm/xterm`）を使用して PTY 出力をブラウザ上に描画する。

**ターミナル設定**:

- サイズ: 80 列 × 24 行（cc-remote-agent 側の PTY サイズと一致）
- テーマ: Tokyo Night Dark（背景 `#1a1b26`）
- フォント: SF Mono / Fira Code / monospace、13px

**動作**:

1. マウント時に `setInterval(pollOutput, 250)` を開始（250ms ポーリング）
2. `GET /auth/output?since=<cursor>` でバイナリ差分を取得
3. base64 デコード → `Uint8Array` → `term.write(bytes)` でそのまま書き込み（ANSI エスケープも描画）
4. 出力テキストから URL を正規表現で抽出し、「認証URLを開く」ボタンとして表示
5. キーボード入力は `term.onData` → `submitAuthInput(data)` で PTY に転送
6. クリップボード貼り付けボタン・Enter ボタンも提供

**アンマウント時**: `clearInterval` + `term.dispose()` でリソースを解放。
