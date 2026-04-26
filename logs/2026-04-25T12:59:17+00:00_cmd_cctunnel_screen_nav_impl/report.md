# cmd_cctunnel_screen_nav_impl 変更ログ

## 概要

docs/screen-navigation.md の設計を実装。frontend に5画面構成 + ルートガード + モック認証、
cc-tunnel にモック認証 API。

## 実装フェーズ

| Phase | 担当 | 内容 |
|-------|------|------|
| 000 | 足軽3号 | openapi.yaml /app-auth/* 追加 + gen.go/schema.d.ts 再生成 |
| 1a | 足軽1号 | cc-tunnel AppAuth API実装（43テスト） |
| 1b | 足軽2号 | frontend AppAuthContext+AppAuthGuard（68テスト） |
| 2 | 足軽2号 | ルーティング再編 + AppAuthProvider + ルートガード（73テスト） |
| 3 | 足軽1号 | HomePage + UserMenu + LoginPage（86テスト） |
| 4 | 足軽2号 | AgentSelector + ChatPage統合（94テスト） |
| 5 | 足軽1号 | AccountSettingsPage + AgentSettingsPage（111テスト） |
| 6 | 足軽3号 | docs更新 + 変更ログ |

## 主要成果物（backend）

- apps/cc-tunnel/internal/api/handler.go: AppAuth 4エンドポイント + in-memory session
- apps/cc-tunnel/internal/api/app_auth_test.go: TDDテスト8ケース
- apps/openapi/openapi.yaml: /app-auth/* スキーマ追加

## 主要成果物（frontend）

- src/contexts/AppAuthContext.tsx: AppAuth状態管理（sessionStorage）
- src/components/AppAuthGuard.tsx: ルートガード（未認証→/loginリダイレクト）
- src/components/UserMenu.tsx: 右上ユーザーアイコン+ドロップダウン
- src/components/AgentSelector.tsx: Agent選択UI（Claude Code有効、他グレーアウト）
- src/pages/: HomePage / LoginPage / ChatPage / AccountSettingsPage / AgentSettingsPage
- src/App.tsx: ルーティング再編（BrowserRouter + AppAuthProvider）

## テスト結果

- frontend: 111テスト全PASS（SKIP=0, FAIL=0）
- backend: 43テスト全PASS（SKIP=0, FAIL=0）

## ルーティング構造

| ルート | 画面 | 認証 |
|--------|------|------|
| / | ホーム | 不要 |
| /login | ログイン | 不要 |
| /chat | チャット | AppAuth必須 → Agent認証必須 |
| /chat/:id | チャット(会話選択) | AppAuth必須 → Agent認証必須 |
| /settings/account | アカウント設定 | AppAuth必須 |
| /settings/agents | Agentログイン設定 | AppAuth必須 |
| /conversation/:id | (リダイレクト) | /chat/:id へ転送 |

## ドキュメント更新（Phase 6）

- docs/frontend.md: ルーティング構造（5画面）・AppAuthContext/AppAuthGuard/UserMenu/AgentSelector・ページ一覧を追加
- docs/api.md: /app-auth/* エンドポイント（POST login, GET me, POST logout, PATCH me）を追加
- docs/screen-navigation.md: 実装状態を「実装済み」に更新、将来拡張対応表を現状反映に修正
