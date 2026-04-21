# cmd_cctunnel_url_routing_001 実装レポート

## 概要

URLルーティング導入 — React Router v7 + nginx SPA フォールバック — TDD実装

## TDDサイクル

### Cycle 1: 失敗テスト先行

`src/__tests__/routing.test.tsx` を新規作成（4テスト）:

1. `/ にアクセスすると会話が選択されず ChatView が非表示`
2. `/conversation/:id アクセス時 getConversation(id) が呼ばれ ChatView が表示される`
3. `サイドバーで会話を選択すると URL が /conversation/{id} に変わる`
4. `存在しない会話IDにアクセスした場合 / へリダイレクトされ ChatView が非表示`

初回テスト実行: react-router-dom 未インストールのため import エラーで全失敗 → 期待通り

### Cycle 2: 実装

**react-router-dom インストール**
```
npm install react-router-dom  (4 packages added)
```

**main.tsx**: BrowserRouter でラップ

**App.tsx**: 構造変更
- `AppContent` (全ステート・ロジック) + `App` (Routes ラッパー) に分割
- `AppContent` に `useParams`/`useNavigate` 追加
- `handleSelectConversation`: `navigate('/conversation/${id}')` 追加
- URL init effect: 直接URLアクセス時の会話ロード + 存在しない ID → / リダイレクト
- `handleDeleteConversation`: 削除時 `navigate('/')` 追加

**App.test.tsx**: 既存テストに `<MemoryRouter>` ラップ追加

**nginx.conf.template**: 変更不要（既に SPA フォールバック設定済み）
```nginx
location / {
    try_files $uri $uri/ /index.html;
}
```

### Cycle 3: テスト通過確認

```
Test Files  9 passed (9)
Tests  41 passed (41)
SKIP=0
```

### mise run check 最終結果

```
Test Files  9 passed (9)
Tests  41 passed (41)
lint: 0 issues (frontend, cc-tunnel, cc-remote-agent)
Finished in 23.73s
```

## 変更ファイル一覧

| ファイル | 変更内容 |
| --- | --- |
| `apps/frontend/src/main.tsx` | BrowserRouter 追加 |
| `apps/frontend/src/App.tsx` | App/AppContent 分割、useParams/useNavigate、URL init effect |
| `apps/frontend/src/__tests__/App.test.tsx` | MemoryRouter ラップ追加 |
| `apps/frontend/src/__tests__/routing.test.tsx` | 新規: URLルーティングテスト (4件) |
| `apps/frontend/package.json` | react-router-dom 依存追加 |
| `docs/frontend.md` | URLルーティングセクション追加 |
