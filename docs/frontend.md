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

### ストリーミング中

`streamBlocksRef` がすべてのブロックをリアルタイムで蓄積する。RAF (requestAnimationFrame) バッチ更新で `streamBlocks` state に反映される。

- `text` / `text_delta` イベント → 末尾が `text` ブロックなら追記、なければ新規追加
- `thinking` / `thinking_delta` イベント → 末尾が `thinking` ブロックなら追記、なければ新規追加
- `tool_use_start` イベント → `{ type: 'tool', toolCall: { isRunning: true } }` を末尾に追加
- `tool_input_delta` イベント → 該当 `index` の tool ブロックの `inputJson` を追記
- `tool_result` イベント → 該当 `toolUseId` の tool ブロックに `result` をセットし `isRunning: false`

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

アプリケーションのルートコンポーネント。会話リスト・選択中会話・メッセージ一覧の state を保持し、API 呼び出しと SSE ストリーミングを制御する。`useAuth` フックで認証状態を管理し、`AuthGuard` に渡す。

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

ストリーミング進行中を示すパルスドットアニメーションコンポーネント。

```tsx
export function TypingIndicator() { ... }
```

- 3つのドット（`span`要素）を横並びで表示
- 各ドットに `animate-pulse` クラスを付与
- ストークドディレイ: 0s / 0.2s / 0.4s（`animationDelay` スタイル）
- カラー: `var(--color-text-muted)`（Tokyo Night テーマに合わせたグレー系）
- `data-testid="typing-indicator"` でテスト可能

ChatView での使用:

| 状態 | 表示内容 |
| ---- | -------- |
| `isInProgress` + ブロックが空テキストのみ | TypingIndicator のみ（MessageBubble は非表示） |
| `isInProgress` + ブロックにコンテンツあり | コンテンツを通常描画 + 末尾に TypingIndicator |
| `isInProgress=false` | TypingIndicator 非表示 |

`isInProgress` = `isStreamingMsg || isPollingStreamingMsg || isRunning === true`（SSE・DBポーリング両方に適用）。

`isRunning` prop は App.tsx から `sending || hasStreamingMessage` として渡される。`hasStreamingMessage = messages.some(m => m.status === 'streaming')` により、ポーリング復帰時の `isPolling` 更新タイミングのズレ（race condition）でも TypingIndicator が確実に表示される。

### `ChatView`

選択中会話のメッセージ一覧と入力欄を表示する。メッセージ追加時に最下部へ自動スクロールする (`messagesEndRef`)。アシスタントメッセージは `AssistantBlock` の配列を順番にレンダリングする。

**Props**

| 名前            | 型                       | 用途                                         |
| --------------- | ------------------------ | -------------------------------------------- |
| `messages`      | `Message[]`              | 表示するメッセージ一覧                       |
| `onSend`        | `(content) => void`      | 送信ハンドラ                                 |
| `isStreaming`   | `boolean`                | SSE 受信中フラグ                             |
| `isPolling`     | `boolean \| undefined`   | ポーリング中フラグ（DB駆動状態管理）         |
| `isRunning`     | `boolean \| undefined`   | 実行中フラグ（`sending \|\| hasStreamingMessage`）TypingIndicator 表示制御用 |
| `streamMeta`    | `StreamMeta \| null`     | モデル・コスト・所要時間などのメタ情報       |
| `hookEvents`    | `SSEHookEvent[]`         | フックイベント一覧（フックイベントパネル用） |
| `streamBlocks`  | `AssistantBlock[]`       | ストリーミング中のブロック列                 |
| `input`         | `string`                 | テキストエリアの入力値                       |
| `onInputChange` | `(value) => void`        | 入力値変更ハンドラ                           |
| `onHamburger`   | `() => void`             | モバイルでサイドバーを開くハンドラ           |

**メッセージ表示の優先順位**

1. `isStreaming && isLast && role === 'assistant'` → SSE ライブストリーミング（`streamBlocks` を使用）
2. `isPolling && msg.status === 'streaming'` → DBポーリングストリーミング（`message_data.content_blocks` を使用、ストリーミングアニメーション付き）
3. `msg.status === 'error'` → エラーバッジ表示（「エラーが発生しました」）
4. それ以外 → DB復元レンダリング（完了メッセージ）

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
| `selectedId`       | `useState<string \| null>`  | 選択中会話 ID                                            |
| `messages`         | `useState<Message[]>`       | 現在の会話のメッセージ一覧                               |
| `input`            | `useState<string>`          | テキストエリアの入力値                                   |
| `sending`          | `useState<boolean>`         | SSE 送信中フラグ                                         |
| `sidebarOpen`      | `useState<boolean>`         | モバイルでのサイドバー開閉状態                           |
| `streamMeta`       | `useState<StreamMeta\|null>` | ストリーミング中のモデル・コスト等のメタ情報            |
| `hookEvents`       | `useState<SSEHookEvent[]>`  | 受信したフックイベント一覧                               |
| `streamBlocks`     | `useState<AssistantBlock[]>` | ストリーミング中のブロック列 (RAF 更新で反映)           |
| `rafIdRef`         | `useRef<number>`            | `requestAnimationFrame` ID (重複スケジュール防止)        |
| `streamMetaRef`    | `useRef<StreamMeta>`        | メタ情報の最新値を保持するバッファ (RAF 間の参照用)      |
| `hookEventsRef`    | `useRef<SSEHookEvent[]>`    | フックイベントの最新値バッファ (finally で参照)          |
| `streamBlocksRef`  | `useRef<AssistantBlock[]>`  | ブロック列の最新値バッファ (RAF バッチ更新用)            |

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

## SSE 受信フロー（sendMessage の動作）

1. `App.tsx` の `handleSend` が呼ばれると、ユーザーメッセージと空のアシスタントメッセージを即座に `messages` に追加する。
2. `api/client.ts` の `sendMessage` 関数が `POST /api/conversations/{id}/messages` を `fetch` で呼び出す。
3. レスポンスボディを `ReadableStream` として読み取り、`TextDecoder` で SSE テキストにデコードする。
4. `\n\n` 区切りで各 SSE イベントを分割し、`data: {...}` の JSON をパースする。
5. イベントタイプに応じた処理:
   - `type: "text"` / `type: "text_delta"` → `streamBlocksRef` の末尾 text ブロックに追記（なければ新規）し RAF スケジュール。
   - `type: "thinking"` / `type: "thinking_delta"` → `streamBlocksRef` の末尾 thinking ブロックに追記（なければ新規）し RAF スケジュール。
   - `type: "tool_use_start"` → `streamBlocksRef` に `{ type: 'tool', toolCall: { isRunning: true } }` を追加し RAF スケジュール。
   - `type: "tool_input_delta"` → `streamBlocksRef` の該当 `index` の tool ブロックの `inputJson` に追記し RAF スケジュール。
   - `type: "tool_result"` → `streamBlocksRef` の該当 `toolUseId` の tool ブロックに `result` をセットし `isRunning: false`、RAF スケジュール。
   - `type: "init"` → `streamMetaRef` にモデル名・セッション ID を設定し `streamMeta` を更新する。
   - `type: "rate_limit"` → `streamMetaRef` にレートリミット状態を設定し `streamMeta` を更新する。
   - `type: "cost"` → `streamMetaRef` にコスト・所要時間を設定し `streamMeta` を更新する。
   - `type: "hook_event"` → `hookEventsRef` および `hookEvents` に追加する。
   - `type: "done"` / `type: "error"` → 現状の UI では特別処理なし。
6. ストリーミング完了後 (`finally` ブロック):
   - `streamBlocksRef` から `finalText`（text ブロックの結合）と `toolCallsList` を抽出する。
   - thinking または tool ブロックが存在する場合、`metadata.content_blocks` に全ブロックを保存する。
   - `metadata.tool_calls` に ToolCallData リストを保存する。
   - `sending` を `false` にして会話リストを更新する。

---

## content_blocks 保存（ストリーミング完了時）

ストリーミング完了後の `finally` ブロックで、ブロック情報を `metadata` に保存する。

- **保存条件**: `finalBlocks` に `thinking` または `tool` ブロックが含まれる場合。
- **`metadata.content_blocks`**: ブロック順序を保持した配列。
  ```ts
  [
    { type: 'text', content: '...' },
    { type: 'thinking', content: '...' },
    { type: 'tool_use', tool_use_id: '...' },
  ]
  ```
- **`metadata.tool_calls`**: ToolCallData の配列（`tool_use_id`, `tool_name`, `input_json`, `result`）。
- DB 復元時は `content_blocks` を使って順序通りのブロック列を再構築する。

---

## OpenAPI 生成型の使用

- **`api/schema.d.ts`**: `openapi-typescript` で `apps/openapi/openapi.yaml` から自動生成。**手動編集禁止**。
- **`api/client.ts`**: `components['schemas'][...]` 型エイリアスを定義し、`openapi-fetch` ベースの API クライアントを実装。手書きの型定義を廃止し、生成型に統一。
  - `ToolCallData`: `components['schemas']['ToolCallData']` のエイリアス
  - 各 SSE イベント型: `components['schemas']['SSE*Event']` のエイリアス
- **`ChatView.tsx`**: `StoredToolCall` 型を廃止し、`ToolCallData` を `api/client.ts` からインポートして使用。

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

## フックイベントパネル

ストリーミング中に受信した `hook_event` をアシスタントメッセージのメタデータ行に `<details>` 要素で折りたたみ表示する。

- `msgHookEvents.length > 0` のとき表示される。
- `<summary>` に件数 (`▸ Hook Events (N)`) を表示。
- 展開すると各イベントを `subtype — hook_name` 形式で一覧表示する。

---

## サイドバースピナー制御

### 楽観的 status 更新 (handleSend)

`handleSend` の冒頭で、送信対象会話の status を即座に `'running'` に更新する。
これにより、バックエンドが DB を更新するより先にサイドバーのスピナーが表示される。

```ts
// App.tsx handleSend より
setConversations(prev =>
  prev.map(c => c.id === selectedId ? { ...c, status: 'running' as const } : c)
);
```

送信完了後の `refreshConversations()` で DB の実際の値（`'completed'`）に戻る。

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
  intervalMs?: number;  // デフォルト 2000ms
}
```

### 動作仕様

| 状態 | 動作 |
| ---- | ---- |
| `status === 'running'` | 毎ポーリングで `onMessages(全メッセージ)` を呼ぶ |
| `status === 'completed'` | `onMessages(全メッセージ)` を呼んだ後に `onCompleted()` を呼ぶ、ポーリング停止 |
| エラー発生時 | 一時エラーは無視してポーリング継続 |

### App.tsx での使用

```ts
useConversationPoller({
  conversationId: isPolling ? selectedId : null,
  isRunning: isPolling,
  onMessages: (msgs) => {
    setMessages(msgs);  // 全置換（差分でなく全上書き）
    if (msgs.length > 0) {
      lastMessageIdRef.current = msgs[msgs.length - 1].id;
    }
  },
  onCompleted: () => {
    setIsPolling(false);
    refreshConversations();
  },
});
```

### ChatView のストリーミング表示拡張

ポーリング中に `msg.status === 'streaming'` のメッセージを受け取った場合（`isPollingStreamingMsg`）:
- `message_data.content_blocks` があればDBの部分コンテンツを表示
- `content_blocks` が空なら空テキストブロックを表示
- ストリーミングアニメーション（カーソル）を付与する
- `text` / `thinking` ブロックはそのまま表示
- `tool_use` ブロックは `message_data.tool_calls` の `tool_use_id` マップから復元し `ToolCallCard` で表示（`isRunning: true`）
  - `tool_calls` にエントリがない場合（バッチ保存前の未確定状態）はそのブロックをスキップ
- メッセージバブルの下部にパルスインジケータ（`生成中...` テキスト + animate-pulse ドット）を表示
  - `isPollingStreamingMsg` のときのみ表示。`isPolling=false` の場合は非表示。

### バックエンド: dbMsgToAPI の status マッピング

`handler.go` の `dbMsgToAPI` 関数は `db.Message.Status` を `Message.Status` にマッピングする。
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

// URL パラメータから会話を初期化
const urlInitHandled = useRef<string | undefined>(undefined);
useEffect(() => {
  if (!urlId) {
    // / へのナビゲーション時: 選択状態をクリア
    setSelectedId(null);
    return;
  }
  if (selectedId === urlId) { urlInitHandled.current = urlId; return; }
  if (urlInitHandled.current === urlId) return;
  urlInitHandled.current = urlId;

  setSelectedId(urlId);
  getConversation(urlId).then(detail => {
    setMessages(detail.messages ?? []);
    if (detail.status === 'running') setIsPolling(true);
  }).catch(() => {
    setSelectedId(null);
    navigate('/');  // 存在しない ID → / へリダイレクト
  });
}, [urlId, selectedId, navigate]);
```

### nginx SPA フォールバック

`apps/frontend/nginx.conf.template` の `location /` ブロックで設定済み。
`/conversation/xxx` への直接アクセスも `index.html` を返す。

```nginx
location / {
    try_files $uri $uri/ /index.html;
}
```

---

## SSE 切断耐性設計

### 概要

セッション切替時・コンポーネントアンマウント時に、進行中の SSE ストリームを確実にクリーンアップする。

### `useSSEAbort` フック (`src/hooks/useSSEAbort.ts`)

SSE の AbortController ライフサイクルを管理するカスタムフック。

| 関数 | 動作 |
| ---- | ---- |
| `startStream(sessionId)` | 既存のコントローラを abort し、新しい `AbortController` を作成して `{ signal }` を返す |
| `switchSession(sessionId)` | 現在のコントローラを abort し、アクティブセッション ID を更新する |
| `isActiveSession(sessionId)` | 現在アクティブなセッション ID と一致するか返す |

コンポーネントアンマウント時は `useEffect` のクリーンアップで自動的に abort される。

### `sendMessage` の AbortSignal 対応 (`src/api/client.ts`)

`sendMessage(conversationId, content, onEvent, signal?)` に `signal` パラメータを追加。

- `fetch()` に `signal` を渡す
- `fetch` / `reader.read()` が `AbortError` を投げた場合は静かに return（throw しない）

### `App.tsx` の統合

`handleSend`:

1. `startStream(mySessionId)` で `signal` を取得し SSE 開始
2. コールバック内で `isActiveSession(mySessionId)` をチェック → 切替後は無視
3. `finally` ブロックでも `isActiveSession` を確認し、切替後は state を更新しない

`handleSelectConversation`:

1. 先頭で `switchSession(id)` を呼び出し、進行中の SSE を abort
2. ストリーミング state を全てリセット（`streamBlocks`, `streamMeta`, `hookEvents`, `sending`）
3. RAF をキャンセル

### リロード後の状態

バックエンドは `context.WithoutCancel` でフロントエンド切断後も処理を継続する。処理完了後に DB へ保存するため、ユーザーが再度そのセッションを選択すると完了済みのメッセージが表示される。処理完了前に選択した場合は既存メッセージ（user メッセージまで）が表示され、リロードで最新状態を確認できる。

---

## ツール使用可視化

ストリーミング中・DB 復元時ともに、アシスタントメッセージのブロック列に `ToolCallCard` が順番にレンダリングされる。

- ストリーミング中: `streamBlocks` の `type === 'tool'` ブロックとして表示される。
- DB 復元時: `metadata.content_blocks` から `type === 'tool_use'` エントリを `ToolCallData` に紐付けて再構築。
- `tool_use_start` → `tool_input_delta` → `tool_result` の順で状態が遷移する。
- ストリーミング完了後は `streamBlocks` がリセット (`setStreamBlocks([])`) され、履歴は `metadata.content_blocks` として `messages` に保持される。
