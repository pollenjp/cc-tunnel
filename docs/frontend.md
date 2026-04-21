# Frontend ドキュメント

## 技術スタック

- React 18 + TypeScript
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
App.tsx
└─ AuthGuard
   ├─ [ログイン済み] メインレイアウト
   │   ├─ Sidebar
   │   └─ main
   │       └─ ChatView
   │           ├─ (assistant メッセージごとにブロック順にレンダリング)
   │           │   ├─ ThinkingAccordion (thinking ブロック)
   │           │   ├─ MessageBubble (text ブロック)
   │           │   └─ ToolCallCard (tool ブロック)
   │           └─ MessageInput
   └─ [ログイン待ち] 認証画面
       └─ AuthTerminal
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

アプリケーションのルートコンポーネント。会話リスト管理・URL ベースの会話選択・サイドバー制御を担う。メッセージ管理・送信・ポーリングは ChatView に委譲。`useAuth` フックで認証状態を管理し、`AuthGuard` に渡す。

**責務（リファクタ後）**

- URL ルーティング（React Router）
- 会話一覧取得・更新（`conversations` state）
- 会話選択・新規作成・削除
- サイドバー開閉管理
- ChatView に `conversationId` を渡すだけ

### `AuthGuard`

認証状態に応じて子コンポーネントの表示を切り替えるゲートコンポーネント。

- ローディング中: スピナー表示
- `loggedIn: true`: children (メインレイアウト) を描画
- `loginPending: true`: `AuthTerminal` を描画
- 未認証: ログインボタン画面を描画

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

サイドバーのログアウトボタンから `useAuth.logout()` → `POST /auth/logout` を呼び出す。API Key 認証の場合はログアウトボタンは表示されない。

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

React Router v7 (`react-router-dom`) による URL ベースの会話選択。会話 ID が URL に含まれるため、ブラウザの戻る/進む・ページリロード・URL 直接アクセスに対応する。

### ルート定義

```tsx
// main.tsx: BrowserRouter でアプリ全体をラップ
<BrowserRouter>
  <App />
</BrowserRouter>

// App.tsx: 2つのルートを定義
<Routes>
  <Route path="/" element={<AppContent />} />
  <Route path="/conversation/:id" element={<AppContent />} />
</Routes>
```

### URL 設計

| URL パターン | 表示 |
| --- | --- |
| `/` | 会話未選択（サイドバーから選択を促すプレースホルダー） |
| `/conversation/{id}` | 指定 ID の会話を選択・表示 |

### 動作仕様

| 操作 | URL 変化 |
| --- | --- |
| サイドバーで会話をクリック | `→ /conversation/{id}` |
| 新規会話作成 | `→ /conversation/{新ID}` |
| 選択中会話を削除 | `→ /` |
| ブラウザの戻る/進む | React Router が popstate を自動管理 |
| 存在しない ID に直接アクセス | `→ /`（リダイレクト） |

### 直接 URL アクセス時の初期化 (`AppContent` 内)

```tsx
const { id: urlId } = useParams<{ id: string }>();
const navigate = useNavigate();

// selectedId を URL から直接導出（setState-in-effect を避けるため）
const selectedId = urlId ?? null;

// URL直接アクセス時: 会話の存在確認（存在しなければ / へリダイレクト）
useEffect(() => {
  if (!urlId) return;
  getConversation(urlId).catch(() => {
    navigate('/');  // 存在しない ID → / へリダイレクト
  });
}, [urlId, navigate]);
```

`selectedId` は `useState` を使わず URL params から直接導出する。これにより `react-hooks/set-state-in-effect` ESLint ルールに準拠する。メッセージのロードは ChatView が `conversationId` の変化を監視して行う。

### nginx SPA フォールバック

`apps/frontend/nginx.conf.template` の `location /` ブロックで設定済み。
`/conversation/xxx` への直接アクセスも `index.html` を返す。

```nginx
location / {
    try_files $uri $uri/ /index.html;
}
```




