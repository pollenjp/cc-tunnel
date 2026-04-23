# apps/frontend

cc-tunnel の Web フロントエンド。Vite + React + TypeScript で構成。

## 前提条件

- Node.js 24+

## セットアップ

```bash
npm install
```

## 開発サーバー

```bash
npm run dev
```

http://localhost:5173 で起動する。`/api` プレフィックスのリクエストは `localhost:8080` (Server B) にプロキシされる (`/api` プレフィックスは除去される)。

Server A・Server B を先に起動しておくこと:

```bash
# Server A
cd ../cc-remote-agent && PORT=9090 go run ./cmd/cc-remote-agent/

# Server B
cd ../cc-tunnel && go run ./cmd/cc-tunnel/ -addr :8080 -runner-url http://localhost:9090
```

Docker Compose では frontend コンテナが `/api` を `cc-tunnel` にリバースプロキシするため、ホストへ公開するのは frontend のみ。

## ビルド

```bash
npm run build
```

`dist/` に静的ファイルが出力される。

## 機能

- セッションの作成・一覧・削除 (サイドバー)
- セッションタイプの選択 (`claude_code` / `multi_agent_shogun`)
- multi_agent_shogun の 3x3 グリッドビュー + 個別ペインタブ切り替え
- マルチライン入力 (textarea、Ctrl+Enter で送信)
- Shift+Enter で Enter キー送信、Send / Enter ボタンの分離
- スクロール・リサイズ可能なペイン表示
- Auto-refresh のオン/オフ切り替え
- セッション作成時のローディングインジケーター

## ディレクトリ構成

```
apps/frontend/
├── src/
│   ├── api/
│   │   ├── client.ts       # API クライアント (openapi-fetch)
│   │   ├── client.test.ts  # API クライアントテスト
│   │   └── schema.d.ts     # 生成コード: OpenAPI 型定義 (DO NOT EDIT)
│   ├── App.tsx             # メインコンポーネント
│   ├── App.css             # アプリケーションスタイル
│   ├── env.d.ts            # 環境変数の型定義
│   ├── index.css           # グローバルスタイル
│   └── main.tsx            # エントリーポイント
├── vite.config.ts          # Vite 設定 (プロキシ含む)
├── package.json
└── tsconfig.json
```
