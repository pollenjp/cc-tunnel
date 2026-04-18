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
   │           ├─ MessageBubble (×N)
   │           │   └─ ToolCallCard (×N, streaming 中)
   │           └─ MessageInput
   └─ [ログイン待ち] 認証画面
       └─ AuthTerminal
```

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

選択中会話のメッセージ一覧と入力欄を表示する。メッセージ追加時に最下部へ自動スクロールする (`messagesEndRef`)。

**Props**

| 名前            | 型                    | 用途                                         |
| --------------- | --------------------- | -------------------------------------------- |
| `messages`      | `Message[]`           | 表示するメッセージ一覧                       |
| `onSend`        | `(content) => void`   | 送信ハンドラ                                 |
| `isStreaming`   | `boolean`             | SSE 受信中フラグ                             |
| `streamMeta`    | `StreamMeta \| null`  | モデル・コスト・所要時間などのメタ情報       |
| `hookEvents`    | `SSEHookEvent[]`      | フックイベント一覧（フックイベントパネル用） |
| `toolCalls`     | `ToolCall[]`          | ツール呼び出し一覧（ToolCallCard 表示用）    |
| `input`         | `string`              | テキストエリアの入力値                       |
| `onInputChange` | `(value) => void`     | 入力値変更ハンドラ                           |
| `onHamburger`   | `() => void`          | モバイルでサイドバーを開くハンドラ           |

### `MessageBubble`

1件のメッセージを描画するコンポーネント。ユーザー/アシスタントでスタイルを切り替える。アシスタントメッセージは ReactMarkdown でレンダリングし、コードブロックは react-syntax-highlighter でハイライト。思考ブロック (`thinking`) は折りたたみ UI で表示する。

### `ToolCallCard`

ツール呼び出し 1 件を表示するカードコンポーネント。ツール名に対応したアイコンとツール名を表示し、クリックで引数・結果を折りたたみ表示する。

- `open` (`useState<boolean>`) で展開/折りたたみ状態を管理する。
- `toolCall.isRunning` が `true` のとき: 黄色の pulse インジケータを表示。
- `toolCall.isRunning` が `false` のとき: 緑の ✓ マークを表示。
- 展開時: 引数 (`inputJson`) を `pre` タグで、結果 (`result`) を `pre` タグで表示（最大高さあり、スクロール可）。
- アイコンは `TOOL_ICONS` テーブルで管理し、未定義ツールは `🔧` にフォールバック。

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
| `toolCalls`        | `useState<ToolCall[]>`      | 受信したツール呼び出し一覧                               |
| `streamContentRef` | `useRef<string>`            | ストリーミング中のテキスト累積 (RAF バッチ更新用)        |
| `streamThinkingRef`| `useRef<string>`            | ストリーミング中の思考ブロック累積テキスト (RAF バッチ更新用) |
| `rafIdRef`         | `useRef<number>`            | `requestAnimationFrame` ID (重複スケジュール防止)        |
| `streamMetaRef`    | `useRef<StreamMeta>`        | メタ情報の最新値を保持するバッファ (RAF 間の参照用)      |

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
   - `type: "text"` → `messages` の最後のアシスタントメッセージに `content` を追記する。
   - `type: "thinking"` → `streamThinkingRef` に追記し、`messages` の `metadata.thinking` を更新する。
   - `type: "text_delta"` → `streamContentRef` に追記し、`requestAnimationFrame` でバッチ更新する。
   - `type: "thinking_delta"` → `streamThinkingRef` に追記し、`requestAnimationFrame` でバッチ更新する。
   - `type: "init"` → `streamMetaRef` にモデル名・セッション ID を設定し `streamMeta` を更新する。
   - `type: "rate_limit"` → `streamMetaRef` にレートリミット状態を設定し `streamMeta` を更新する。
   - `type: "cost"` → `streamMetaRef` にコスト・所要時間を設定し `streamMeta` を更新する。
   - `type: "hook_event"` → `hookEvents` に追加する。
   - `type: "tool_use_start"` → `toolCalls` に新しい `ToolCall`（`isRunning: true`）を追加する。
   - `type: "tool_input_delta"` → `toolCalls` の該当インデックスの `inputJson` に追記する。
   - `type: "tool_result"` → `toolCalls` の該当 `toolUseId` に `result` をセットし `isRunning: false` にする。
   - `type: "done"` / `type: "error"` → コールバックに渡されるが現状の UI では特別処理なし。
6. ストリーミング完了後、`sending` を `false` にして会話リストを更新する。

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

ストリーミング中に受信した `hook_event` を `ChatView` 下部の `<details>` 要素に折りたたみ表示する。

- `hookEvents.length > 0` のとき表示される。
- `<summary>` に件数 (`Hook Events (N)`) を表示。
- 展開すると各イベントを `[subtype] hook_name` 形式で一覧表示する（最大高さ 160px、スクロール可）。

---

## ツール使用可視化

SSE ストリーミング中にアシスタントがツールを呼び出すと、最後のアシスタントメッセージの直下に `ToolCallCard` のリストが表示される。

- 表示条件: `isStreaming && isLast && msg.role === 'assistant' && toolCalls.length > 0`
- 各カードはツール名・実行状態（実行中/完了）・引数・結果を表示する。
- `tool_use_start` → `tool_input_delta` → `tool_result` の順で状態が遷移する。
- ストリーミング完了後は `toolCalls` がリセット (`setToolCalls([])`) され、カードは非表示になる。

---

## 思考ブロック（thinkingOpen 折りたたみ UI）

アシスタントメッセージに `metadata.thinking` が存在する場合、`MessageBubble` はメッセージ本文の上部に折りたたみ可能な「思考過程」セクションを表示する。

- `thinkingOpen` (`useState<boolean>`) で開閉状態を管理する。
- 閉じた状態: `▸ 思考過程` ボタンのみ表示。
- 開いた状態: `▾ 思考過程` ボタン + 思考テキスト (最大高さ 256px、スクロール可) を表示。
- ストリーミング中は `streamThinkingRef` を通じてリアルタイムに `metadata.thinking` が更新されるため、開いていれば逐次表示される。
