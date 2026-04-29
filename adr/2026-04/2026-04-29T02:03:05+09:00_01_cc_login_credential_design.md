# ADR: cc-login credential 管理システム設計と実装

## 背景・経緯

従来の認証フローでは、`claude` CLI が出力する `~/.claude/.credentials.json` を `claude-sessions` Docker volume に永続化し、全ユーザーが同一 volume を共有していた。これによりマルチテナンシーが成立せず、セキュリティ上のリスクがあった。

また、認証 PTY の管理は `cc-remote-agent-auth` が担っていたが、責務が混在していた。

Phase 1 の目的:
1. 各ユーザーの credentials を AES-256-GCM で暗号化して PostgreSQL に保存する
2. `cc-login` という独立 HTTP サービスでログインフロー（PTY 管理）を担う
3. セッション開始時に cc-tunnel が DB から credentials を復号し、コンテナ内 tmpfs に書き込む

## 設計判断

### 暗号化方式（AES-256-GCM + AAD=username + 環境変数鍵）

| 項目 | 選定 | 理由 |
|------|------|------|
| 対称暗号 | AES-256-GCM | 認証付き暗号、Go 標準ライブラリで実装可能、性能十分 |
| Nonce 長 | 12 byte | GCM 標準。`crypto/rand` で毎回生成し nonce 再利用を防止 |
| AAD | `username` | nonce 衝突時のクロスユーザー攻撃を阻止 |
| 鍵供給 | 環境変数 `CC_LOGIN_ENCRYPTION_KEY` (base64 32 byte) | Phase 1 は環境変数、Phase 2 で GCP Secret Manager へ移行予定 |

`Encryptor` は cc-login と cc-tunnel の両方に実装されている（Phase 2 で共通パッケージに統合予定）。

### cc-login アーキテクチャ（独立 HTTP サービス: port 9092）

独立コンテナとして実装する方針を採用した。

| 候補 | 採否 | 理由 |
|------|------|------|
| HTTP service（独立コンテナ） | ✅ 採用 | cc-remote-agent と同様のパターン、PTY 管理ノウハウを流用、テストしやすい |
| CLI ツール | ✗ | フロントエンドからの操作が困難 |
| cc-tunnel に同居 | ✗ | 責務分離が壊れる、PTY ライフサイクルが会話実行と干渉する |

エンドポイント: `POST /credentials/login`, `POST /credentials/input`, `GET /credentials/output`, `POST /credentials/finalize`, `GET /credentials/status`, `DELETE /credentials`

### DB スキーマ（007_create_credentials.sql: username UNIQUE, encrypted_data BYTEA, nonce BYTEA）

```sql
CREATE TABLE credentials (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    username        TEXT        NOT NULL UNIQUE,
    encrypted_data  BYTEA       NOT NULL,
    nonce           BYTEA       NOT NULL,
    key_version     INTEGER     NOT NULL DEFAULT 1,
    is_valid        BOOLEAN     NOT NULL DEFAULT TRUE,
    last_validated  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

Phase 1 では `username` を主キー的な識別子として使用。`users` テーブルへの外部キー設定は Phase 2 で行う（app-auth 全体の DB-backed 化と同時に対応）。

### tmpfs マウント採用（セキュリティ: credentials のディスク残存を防ぐ）

セッションコンテナに `/home/user/.claude` を tmpfs でマウントする。

- `claude-sessions` 共有 volume は廃止
- コンテナ破棄時に tmpfs も消滅 → ディスク上に credentials が残らない
- マルチテナンシー実現（ユーザー間の credentials 混在が不可能になる）

コスト: 各セッション開始時に DB 復号 + tmpfs 書き込み（数 ms）。

### /init エンドポイント（セッションコンテナへの credentials 配置）

`cc-remote-agent` に `POST /init` エンドポイントを追加し、`cc-tunnel` の SessionManager がコンテナ起動後に credentials JSON を送信する。コンテナ側は受信した JSON を `/home/user/.claude/.credentials.json` に書き込む。

```
SessionManager.GetOrCreate
  → ContainerCreate（tmpfs マウント付き）
  → ContainerStart
  → waitForReady（ヘルスチェック）
  → client.InitCredentials（POST /init）  ← credentials 注入
```

### Phase1 で許容した技術的負債

- `encryptor.go` の cc-login/cc-tunnel 二重実装（Phase 2 で共通パッケージに統合）
- PTY manager 単体テスト未存在（Phase 2 で補完）
- `username` 識別子（Phase 2 で `users` テーブル FK 移行）
- cc-remote-agent-auth は残置（Phase 2 で廃止または統合）

## Phase2 バックログ

- `users` テーブル新設・credentials FK 移行
- cc-remote-agent-auth 廃止（cc-login に統合）
- `encryptor.go` 共通パッケージ化
- PTY manager 単体テスト充実
- GCP Secret Manager / KMS Envelope Encryption 連携
- 定期 credentials 有効性チェック goroutine（1時間ごと `claude /auth/status`）
- 鍵ローテーション運用手順
- Audit log（誰がどの credential を読み出したか）

## 実装のポイント

TDD アプローチを採用。各コンポーネントについてテストを先に書いてから実装した。

E2E テスト（`TestCredentialFlow_SaveAndFetch`）は testcontainers-go で PostgreSQL コンテナを起動し、cc-login 側の暗号化・DB 保存から cc-tunnel 側の `FetchAndDecrypt` までの全フローを一気通貫で検証する。これにより、暗号化実装が cc-login/cc-tunnel 間で互換性を持つことを保証している。

## 今後の課題・注意点

- `CC_LOGIN_ENCRYPTION_KEY` のローテーション手順の整備（Phase 2 で KMS 移行前に必要）
- マルチテナンシーの完全な検証（username 識別子に依存しているため、app-auth の DB-backed 化と一体で対応）
- コンテナ起動時の `/init` 失敗ハンドリング（現在はコンテナを停止・削除して呼び出し元にエラーを返す）
- cc-remote-agent-auth との二重 PTY 管理の整合性維持（Phase 1 中は両者が並存）
