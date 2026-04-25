# 画面遷移・認証フロー設計

## 1. 概要

cc-tunnel フロントエンドは「アプリ認証」と「Agent 認証」の 2 つの独立した認証概念を持つ。

| 認証種別 | 説明 | 現在の実装 | 将来の実装 |
|----------|------|------------|------------|
| **アプリ認証** | cc-tunnel アプリへのログイン | モック認証（ユーザー名: `test user`） | Google IAP 等 |
| **Agent 認証** | Claude Code 等の外部 Agent への認証 | cc-remote-agent-auth 経由（PTY フロー） | 他 Agent 対応拡張 |

> **現在の実装との差異**:
> 現在の実装（2026-04-25 時点）はアプリ認証を持たず、Agent 認証（Claude CLI への認証）のみが
> `AuthGuard` により全画面をガードしている。画面ルートも `/` と `/conversation/:id` の 2 つのみ。
> 本設計書は今後追加予定の画面構成を記述する。

---

## 2. 画面一覧

### 2.1 ホーム画面 (`/`)

**アクセス**: 未ログイン・ログイン済みどちらでも可

**UI 要素**:
- 右上: ユーザーアイコン
  - 未ログイン時: デフォルトアイコン（匿名）
  - ログイン済み: ユーザー名 + ユーザーアイコン
- 右上クリック → ドロップダウンメニュー:
  - 「アカウント設定」→ `/settings/account`（ログイン済みのみ表示）
  - 「Agentログイン設定」→ `/settings/agents`（ログイン済みのみ表示）
  - 「ログイン」→ `/login`（未ログイン時のみ表示）
- チャット開始ボタン（例: 「チャットを始める」）

**遷移**:
- チャットボタン押下 → ログイン済みか判定
  - ログイン済み → `/chat`
  - 未ログイン → `/login`

---

### 2.2 アプリ認証画面 (`/login`)

**アクセス**: 未ログイン時のみ。ログイン済みなら `/chat` にリダイレクト。

**UI 要素**:
- ログインフォーム（現在はモック: ユーザー名入力だけで認証成功）
- 将来は Google IAP ログインボタン等に置き換え

**遷移**:
- 認証成功 → `/chat` にリダイレクト

**現在の実装**: モック認証。ユーザー名 `"test user"` で常に成功。

**将来の拡張**: Google IAP 等の外部 IdP 連携。

---

### 2.3 チャット画面 (`/chat`)

**アクセス**: アプリ認証必須。未ログインなら `/login` にリダイレクト。

**UI 要素**:
- 左サイドパネル: 会話セッション一覧
  - 「+ 新しい会話」ボタン
  - 各会話の選択・削除
- 右メイン: 選択中の会話内容（メッセージ + ツールコール表示）
- 会話開始時: Agent 選択 UI
  - **Claude Code** — 対応済み（cc-remote-agent-auth 経由の PTY フロー）
  - **GitHub Copilot** — 将来対応（グレーアウト表示）
  - **Cursor CLI** — 将来対応（グレーアウト表示）

> **現在の実装**: 現在の `/` ルートがこの画面に相当する。
> App 認証なしで `AuthGuard`（Agent 認証ガード）のみが存在する。

**遷移**:
- 会話選択 → `/chat/conversation/:id`（または URL パラメーターで管理）
- 右上アイコン → ドロップダウン（ホーム画面と同様）

---

### 2.4 アカウント設定画面 (`/settings/account`)

**アクセス**: アプリ認証必須。未ログインなら `/login` にリダイレクト。

**UI 要素**:
- モックユーザーのニックネーム設定・変更
- 将来拡張: プロフィール画像、メールアドレス設定、パスワード変更等

---

### 2.5 Agentログイン設定画面 (`/settings/agents`)

**アクセス**: アプリ認証必須。未ログインなら `/login` にリダイレクト。

**UI 要素**:
- Agent 一覧カード:
  | Agent | 状態 | ボタン押下時の遷移 |
  |-------|------|-------------------|
  | Claude Code | 対応済み | 現在の認証フロー（cc-remote-agent-auth 経由） |
  | GitHub Copilot | 将来対応 | 「未対応」表示（非活性） |
  | Cursor CLI | 将来対応 | 「未対応」表示（非活性） |

**Claude Code 認証フロー（詳細は `auth.md` 参照）**:
1. `POST /auth/login` でフロー開始
2. `AuthTerminal`（xterm.js TUI）で PTY 出力を表示
3. ユーザーが claude.ai OAuth またはキー入力で認証
4. 認証成功後 → Agent ログイン設定画面に戻る

---

## 3. 画面遷移フロー

```
/ (ホーム)
├── [チャットボタン]
│   ├── ログイン済み → /chat
│   └── 未ログイン  → /login → 認証成功 → /chat
│
├── [右上アイコン クリック]
│   └── ドロップダウン
│       ├── 未ログイン時
│       │   └── 「ログイン」     → /login
│       └── ログイン済み時
│           ├── 「アカウント設定」    → /settings/account
│           └── 「Agentログイン設定」 → /settings/agents
│               ├── Claude Code ボタン → Agent 認証フロー
│               ├── GitHub Copilot     → 「未対応」（非活性）
│               └── Cursor CLI         → 「未対応」（非活性）
│
└── /chat
    └── 会話選択 → /chat/conversation/:id
```

---

## 4. ルートガード（認証保護）

| ルート | 認証要否 | 未ログイン時の挙動 |
|--------|----------|------------------|
| / | 不要 | そのままアクセス可 |
| /login | 不要 | そのままアクセス可 |
| /chat | 必須 | /login へリダイレクト（redirect クエリパラメーター付き） |
| /settings/account | 必須 | /login へリダイレクト |
| /settings/agents | 必須 | /login へリダイレクト |

**ガード実装方針**:
- 保護ルート（`/chat`, `/settings/*`）にアクセスした際、アプリ未認証なら `/login?redirect=<original>` にリダイレクト
- ログイン成功後、`redirect` パラメーターの URL に戻る

---

## 5. アプリ認証フロー

アプリ認証は cc-tunnel アプリへのログイン（≠ Agent 認証）。

### 現在: モック認証

```
ユーザー → /login 画面
         → ユーザー名入力（"test user" 固定で成功）
         → セッション Cookie 発行（or JWT）
         → /chat にリダイレクト
```

### 将来: Google IAP

```
ユーザー → /login 画面
         → 「Google でログイン」ボタン
         → Google IAP 認証ページ
         → 認証成功 → cc-tunnel にリダイレクト
         → /chat
```

**ガード実装方針**:
- `/chat`, `/settings/*` にアクセスした際、アプリ未認証なら `/login?redirect=<original>` にリダイレクト
- ログイン成功後、`redirect` パラメーターの URL に戻る

---

## 6. Agent 認証フロー

Agent 認証は外部 AI Agent（Claude Code 等）への認証。`/settings/agents` から行う。

### Claude Code（cc-remote-agent-auth 経由）— 対応済み

詳細は [`auth.md`](./auth.md) を参照。概要:

```
/settings/agents
→ 「Claude Code」ボタン押下
→ POST /auth/login
→ cc-remote-agent-auth で claude /auth を PTY 起動
→ AuthTerminal（xterm.js）に PTY 出力をポーリング表示
→ ユーザーが認証操作（OAuth URL を開く or Enter）
→ 認証成功 → loginPending=false
→ /settings/agents に戻る
```

**認証状態 UI**:
| 状態 | UI |
|------|----|
| loading | スピナー |
| authenticated | チャット使用可能 |
| login_pending | AuthTerminal（xterm.js TUI）表示 |
| unauthenticated | 「Claude でログイン」ボタン |

### GitHub Copilot / Cursor CLI — 将来対応

これらの Agent 認証は未実装。設定画面で「未対応」として表示。

---

## 7. 現在の実装と将来拡張の対応表

| 項目 | 現在の実装 | 将来の拡張 |
|------|------------|------------|
| アプリ認証 | なし（モック準備中） | Google IAP 等の外部 IdP |
| ホーム画面 | `/` = チャット画面兼用 | `/` と `/chat` を分離 |
| 認証ガード | Agent 認証のみ（`AuthGuard`） | アプリ認証ガード追加（`/chat`, `/settings/*`） |
| Agent | Claude Code のみ | GitHub Copilot, Cursor CLI を追加 |
| ルーティング | `/`, `/conversation/:id` のみ | `/login`, `/chat`, `/settings/account`, `/settings/agents` を追加 |
| ユーザーアイコン | なし | 右上に常時表示 |
