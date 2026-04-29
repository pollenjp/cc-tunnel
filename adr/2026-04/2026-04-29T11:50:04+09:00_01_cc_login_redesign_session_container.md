# ADR: cc-login 廃止 — credentials をセッションコンテナへ統合

## 背景・経緯

旧設計（ADR `2026-04-29T02:03:05+09:00_01_cc_login_credential_design.md`）では、
`apps/cc-login/` を独立 HTTP サービス（port 9092）として新設し、
PTY による `claude /auth` 実行と credentials の暗号化・DB 保存を担わせる方針だった。

殿の裁定（2026-04-29）により、この方針を全面改訂した。

## Cloud Run concurrency=80 セキュリティリスク（方針変更の核心）

Cloud Run はデフォルト concurrency=80 で同一インスタンスが複数リクエストを並行処理する。
cc-login が Cloud Run 上で動作した場合：

- ユーザーAの `claude /auth` フローが書き込んだ `~/.claude/.credentials.json` が
- 同一インスタンスで処理されているユーザーBの finalize 処理から読まれる
- → **ユーザー隔離が破綻し、credentials の他者漏えいリスクが生じる**

cc-remote-agent-auth（既存の認証専用常駐コンテナ）も同質の問題を抱えている。

## 決定事項

**credentials のライフサイクルを「セッション毎の Docker コンテナ（会話 ID 単位）」内に閉じ込める。**

cc-tunnel は既に会話毎に cc-remote-agent 派生のセッションコンテナを起動する仕組みを持ち、
各コンテナは `tmpfs:/home/user/.claude` を持つ独立空間である。
このコンテナの中で「認証も会話実行も両方行う」ことで、
Docker コンテナ単位の隔離が credentials のユーザー隔離に流用できる。

主な決定:
- `apps/cc-login/` を廃止（全削除）
- `apps/compose.yaml` の cc-login サービスを削除
- `cc-remote-agent` に `POST /auth/finalize-credentials` endpoint を新設
  （コンテナ内 `~/.claude/.credentials.json` を読んで返す）
- `cc-tunnel` に再ログイン用 2 endpoint を新設
  - `POST /api/credentials/relogin/start` — 空セッションコンテナ起動
  - `POST /api/credentials/relogin/finalize` — credentials pull → 暗号化 → DB 保存
- フロントエンド: `CredentialGuard`, `CredentialsLoginPage` を cc-tunnel API 呼び出しに切替

## 影響範囲

| 変更種別 | 対象 |
|---------|------|
| 削除 | `apps/cc-login/` ディレクトリ全体 |
| 削除 | `apps/compose.yaml` の cc-login サービス定義 |
| 削除 | `docs/plantuml/credential_login_sequence.puml`（旧 cc-login PTY 図） |
| 新設 | `cc-remote-agent`: `POST /auth/finalize-credentials` |
| 新設 | `cc-tunnel`: `POST /api/credentials/relogin/start`, `/relogin/finalize` |
| 新設 | `cc-tunnel`: `RemoteClient.PullCredentialsFromSession` |
| 改修 | `apps/frontend`: `CredentialGuard`, `CredentialsLoginPage`（呼び先切替） |
| 改修 | `apps/compose.yaml`: `CC_LOGIN_ENCRYPTION_KEY` を cc-tunnel に注入 |

## 旧 ADR との関係

`2026-04-29T02:03:05+09:00_01_cc_login_credential_design.md` は旧方針の設計記録として残置する。
本 ADR が新方針（セッションコンテナ統合）の正式な決定文書である。

暗号化方式（AES-256-GCM + AAD=username + 環境変数鍵 `CC_LOGIN_ENCRYPTION_KEY`）、
DB スキーマ（`007_create_credentials.sql`）は変更なし。

## Phase 2 バックログ（変更なし）

以下は旧 ADR から引き継ぐ Phase 2 課題。本改訂により必要性が強化されたものもある。

- `users` テーブル新設・credentials FK 移行
- cc-remote-agent-auth 共有常駐コンテナ廃止（本改訂で必要性が消滅したため早期削除可）
- GCP Secret Manager / KMS Envelope Encryption 連携
- 鍵ローテーション運用手順
- PTY manager 単体テスト充実（cc-remote-agent 側）
- 定期 credentials 有効性チェック goroutine
- audit log

## 実装完了ステータス（2026-04-29）

| Phase 1 項目 | 担当 | 状態 |
|-------------|------|------|
| migration 007（既存） | — | ✅ 完了 |
| cc-tunnel encryptor / repository / service（既存） | — | ✅ 完了 |
| cc-tunnel SendMessage credentials チェック（既存） | — | ✅ 完了 |
| tmpfs マウント＋/init endpoint（既存） | — | ✅ 完了 |
| cc-remote-agent `/auth/finalize-credentials` | 足軽2号 | ✅ 完了（002c） |
| cc-tunnel `RemoteClient.PullCredentialsFromSession` | 足軽2号 | ✅ 完了（002c） |
| cc-tunnel relogin start/finalize endpoints | 足軽2号 | ✅ 完了（002c） |
| cc-login 削除 / compose 整理 | 足軽2号 | ✅ 完了（002c） |
| フロント `CredentialGuard`, `CredentialsLoginPage` | 足軽3号 | ✅ 完了（002d） |
| compose `CC_LOGIN_ENCRYPTION_KEY` 注入 | 足軽3号 | ✅ 完了（002d） |
| ADR 改訂（本文書） | 足軽3号 | ✅ 完了（002e） |
| 旧 `credential_login_sequence.puml` 削除 | 足軽3号 | ✅ 完了（002e） |
| E2E テスト（再ログインフロー）追加 | 足軽3号 | ✅ 完了（002e） |
