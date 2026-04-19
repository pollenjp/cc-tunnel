# Frontend ドキュメント

## 技術スタック

- React 18 + TypeScript
- Vite (ビルドツール)
- Tailwind CSS v4
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

### `ChatView`

選択中会話のメッセージ一覧と入力欄を表示する。メッセージ追加時に最下部へ自動スクロールする (`messagesEndRef`)。アシスタントメッセージは `AssistantBlock` の配列を順番にレンダリングする。

**Props**

| 名前            | 型                       | 用途                                         |
| --------------- | ------------------------ | -------------------------------------------- |
| `messages`      | `Message[]`              | 表示するメッセージ一覧                       |
| `onSend`        | `(content) => void`      | 送信ハンドラ                                 |
| `isStreaming`   | `boolean`                | SSE 受信中フラグ                             |
| `streamMeta`    | `StreamMeta \| null`     | モデル・コスト・所要時間などのメタ情報       |
| `hookEvents`    | `SSEHookEvent[]`         | フックイベント一覧（フックイベントパネル用） |
| `streamBlocks`  | `AssistantBlock[]`       | ストリーミング中のブロック列                 |
| `input`         | `string`                 | テキストエリアの入力値                       |
| `onInputChange` | `(value) => void`        | 入力値変更ハンドラ                           |
| `onHamburger`   | `() => void`             | モバイルでサイドバーを開くハンドラ           |

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

## ツール使用可視化

ストリーミング中・DB 復元時ともに、アシスタントメッセージのブロック列に `ToolCallCard` が順番にレンダリングされる。

- ストリーミング中: `streamBlocks` の `type === 'tool'` ブロックとして表示される。
- DB 復元時: `metadata.content_blocks` から `type === 'tool_use'` エントリを `ToolCallData` に紐付けて再構築。
- `tool_use_start` → `tool_input_delta` → `tool_result` の順で状態が遷移する。
- ストリーミング完了後は `streamBlocks` がリセット (`setStreamBlocks([])`) され、履歴は `metadata.content_blocks` として `messages` に保持される。
