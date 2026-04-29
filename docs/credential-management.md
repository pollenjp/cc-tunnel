# Credential Management — cc-login + cc-tunnel 改修設計

> Phase 1 設計ドキュメント（subtask_cclogin_001a 軍師起草）
> 関連 cmd: `cmd_cctunnel_cc_login_design_impl`

## 1. 背景と目的

### 現在の認証フロー

- ユーザーは cc-tunnel フロントエンドから `/auth/login` を叩き、cc-remote-agent-auth が PTY で `claude /auth` を起動する
- ユーザーが TUI を操作して認証を完了すると `claude` CLI が `/home/user/.claude/.credentials.json` に書き込む
- credentials は `claude-sessions` Docker volume に永続化されており、コンテナ再起動を跨ぐ
- 単一の volume を全ユーザーが共有するため、**マルチテナンシーが成立していない**

### 目的（新要件）

1. 各ユーザー（app-auth user）ごとに credentials を分離して管理する
2. credentials は AES-256-GCM で暗号化して PostgreSQL に保存する
3. セッション開始時に cc-tunnel が DB から credentials を取り出し、対象コンテナへ配置する
4. cc-login という新規アプリでログインフローを担当する（cc-remote-agent-auth から責務を分離）

## 2. 全体アーキテクチャ

```
┌───────────────────────────────────────────────────────────────────────┐
│                              User Browser                              │
└───────────────┬───────────────────────────────────────────────────────┘
                │ HTTP
                ▼
        ┌───────────────┐                       ┌───────────────────┐
        │   Frontend    │ ─── /api/* ───▶       │     cc-tunnel     │
        │ (React+nginx) │ ─── /login/* ──▶      │  (会話/メッセージ)  │
        └───────┬───────┘                       └─────────┬─────────┘
                │                                          │
                │ /credentials/*                           │ Decrypt &
                │ (auth login flow)                        │ inject creds
                ▼                                          ▼
        ┌───────────────┐    pgx/v5      ┌──────────────────────────┐
        │   cc-login    │ ─────────────▶ │ PostgreSQL                │
        │ (PTY manager) │  encrypt/save  │  - users (Phase2予定)      │
        └───────┬───────┘                │  - credentials (Phase1)    │
                │ exec                   │  - conversations …         │
                ▼                        └──────────────────────────┘
        ┌──────────────────┐                       ▲
        │ claude CLI (PTY) │                       │
        └──────────────────┘                       │
                                                   │
                                  cc-tunnel writes │
                                  decrypted file ─┘
                                  to /home/user/.claude/.credentials.json
                                  inside session container
```

### 主な変更点

| 領域 | 現状 | 変更後 |
|------|------|--------|
| 認証 PTY | cc-remote-agent-auth が担当 | **cc-login** が担当（責務分離） |
| credentials の保存 | Docker volume（共有） | **PostgreSQL の暗号化カラム**（ユーザー別） |
| セッション開始時 | volume の credentials を共有 | **cc-tunnel が DB から取り出し書き込み** |
| 暗号化 | なし | **AES-256-GCM**（鍵: 環境変数→将来 KMS） |

cc-remote-agent-auth は **段階的に廃止または読み取り専用化** する（Phase 2 以降の課題）。Phase 1 では cc-login を新設し、`/auth/*` は当面残置する（フロントエンドの段階的移行に必要）。

## 3. データモデル

### 3.1 credentials テーブル（migration 007）

```sql
-- 007_create_credentials.sql
CREATE TABLE credentials (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    username        TEXT        NOT NULL UNIQUE,
    encrypted_data  BYTEA       NOT NULL,        -- AES-GCM 暗号文（タグ付き）
    nonce           BYTEA       NOT NULL,        -- 12-byte GCM nonce
    key_version     INTEGER     NOT NULL DEFAULT 1,  -- 鍵ローテーション用
    is_valid        BOOLEAN     NOT NULL DEFAULT TRUE,  -- 失効フラグ
    last_validated  TIMESTAMPTZ,                 -- 最終 /auth/status 確認時刻
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_credentials_username ON credentials(username);
CREATE INDEX idx_credentials_is_valid ON credentials(is_valid) WHERE is_valid = TRUE;
```

#### 設計上の決定と注意

- **`username` を主キー的に扱う**: 現行の app-auth は user 情報を DB 永続化していない（`AppSession` 内 in-memory map）。Phase 1 では `username` 文字列を identifier として用い、外部キーは設定しない。
- **users テーブル新設は Phase 2 へ先送り**: app-auth 全体を JWT 化または DB-backed に改修する大規模変更は本任務のスコープ外。Phase 2 で `users (id UUID PK, username TEXT UNIQUE)` を作り、credentials.user_id FK へ移行する。
- **`is_valid` フラグ**: 起動時に `claude /auth/status` で死活確認した際、無効なら FALSE に更新。再ログインを促す。
- **`key_version`**: 暗号化鍵のローテーション時に古い鍵で復号して新鍵で再暗号化するため。

### 3.2 既存テーブルへの影響

なし。conversations, messages, vm_instances, session_endpoints は無変更。

## 4. 暗号化方式と鍵管理

### 4.1 暗号化アルゴリズム

| 項目 | 選定 | 理由 |
|------|------|------|
| 対称暗号 | **AES-256-GCM** | 認証付き暗号、Go 標準 (`crypto/aes` + `crypto/cipher`)、性能十分 |
| Nonce 長 | 12 byte | GCM 標準。`crypto/rand` で毎回生成 |
| AAD | `username` | nonce 衝突時のクロスユーザー攻撃を阻止 |
| 鍵長 | 32 byte (256-bit) | AES-256 |

### 4.2 鍵管理

| 環境 | 鍵供給方法 | 備考 |
|------|----------|------|
| ローカル開発 | 環境変数 `CC_LOGIN_ENCRYPTION_KEY` (base64 32 byte) | `compose.yaml` で `${CC_LOGIN_ENCRYPTION_KEY}` を注入 |
| CI | GitHub Actions Secret | テスト時はテスト用鍵を生成 |
| 本番（Cloud Run） | **GCP Secret Manager** から起動時 fetch | Workload Identity 経由 |
| 鍵ローテーション | 新鍵（key_version=2）配布 → 旧 credentials を順次再暗号化 | バッチ job または初回アクセス時遅延再暗号化 |

#### Phase 1 必須要件

- 環境変数 `CC_LOGIN_ENCRYPTION_KEY` 未設定時は cc-login と cc-tunnel **起動失敗** とする（fail-fast）
- 鍵長検証: 32 byte でなければ panic
- 起動時にテスト暗号化/復号を実行し動作確認

#### Phase 2 推奨

- GCP Secret Manager / GCP KMS Envelope Encryption への移行
- Audit log（誰がどの credential を読み出したか）

## 5. cc-login API 仕様

### 5.1 設計判断: HTTP service として実装する

| 候補 | 採否 | 理由 |
|------|------|------|
| HTTP service（独立コンテナ） | ✅ 採用 | cc-remote-agent と同じパターン、PTY 管理ノウハウを流用、テストしやすい |
| CLI ツール | ✗ | フロントエンドからの操作が困難 |
| cc-tunnel に同居 | ✗ | 責務分離が壊れる、PTY ライフサイクルが会話実行と干渉 |

### 5.2 エンドポイント

すべて Bearer token（app-auth）必須。`username` は token から導出。

| Method | Path | 用途 |
|--------|------|------|
| POST | `/credentials/login` | claude /auth PTY を起動。レスポンスは `{loginId, message, loggedIn}` |
| POST | `/credentials/input` | PTY に入力送信。`{input: string}` |
| GET | `/credentials/output?since=N` | PTY 出力読み取り。`{data, cursor, status}` |
| POST | `/credentials/cancel` | PTY をキャンセル |
| POST | `/credentials/finalize` | PTY 完了後、`~/.claude/.credentials.json` を読み暗号化して DB 保存 |
| GET | `/credentials/status` | 自分の credentials が登録済みか・有効かを返す。`{registered, isValid, lastValidatedAt}` |
| DELETE | `/credentials` | 自分の credentials を削除 |

### 5.3 OpenAPI 追記方針

`apps/openapi/openapi.yaml` に `Credentials` タグを追加。schema は既存 `AppAuthLogin` と同じ命名規則に揃える。再生成は ashigaru1 が担当。

## 6. cc-tunnel セッション開始フロー改修

### 6.1 改修前（現状）

```
POST /conversations/{id}/messages
  → handler.SendMessage
  → execProvider.Execute(ctx, req, onEvent)
     → SessionManager.GetOrCreate(conversationID)
        → DockerRunner.ContainerCreate
        → claude-sessions volume をマウント（共有 credentials）
```

### 6.2 改修後（Phase 1）

```
POST /conversations/{id}/messages
  → handler.SendMessage
  → handler は req から bearer token → username を解決
  → CredentialService.FetchAndDecrypt(ctx, username) → credentials_json
       ↓ DB 不在 → 401 {needCredentials: true, redirect: "/login/credentials"}
       ↓ DB 存在 / is_valid=false → 401 {needRelogin: true}
  → execProvider.Execute(ctx, req, onEvent, credentials_json)
     → SessionManager.GetOrCreate(conversationID, credentials_json)
        → DockerRunner.ContainerCreate（**ユーザー専用 ephemeral ボリューム**）
        → 起動前に CredentialInitializer がコンテナ内 /home/user/.claude/.credentials.json に書き込み
```

### 6.3 ユーザー別ボリュームの扱い

Phase 1 戦略: **ephemeral tmpfs** をコンテナ起動時に注入

- `claude-sessions` 共有 volume はマウントしない
- 代わりに `tmpfs:/home/user/.claude` をマウント、起動時に CredentialInitializer が `.credentials.json` を書き込む
- 会話終了でコンテナ破棄 → tmpfs も消滅 → ディスク上に credentials は残らない

これにより:
- メモリ上のみに credentials が存在 → セキュリティ向上
- volume 共有問題が消滅 → マルチテナンシー実現
- コスト: 各セッション開始時に DB から復号→書き込み（数 ms）

### 6.4 credentials 有効性確認

| タイミング | 方法 | 失敗時 |
|----------|------|-------|
| セッション開始時 | DB から取り出し復号できれば proceed（高頻度のため active 確認はしない） | 復号失敗→500、DB 不在→401 |
| 定期バックグラウンド | 1時間ごとに `claude /auth/status` を裏で叩く | 401-class → `is_valid=FALSE` 更新 |
| メッセージ送信時に Anthropic API が 401 | execProvider が捕捉して `is_valid=FALSE` を更新 | 次回送信で 401 リダイレクト |

定期チェックは Phase 2 の goroutine（VMScaler 同様のパターン）で実装。Phase 1 は手動再ログインで運用する。

## 7. フロントエンドへの影響

### 7.1 必要な変更

1. `/login/credentials` ページ新設（`apps/frontend/src/pages/CredentialsLoginPage.tsx`）
   - cc-login PTY 出力をターミナル表示
   - ユーザー入力を `/credentials/input` へ送信
   - `loggedIn` を検出したら `/credentials/finalize` を呼ぶ
2. `AppAuthGuard` 拡張または **CredentialGuard** 新設
   - app-auth ログイン後、`/credentials/status` を確認
   - `registered=false` または `isValid=false` なら `/login/credentials?redirect=...` へ遷移
3. API クライアント `apps/frontend/src/api/credentials.ts` 新設

### 7.2 影響を受けないもの

- 既存 `/auth/*` エンドポイント呼び出しは Phase 1 では残置（旧 AuthGuard 互換）
- ChatView, Sidebar, 会話一覧周辺は無変更
- ルーティングは追加のみ

## 8. Phase 1 と Phase 2 の境界

### Phase 1（本 cmd で完了させるもの）

- ✅ migration 007_create_credentials.sql
- ✅ apps/cc-login/ 新規作成（HTTP service + PTY）
- ✅ AES-256-GCM 暗号化（環境変数鍵）
- ✅ cc-tunnel 改修（DB から復号して tmpfs へ書き込み）
- ✅ フロントエンド `/login/credentials` ページ（最小実装）
- ✅ `/auth/*` エンドポイントは残置（互換性維持）

### Phase 2（後続 cmd で対応）

- users テーブル新設・credentials FK 移行
- GCP Secret Manager / KMS 連携
- cc-remote-agent-auth コンテナ廃止
- 定期 credentials 有効性チェック goroutine
- 鍵ローテーション運用手順
- audit log
- 多要素認証（MFA）

## 9. 残課題・検討事項（家老/殿への問いかけ）

| ID | 内容 | 軍師の見解 | 要判断 |
|----|------|----------|-------|
| Q1 | username を identifier に使う Phase 1 簡略化を許容するか | 推奨：許容（早期着手のため） | 殿/家老 |
| Q2 | tmpfs 採用でディスク永続性を捨てるリスクは？ | 推奨：許容（セキュリティ優先、再ログインは年単位の頻度） | 殿/家老 |
| Q3 | `CC_LOGIN_ENCRYPTION_KEY` 不在で起動失敗にする厳格性 | 推奨：採用（fail-fast） | — |
| Q4 | cc-remote-agent-auth との二重 PTY をいつ統合するか | 推奨：Phase 2 で cc-remote-agent-auth を廃止 | 殿/家老 |
| Q5 | docker_gce プロバイダでの credential 配置経路（VM 上の cc-remote-agent はネットワーク越し） | 詳細設計は Phase 1 後半で詰める。当面は GCE 上でも同様に init endpoint で credentials を流し込む | 軍師継続検討 |

## 10. 実装着手順序（推奨）

1. **足軽1号 → cc-login 実装**（subtask_cclogin_001b）
   - migration 007 作成
   - encryption package 実装（AES-GCM）
   - HTTP server + PTY manager
   - 単体テスト（暗号化往復、PTY モック、DB CRUD）
2. **足軽2号 → cc-tunnel 改修**（subtask_cclogin_001c）
   - CredentialService 新設（DB 復号）
   - SessionManager / ExecutionProvider に credentials 引き渡し追加
   - tmpfs マウント切替
   - 既存テスト互換性確認
3. **足軽3号 → ADR 作成 + フロントエンド + 統合 check**（subtask_cclogin_001d）
   - ADR 作成
   - `/login/credentials` ページ
   - CredentialGuard
   - mise run check（SKIP=0 を環境スキップ除いて達成）

実装ガイドの詳細は `docs/_cc_login_impl_guide.md` を参照。
