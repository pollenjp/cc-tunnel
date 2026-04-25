# cmd_cctunnel_screen_nav_design 変更ログ

## 概要
cc-tunnel の画面遷移・認証フロー設計ドキュメントと PlantUML 図を新規作成。

## 作成ファイル

| ファイル | 説明 |
|---------|------|
| docs/screen-navigation.md | 画面遷移・認証フロー設計ドキュメント（5画面） |
| docs/plantuml/screen_navigation.puml | 画面遷移 state diagram |
| docs/plantuml/auth_flow.puml | 認証フロー activity diagram |
| docs/plantuml/out/screen_navigation.svg | SVG（生成済み） |
| docs/plantuml/out/screen_navigation.png | PNG（生成済み） |
| docs/plantuml/out/auth_flow.svg | SVG（生成済み） |
| docs/plantuml/out/auth_flow.png | PNG（生成済み） |

## 設計概要
5画面: ホーム(/)、アプリ認証(/login)、チャット(/chat)、アカウント設定(/settings/account)、Agentログイン設定(/settings/agents)
アプリ認証（モック認証→将来Google IAP）とAgent認証（cc-remote-agent-auth）を区別。

## 修正（subtask_screen_nav_cq001）
- auth_flow.puml Line 79: typo "lLoginPending" → "loginPending" を修正
- SVG を再生成済み
