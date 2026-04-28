# subtask_docs_update_004 完了報告

## 更新ファイル一覧

| ファイル | 主な変更内容 |
|---------|------------|
| `docs/frontend.md` | SSE受信フロー削除、content_blocks保存SSEセクション削除、SSE切断耐性設計削除、ツール使用可視化SSE参照削除、フックイベントパネルSSE参照削除、楽観的status更新(App.tsx handleSend)削除、TypingIndicator説明をシマーアニメーションに更新、useConversationPoller intervalMs 2000ms→1000ms更新、OpenAPI生成型からSSEイベント型参照削除、dbMsgToAPI→newMessage()参照更新 |
| `docs/architecture.md` | データフロー図をSSE→202+DBポーリングに全面更新、SSEイベント型一覧削除、SSE切断後復帰セクションをDBポーリング方式に改名・更新、バッチ保存5秒→2秒更新、コンポーネント図のSSE表記削除 |
| `docs/api.md` | メッセージ送信セクションをSSEストリーミング→非同期処理(202 Accepted)に全面書き換え、SSEイベント型記述を全削除 |
| `docs/directory-structure.md` | docs/plantuml/追加、docs/api.md・docs/database.md追加、mapping.go追加、migrations 003/004追加、useSSEAbort.ts削除、useConversationPoller.ts・useConversationListPoller.ts・TypingIndicator.tsx追加、App.tsx/client.tsコメント更新 |
| `docs/database.md` | バッチ保存5秒→2秒更新 |

## 変更概要

- SSEに関する記述を全て削除（SSE受信コード、ReadableStream、EventSource等）
- POST /messages が 202 Accepted を返す非同期処理フローに更新
- ChatView自己完結化（conversationId のみ受け取り、内部でポーリング・isRunning管理）
- React Router v7 URLルーティング（docs/frontend.mdに既存）
- useConversationPoller 1秒間隔更新
- TypingIndicator シマーアニメーション更新
- コンストラクタ関数パターン（newConversation/newMessage/newConversationDetail）反映
- ディレクトリ構造の変化（plantuml/追加、useSSEAbort.ts削除、mapping.go追加等）反映
