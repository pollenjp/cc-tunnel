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

### `MessageBubble`

1件のメッセージを描画するコンポーネント。ユーザー/アシスタントでスタイルを切り替える。アシスタントメッセージは ReactMarkdown でレンダリングし、コードブロックは react-syntax-highlighter でハイライト。思考ブロック (`thinking`) は折りたたみ UI で表示する。

### `MessageInput`

テキストエリアと送信ボタンを含む入力欄。`Enter` で送信、`Shift+Enter` で改行。テキスト量に応じて高さが自動調整される。モバイルではハンバーガーボタンでサイドバーを開ける。

### `AuthTerminal`

Claude CLI の認証フロー (OAuth) 用のターミナルエミュレータ。`@xterm/xterm` を埋め込み、バックエンド `/auth/output` エンドポイントを 250ms ポーリングして出力をターミナルに書き込む。認証 URL が検出された場合はリンクボタンとして表示する。

---

## 主要 state 一覧

### `App.tsx`

| 名前                   | 種別                       | 用途                                                                        |
| ---------------------- | -------------------------- | --------------------------------------------------------------------------- |
| `conversations`        | `useState<Conversation[]>` | 会話リスト                                                                  |
| `selectedId`           | `useState<string \| null>` | 選択中会話 ID                                                               |
| `messages`             | `useState<Message[]>`      | 現在の会話のメッセージ一覧                                                  |
| `input`                | `useState<string>`         | テキストエリアの入力値                                                      |
| `sending`              | `useState<boolean>`        | SSE 送信中フラグ                                                            |
| `sidebarOpen`          | `useState<boolean>`        | モバイルでのサイドバー開閉状態                                              |
| `streamingThinkingRef` | `useRef<string>`           | ストリーミング中の思考ブロック累積テキスト (再レンダリングを避けるため ref) |

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
   - `type: "thinking"` → `streamingThinkingRef` に追記し、`messages` の `metadata.thinking` を更新する。
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

## 思考ブロック（thinkingOpen 折りたたみ UI）

アシスタントメッセージに `metadata.thinking` が存在する場合、`MessageBubble` はメッセージ本文の上部に折りたたみ可能な「思考過程」セクションを表示する。

- `thinkingOpen` (`useState<boolean>`) で開閉状態を管理する。
- 閉じた状態: `▸ 思考過程` ボタンのみ表示。
- 開いた状態: `▾ 思考過程` ボタン + 思考テキスト (最大高さ 256px、スクロール可) を表示。
- ストリーミング中は `streamingThinkingRef` を通じてリアルタイムに `metadata.thinking` が更新されるため、開いていれば逐次表示される。
