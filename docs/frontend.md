# Frontend ドキュメント

## 技術スタック

- React 19 + TypeScript
- Vite (ビルドツール)
- Tailwind CSS v4
- react-router-dom v7 (URLルーティング)
- ReactMarkdown + remark-gfm (Markdown レンダリング)
- react-syntax-highlighter (コードブロックシンタックスハイライト)
- @xterm/xterm (認証ターミナルエミュレータ)
- openapi-fetch (型安全 API クライアント)

---

## コンポーネント構成図

```
main.tsx
└─ BrowserRouter
   └─ AppAuthProvider (AppAuthContext)
      └─ Routes
         ├─ / → HomePage
         │       └─ UserMenu (右上アイコン+ドロップダウン)
         ├─ /login → LoginPage
         ├─ /chat → AppAuthGuard → ChatPage
         │           └─ AuthGuard
         │               ├─ [Agent ログイン済み] メインレイアウト
         │               │   ├─ Sidebar
         │               │   └─ AgentSelector / ChatView
         │               │       ├─ ThinkingAccordion (thinking ブロック)
         │               │       ├─ MessageBubble (text ブロック)
         │               │       └─ ToolCallCard (tool ブロック)
         │               └─ [Agent ログイン待ち] AuthTerminal
         ├─ /chat/:id → AppAuthGuard → ChatPage (同上)
         ├─ /settings/account → AppAuthGuard → AccountSettingsPage
         ├─ /settings/agents  → AppAuthGuard → AgentSettingsPage
         │                           └─ AgentSelector + AuthTerminal
         └─ /conversation/:id → Navigate → /chat/:id (後方互換リダイレクト)
```

---

## AssistantBlock 型とブロック分割表示

アシスタントの応答は `AssistantBlock` の配列として管理される。

```ts
export type AssistantBlock =
  | { type: 'thinking'; content: string }
  | { type: 'text';    content: string }
  | { type: 'tool';    toolCall: ToolCall }
```

### DB ポーリング中（streaming メッセージ）

`messages[].status === 'streaming'` のメッセージを受け取った場合:

- `message_data.content_blocks` があれば DB の部分コンテンツを表示
- `content_blocks` が空なら TypingIndicator のみ表示（空バブルは表示しない）
- `message_data.tool_calls` を `tool_use_id` でマップして ToolCallCard を表示（`isRunning: true`）

### DB 復元時 (新形式)

`metadata.content_blocks` にブロック順序が保存されている場合:

```ts
type ContentBlockEntry =
  | { type: 'thinking'; content: string }
  | { type: 'text'; content: string }
  | { type: 'tool_use'; tool_use_id: string }
```

`metadata.tool_calls` (ToolCallData[]) を `tool_use_id` でマップし、`tool_use` エントリを `ToolCall` に変換してブロック列を再構築する。

### DB 復元時 (旧形式)

`metadata.content_blocks` がない場合は `msg.content` を text ブロックとし、`metadata.tool_calls` を末尾に並べるフォールバック表示を行う。

---

## 各コンポーネントの役割

### `App.tsx`

アプリケーションのルートコンポーネント。`AppAuthProvider` でアプリ認証コンテキストを提供し、`Routes` で画面遷移を定義する。

**責務**

- `AppAuthProvider` でアプリ認証コンテキストをアプリ全体に提供
- `Routes` で 5 画面 + 後方互換リダイレクトを定義
- `AppAuthGuard` で `/chat`, `/settings/*` を保護

### `AppAuthContext` / `AppAuthProvider` / `useAppAuth`

アプリ認証（cc-tunnel アプリへのログイン）の状態管理を行う Context。

- **実装ファイル**: `src/contexts/AppAuthContext.tsx`
- `AppAuthProvider`: アプリ全体を包むプロバイダー。`sessionStorage` でトークンを永続化。
- `useAppAuth()`: 現在のアプリ認証状態と操作関数を返すフック

| 状態 / 操作 | 型 | 説明 |
|------------|-----|------|
| `user` | `AppUser \| null` | 現在のログインユーザー情報（null = 未ログイン） |
| `isLoading` | `boolean` | 認証状態確認中フラグ |
| `login(username)` | `Promise<void>` | アプリ認証でログイン（`POST /app-auth/login`）。サーバーが 32 バイト乱数を hex エンコードした 64 文字トークンを発行し、`sessionStorage` に保存される |
| `logout()` | `Promise<void>` | ログアウト（`POST /app-auth/logout`） |
| `updateNickname(name)` | `Promise<void>` | ニックネーム更新（`PATCH /app-auth/me`） |

### `AppAuthGuard`

アプリ認証が必要なルートを保護するゲートコンポーネント。

- **実装ファイル**: `src/components/AppAuthGuard.tsx`
- ローディング中: スピナー表示
- 未認証: `/login` にリダイレクト
- 認証済み: `children` を描画

### `CredentialGuard`

`/chat/:id` 入室時に credentials の登録・有効性を確認するガードコンポーネント。

- **実装ファイル**: `src/components/CredentialGuard.tsx`
- マウント時に `GET /credentials/status`（Bearer トークン付き）を呼び出し、`{ registered, isValid }` を確認
- `registered=false`: `/login/credentials?reason=missing&conversationId=<id>` にリダイレクト
- `isValid=false`: `/login/credentials?reason=expired&conversationId=<id>` にリダイレクト
- 確認完了まで: スピナー表示
- credentials が有効: `children` を描画（fail-open: エンドポイント呼び出し失敗時も描画）

**Bearer 認証ミドルウェア**: cc-tunnel API の保護エンドポイントはすべて `Authorization: Bearer <token>` ヘッダを検証する。`/app-auth/login` で発行された 64 文字 hex トークンを `sessionStorage` から取得して付与する（`src/api/credentials.ts` の `apiFetch` 参照）。

### `AuthGuard`

Agent 認証（Claude Code 認証）状態に応じて子コンポーネントの表示を切り替えるゲートコンポーネント。

- ローディング中: スピナー表示
- `loggedIn: true`: children (メインレイアウト) を描画
- `loginPending: true`: `AuthTerminal` を描画
- 未認証: ログインボタン画面を描画

### `UserMenu`

ホーム・チャット画面右上のユーザーアイコン + ドロップダウンメニューコンポーネント。

- **実装ファイル**: `src/components/UserMenu.tsx`
- 未ログイン時: 匿名アイコン + 「ログイン」メニュー項目
- ログイン済み: ユーザー名アイコン + 「アカウント設定」「Agentログイン設定」「ログアウト」

### `AgentSelector`

Agent 選択 UI コンポーネント。会話開始時に表示。

- **実装ファイル**: `src/components/AgentSelector.tsx`
- **Claude Code**: 対応済み（有効ボタン）
- **GitHub Copilot**: 将来対応（グレーアウト表示）
- **Cursor CLI**: 将来対応（グレーアウト表示）

### ページ一覧

| ページ | ファイル | ルート | 説明 |
|--------|---------|--------|------|
| `HomePage` | `pages/HomePage.tsx` | `/` | ホーム。チャットボタン + UserMenu |
| `LoginPage` | `pages/LoginPage.tsx` | `/login` | モック認証フォーム（ユーザー名入力で認証） |
| `ChatPage` | `pages/ChatPage.tsx` | `/chat`, `/chat/:id` | 会話画面（既存チャット機能を統合） |
| `AccountSettingsPage` | `pages/AccountSettingsPage.tsx` | `/settings/account` | ニックネーム設定 |
| `AgentSettingsPage` | `pages/AgentSettingsPage.tsx` | `/settings/agents` | Claude Code 認証 + Agent 一覧 |
| `CredentialsLoginPage` | `pages/CredentialsLoginPage.tsx` | `/login/credentials` | credentials 再ログインフロー（PTY 認証 + 自動/手動完了ボタン） |

#### `CredentialsLoginPage` — 認証完了トリガー（dual-trigger）

`CredentialsLoginPage` は credentials 再ログインフローを担う。認証完了の検知は **dual-trigger** 方式:

1. **自動検知**: `AuthTerminal` の `onTextOutput` コールバックで PTY 出力を監視し、`/Login successful|Logged in|authentication successful/i` パターンを検出したら自動で `POST /credentials/relogin/finalize` を呼び出す
2. **手動ボタン**: PTY フェーズ中に「完了」ボタンを表示。ユーザーが手動でクリックしても finalize を実行できる

どちらのトリガーも `finalizedRef` フラグで二重実行を防止する。finalize 成功後は `/chat/<conversationId>` にリダイレクトされる。

### `Sidebar`

会話リストの表示・選択・削除、新規会話作成ボタン、認証情報表示・ログアウトボタンを提供するサイドバー。モバイルではオーバーレイ形式でスライドイン表示される。

#### スピナー表示仕様

会話リストの各アイテムで、`conversation.status === 'running'` の場合にタイトル左側にスピナーを表示する。

| status | スピナー |
| ------ | -------- |
| `running` | 表示（`animate-spin` CSS アニメーション） |
| `idle` | 非表示 |
| `completed` | 非表示 |

スピナーは `h-3 w-3` の円形ボーダー要素（`border-[var(--color-accent)] border-t-transparent animate-spin`）で、Tailwind CSS の `animate-spin` クラスで回転アニメーションを実現する。

### `TypingIndicator`

処理進行中を示すシマーアニメーションコンポーネント。

```tsx
export function TypingIndicator() {
  return (
    <div className="flex items-center px-1 py-1" data-testid="typing-indicator">
      <span className="typing-shimmer text-sm font-medium">進行中...</span>
    </div>
  );
}
```

- 「進行中...」テキストを表示
- `typing-shimmer` CSS クラスによるグラデーションシマーアニメーション
- `data-testid="typing-indicator"` でテスト可能

ChatView での使用:

| 状態 | 表示内容 |
| ---- | -------- |
| `isInProgress` + `content_blocks` が空 | TypingIndicator のみ（MessageBubble は非表示） |
| `isInProgress` + `content_blocks` にコンテンツあり | コンテンツを通常描画 + 末尾に TypingIndicator |
| `isInProgress=false` | TypingIndicator 非表示 |

`isInProgress` = `isRunning === true`。

`isRunning` は ChatView 内部で `sending || isPolling || messages.some(m => m.status === 'streaming')` として算出される。`messages.some(m => m.status === 'streaming')` により、ポーリング復帰時の `isPolling` 更新タイミングのズレ（race condition）でも TypingIndicator が確実に表示される。

### `ChatView`

選択中会話のメッセージ一覧と入力欄を表示する。メッセージ取得・送信・ポーリングをすべて内部で自己完結する。メッセージ追加時に最下部へ自動スクロールする (`messagesEndRef`)。アシスタントメッセージは `AssistantBlock` の配列を順番にレンダリングする。

**責務（リファクタ後）**

- `conversationId` 変更時に messages をクリアし `getConversation` で再取得
- `messages`, `sending`, `isPolling` state を内部管理
- `isRunning = sending || isPolling || messages.some(m => m.status === 'streaming')`
- `useConversationPoller` を内部で呼んでポーリング制御
- メッセージ送信（`handleSend`）を内部に実装
- TypingIndicator・content_blocks・ToolCallCard 等の全表示責任

**Props**

| 名前                    | 型                   | 用途                                         |
| ----------------------- | -------------------- | -------------------------------------------- |
| `conversationId`        | `string \| null`     | 表示する会話の ID（null = 未選択）           |
| `onConversationUpdate`  | `() => void`         | 会話完了時に App 側の conversations を更新  |
| `onHamburger`           | `() => void`         | モバイルでサイドバーを開くハンドラ           |

**内部 state**

| 名前        | 型                    | 用途                              |
| ----------- | --------------------- | --------------------------------- |
| `messages`  | `Message[]`           | 表示するメッセージ一覧            |
| `sending`   | `boolean`             | 送信中フラグ                      |
| `isPolling` | `boolean`             | ポーリング中フラグ                |
| `input`     | `string`              | テキストエリアの入力値            |
| `isRunning` | `boolean`（derived）  | `sending \|\| isPolling \|\| messages.some(m => m.status === 'streaming')` |

**メッセージ表示の優先順位**

1. `isPolling && msg.status === 'streaming'` → DBポーリングストリーミング（`message_data.content_blocks` を使用、ストリーミングアニメーション付き）
2. `msg.status === 'error'` → エラーバッジ表示（「エラーが発生しました」）
3. それ以外 → DB復元レンダリング（完了メッセージ）

**streaming メッセージのレンダリングロジック**

```
streaming メッセージの表示優先順位:
1. content_blocks があれば通常通りレンダリング
2. TypingIndicator は content_blocks の「後ろ」に追加表示（末尾インジケータ）
3. content_blocks が空（まだバッチ保存前）の場合のみ TypingIndicator 単体表示

isInProgress = isRunning === true
```

### `MessageBubble`

1件のテキストブロックを描画するコンポーネント。ユーザー/アシスタントでスタイルを切り替える。アシスタントメッセージは ReactMarkdown でレンダリングし、コードブロックは react-syntax-highlighter でハイライト。

- `textContent` prop が渡された場合はそちらを優先して表示（ストリーミング中の部分テキスト）。
- `ThinkingAccordion` も同ファイルからエクスポートされる。

### `ThinkingAccordion`

thinking ブロック 1 件を折りたたみ UI で表示するコンポーネント (`MessageBubble.tsx` からエクスポート)。

- 閉じた状態: `🤔` + テキスト先頭 40 文字プレビュー + `▸`
- 開いた状態: `🤔 思考過程` + 全テキスト (最大高さ 256px、スクロール可)

### `ToolCallCard`

ツール呼び出し 1 件を表示するカードコンポーネント。

- `open` (`useState<boolean>`) で展開/折りたたみ状態を管理する。
- **閉じた状態 (ヘッダー行)**: アイコン + ツール名（太字）+ `getInputPreview()` によるインプットプレビュー（70 文字制限）+ 実行状態インジケータ。
- **閉じた状態 (プレビュー行)**:
  - 実行中 (`isRunning: true`、結果なし): `実行中...` テキスト
  - 完了 (`isRunning: false`、結果あり): `getResultPreview()` による先頭 4 行プレビュー (`line-clamp-4`)
- **開いた状態**: 全 `inputJson` + 全 `result` を `pre` タグで表示（最大高さあり、スクロール可）。
- アイコンは `TOOL_ICONS` テーブルで管理し、未定義ツールは `🔧` にフォールバック。

**`getInputPreview(toolName, inputJson)` のフィールド抽出ルール**

| toolName | 抽出フィールド |
| -------- | -------------- |
| Bash | `command` |
| Read / Edit / Write | `file_path` |
| Glob | `pattern` |
| Grep | `pattern` |
| WebSearch | `query` |
| WebFetch | `url` |
| その他 | 最初のキーの値 |

### `MessageInput`

テキストエリアと送信ボタンを含む入力欄。`Enter` で送信、`Shift+Enter` で改行。テキスト量に応じて高さが自動調整される。モバイルではハンバーガーボタンでサイドバーを開ける。

### `AuthTerminal`

Claude CLI の認証フロー (OAuth) 用のターミナルエミュレータ。`@xterm/xterm` を埋め込み、バックエンド `/auth/output` エンドポイントを 250ms ポーリングして出力をターミナルに書き込む。認証 URL が検出された場合はリンクボタンとして表示する。

---

## 主要 state 一覧

### `App.tsx`

App.tsx はルーティング定義のみを担う。会話管理・サイドバー管理は `ChatPage` に移動済み。

### `AppAuthContext.tsx`

| 名前        | 種別                         | 用途                           |
| ----------- | ---------------------------- | ------------------------------ |
| `user`      | `useState<AppUser \| null>`  | ログイン中ユーザー情報         |
| `isLoading` | `useState<boolean>`          | 認証確認中フラグ               |
| `token`     | `sessionStorage` 永続化      | 認証トークン                   |

### `ChatPage.tsx`

| 名前               | 種別                        | 用途                                                     |
| ------------------ | --------------------------- | -------------------------------------------------------- |
| `conversations`    | `useState<Conversation[]>`  | 会話リスト                                               |
| `selectedId`       | derived（URL params）       | 選択中会話 ID（`useParams` から直接導出、state ではない）|
| `sidebarOpen`      | `useState<boolean>`         | モバイルでのサイドバー開閉状態                           |

> `messages`, `sending`, `isPolling`, `isRunning`, `input` は ChatView に移動済み。

### `useAuth.ts`

| 名前        | 種別                           | 用途                           |
| ----------- | ------------------------------ | ------------------------------ |
| `status`    | `useState<AuthStatus \| null>` | 認証状態オブジェクト           |
| `isLoading` | `useState<boolean>`            | API 呼び出し中フラグ           |
| `pollRef`   | `useRef<setInterval>`          | ログイン待ちポーリングタイマー |

### `AuthTerminal.tsx`

| 名前            | 種別                       | 用途                                                 |
| --------------- | -------------------------- | ---------------------------------------------------- |
| `terminalRef`   | `useRef<HTMLDivElement>`   | xterm マウント先 DOM                                 |
| `xtermRef`      | `useRef<Terminal>`         | xterm インスタンス                                   |
| `cursorRef`     | `useRef<number>`           | `/auth/output` ポーリングカーソル (取得済みバイト数) |
| `pollRef`       | `useRef<setInterval>`      | ポーリングタイマー                                   |
| `authUrl`       | `useState<string \| null>` | 出力から検出した認証 URL                             |
| `fullOutputRef` | `useRef<string>`           | URL 検出用の出力累積バッファ                         |





---

## OpenAPI 生成型の使用

- **`api/schema.d.ts`**: `openapi-typescript` で `apps/openapi/openapi.yaml` から自動生成。**手動編集禁止**。
- **`api/client.ts`**: `components['schemas'][...]` 型エイリアスを定義し、`openapi-fetch` ベースの API クライアントを実装。手書きの型定義を廃止し、生成型に統一。
  - `ToolCallData`: `components['schemas']['ToolCallData']` のエイリアス
  - `Message`: `components['schemas']['Message']` のエイリアス
  - `Conversation`: `components['schemas']['Conversation']` のエイリアス
- **`ChatView.tsx`**: `ToolCallData` を `api/client.ts` からインポートして使用。

---

## 認証フロー

### アプリ認証フロー（AppAuth）

```
LoginPage
  └─ useAppAuth().login(username)
        │
        ▼
POST /app-auth/login
  └─ サーバーが crypto/rand 32 バイトを hex エンコードし 64 文字トークンを発行
        │
        ▼
AppAuthContext: user が設定される、token を sessionStorage に保存
  └─ LoginPage が /chat にリダイレクト

AppAuthGuard
  ├─ ローディング中: スピナー表示
  ├─ 認証済み: children 描画
  └─ 未認証: /login にリダイレクト
```

### Credentials ガードフロー（CredentialGuard）

`/chat/:id` にアクセスすると `CredentialGuard` が挿入され、credentials チェックを行う。

```
/chat/:id アクセス
  └─ CredentialGuard マウント
        │
        ▼
GET /credentials/status  (Authorization: Bearer <token>)
  └─ { registered: bool, isValid: bool } を返す
        │
        ├─ registered=false → /login/credentials?reason=missing&conversationId=<id>
        ├─ isValid=false    → /login/credentials?reason=expired&conversationId=<id>
        └─ OK              → children (ChatPage) を描画
```

### Agent 認証フロー（Claude Code）

```
useAuth (ポーリング)
  └─ getAuthStatus() → /auth/status
        │
        ▼
AuthGuard
  ├─ loggedIn: true  → メインレイアウト表示
  ├─ loginPending: true → AuthTerminal 表示
  │       └─ getAuthOutput() を 250ms ポーリング
  │               → xterm に書き込み
  │               → 認証 URL 検出 → リンクボタン表示
  └─ 未認証 → ログインボタン表示
               └─ onClick: initiateLogin() → /auth/login
                       → useAuth がポーリング開始 (3000ms)
                       → loginPending になる → AuthTerminal へ遷移
```

### ログアウト

- **アプリ認証**: `UserMenu` からログアウト → `useAppAuth().logout()` → `POST /app-auth/logout`
- **Agent 認証**: サイドバーのログアウトボタンから `useAuth.logout()` → `POST /auth/logout`

### ログインキャンセル

AuthTerminal 画面のキャンセルボタンから `useAuth.cancelLogin()` → `POST /auth/cancel` を呼び出す。



---

## サイドバースピナー制御

### conversations ポーリング (useConversationListPoller)

`status === 'running'` の会話がある間、3 秒ごとに `listConversations` を呼んで
サイドバーの会話リストを最新状態に保つ。

```ts
// hooks/useConversationListPoller.ts
export function useConversationListPoller({
  hasRunning,
  onPoll,
  intervalMs = 3000,
}: {
  hasRunning: boolean;
  onPoll: () => void;
  intervalMs?: number;
}): void {
  useEffect(() => {
    if (!hasRunning) return;
    const id = setInterval(onPoll, intervalMs);
    return () => clearInterval(id);
  }, [hasRunning, onPoll, intervalMs]);
}
```

**App.tsx での使用:**

```ts
const hasRunning = conversations.some(c => c.status === 'running');
useConversationListPoller({
  hasRunning,
  onPoll: refreshConversations,
});
```

`hasRunning` が `false` になった瞬間にポーリングが停止する。
DB から `status === 'completed'` が返ると `conversations` が更新され `hasRunning` が `false` になる。

---

## DB駆動状態管理 (useConversationPoller)

### 概要

`useConversationPoller` フックは、バックエンド処理が進行中の会話を定期ポーリングしてメッセージを全置換更新する。SSE では捕捉できない「既存メッセージのDB更新（streaming中のcontent_blocks更新）」に対応するために設計された。

### インターフェース

```ts
export interface UseConversationPollerOptions {
  conversationId: string | null;
  isRunning: boolean;
  onMessages: (messages: Message[]) => void;  // 全メッセージを受け取る（差分でなく全置換）
  onCompleted: () => void;
  intervalMs?: number;  // デフォルト 1000ms
}
```

### 動作仕様

| 状態 | 動作 |
| ---- | ---- |
| `status === 'running'` | 毎ポーリングで `onMessages(全メッセージ)` を呼ぶ |
| `status === 'completed'` | `onMessages(全メッセージ)` を呼んだ後に `onCompleted()` を呼ぶ、ポーリング停止 |
| エラー発生時 | 一時エラーは無視してポーリング継続 |

### ChatView での使用

```ts
// ChatView.tsx 内
useConversationPoller({
  conversationId: isPolling ? conversationId : null,
  isRunning: isPolling,
  onMessages: (msgs) => setMessages(msgs),  // 全置換（差分でなく全上書き）
  onCompleted: () => {
    setIsPolling(false);
    onConversationUpdate?.();  // App 側の conversations 一覧を更新
  },
  intervalMs: 1000,
});
```

### ChatView のストリーミング表示拡張

ポーリング中に `msg.status === 'streaming'` のメッセージを受け取った場合（`isPollingStreamingMsg`）:
- `message_data.content_blocks` があればDBの部分コンテンツを表示
- `content_blocks` が空なら TypingIndicator のみ表示（空バブルは表示しない）
- ストリーミングアニメーション（カーソル）を付与する
- `text` / `thinking` ブロックはそのまま表示
- `tool_use` ブロックは `message_data.tool_calls` の `tool_use_id` マップから復元し `ToolCallCard` で表示（`isRunning: true`）
  - `tool_calls` にエントリがない場合（バッチ保存前の未確定状態）はフォールバック ToolCallCard を表示
- TypingIndicator は `isRunning === true` のときのみ表示（`isPolling` に依存しない）
  - `isRunning = sending || isPolling || hasStreamingMessage`

### バックエンド: コンストラクタ関数による status マッピング

`mapping.go` の `newMessage()` 関数は `db.Message.Status` を `Message.Status` にマッピングする。
これにより、`GetConversation` で取得したメッセージの `status` フィールドが正しくフロントエンドに返される。
`status` が空文字の場合（旧データ）は `Status` フィールドを設定しない（omitempty）。

### ChatView のエラー表示

`msg.status === 'error'` のメッセージは赤いエラーバッジ（「エラーが発生しました」）を表示する。

---

## URLルーティング

### 概要

React Router v7 (`react-router-dom`) による 5 画面構成のルーティング。アプリ認証（AppAuthGuard）と Agent 認証（AuthGuard）の 2 層ガードで保護される。

### ルート定義

```tsx
// main.tsx: BrowserRouter でアプリ全体をラップ
<BrowserRouter>
  <App />
</BrowserRouter>

// App.tsx: AppAuthProvider + Routes
<AppAuthProvider>
  <Routes>
    {/* 公開ルート */}
    <Route path="/" element={<HomePage />} />
    <Route path="/login" element={<LoginPage />} />

    {/* 保護ルート: AppAuthGuard */}
    <Route path="/chat" element={<AppAuthGuard><ChatPage /></AppAuthGuard>} />
    <Route path="/chat/:id" element={<AppAuthGuard><ChatPage /></AppAuthGuard>} />
    <Route path="/settings/account" element={<AppAuthGuard><AccountSettingsPage /></AppAuthGuard>} />
    <Route path="/settings/agents" element={<AppAuthGuard><AgentSettingsPage /></AppAuthGuard>} />

    {/* 後方互換: /conversation/:id → /chat/:id */}
    <Route path="/conversation/:id" element={<ConversationRedirect />} />
  </Routes>
</AppAuthProvider>
```

### URL 設計

| URL パターン | 画面 | 認証要否 |
| --- | --- | --- |
| `/` | ホーム | 不要 |
| `/login` | ログイン | 不要（ログイン済みなら `/chat` へ） |
| `/chat` | チャット（会話未選択） | AppAuth 必須 |
| `/chat/{id}` | チャット（会話選択済み） | AppAuth 必須 |
| `/settings/account` | アカウント設定 | AppAuth 必須 |
| `/settings/agents` | Agent ログイン設定 | AppAuth 必須 |
| `/conversation/{id}` | （後方互換）→ `/chat/{id}` へリダイレクト | - |

### 動作仕様

| 操作 | URL 変化 |
| --- | --- |
| サイドバーで会話をクリック | `→ /chat/{id}` |
| 新規会話作成 | `→ /chat/{新ID}` |
| 選択中会話を削除 | `→ /chat` |
| ブラウザの戻る/進む | React Router が popstate を自動管理 |
| 存在しない ID に直接アクセス | `→ /chat`（リダイレクト） |
| 未認証で `/chat` にアクセス | `→ /login` |
| ログイン済みで `/login` にアクセス | `→ /chat` |

### nginx SPA フォールバック

`apps/frontend/nginx.conf.template` の `location /` ブロックで設定済み。
直接 URL アクセスも `index.html` を返す。

```nginx
location / {
    try_files $uri $uri/ /index.html;
}
```

### nginx reverse proxy（/api/ → cc-tunnel API）

frontend と cc-tunnel API は別々の Cloud Run サービスであり、ブラウザから直接 cc-tunnel URL に
リクエストを送ると CORS や認証の問題が生じる。
同オリジン化（候補B）として、frontend の nginx が `/api/*` を cc-tunnel API に reverse proxy する。

**仕組み:**

1. Terraform (`frontend.tf`) が `API_UPSTREAM` env として cc-tunnel の Cloud Run URI を注入
2. `docker-entrypoint.sh` が `BACKEND_URL` env から `window.__ENV__.BACKEND_URL` を `/usr/share/nginx/html/env.js` に書き出す
3. nginx の `envsubst` テンプレート機能が `nginx.conf.template` の `$API_UPSTREAM` を展開して nginx.conf を生成
4. nginx が `/api/*` リクエストを cc-tunnel URI に転送する

```nginx
# SSE ストリーミング専用（conversations/{id}/messages）
location ~ ^/api/conversations/[^/]+/messages {
    rewrite ^/api(.*)$ $1 break;
    proxy_pass $API_UPSTREAM;
    proxy_http_version 1.1;
    proxy_buffering off;
    proxy_cache off;
    proxy_set_header X-Accel-Buffering no;
    proxy_set_header Connection '';
    proxy_set_header Host $host;
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

**フロントエンドコード側:** `src/api/client.ts` は `window.__ENV__?.BACKEND_URL ?? '/api'` を baseUrl として使用。
`BACKEND_URL="/api"` が注入されることで、API リクエストは常に同一オリジンの `/api/` パスに向かう。

**CORS 不要の理由:** ブラウザから見ると frontend と API が同一オリジン（nginx が同一 Port で両方を提供）のため
preflight が発生しない。

### Phase 2: LB 経由配信

frontend Cloud Run は Global HTTPS LB の serverless NEG 経由でアクセス。
LB の default backend が frontend を配信するため、nginx の /api/ proxy_pass は使用されない。
ローカル開発環境（compose.yaml）では引き続き nginx の /api/ proxy_pass が動作する。

- ingress: `INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER`（LB 経由のみ）
- SPA ルーティング: nginx の `try_files $uri $uri/ /index.html` で対応（変更なし）


