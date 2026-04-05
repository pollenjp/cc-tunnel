# apps/frontend

cc-tunnel の Web フロントエンド。Vite + React + TypeScript で構成。

## 前提条件

- Node.js 18+

## セットアップ

```bash
npm install
```

## 開発サーバー

```bash
npm run dev
```

http://localhost:5173 で起動する。`/sessions` へのリクエストは `localhost:8080` (バックエンド) にプロキシされる。

バックエンドを先に起動しておくこと:

```bash
cd ../cc-tunnel
go run ./cmd/cc-tunnel --addr :8080
```

## ビルド

```bash
npm run build
```

`dist/` に静的ファイルが出力される。

## 機能

- セッションの作成・一覧・削除 (サイドバー)
- tmux ペインの出力表示 (ターミナル風ビュー、2秒ポーリング)
- テキスト入力の送信 (Enter キーまたは Send ボタン)
- Auto-refresh のオン/オフ切り替え

## ディレクトリ構成

```
apps/frontend/
├── src/
│   ├── api.ts          # API クライアント (OpenAPI 定義に対応)
│   ├── App.tsx         # メインコンポーネント
│   ├── App.css         # アプリケーションスタイル
│   ├── index.css       # グローバルスタイル
│   └── main.tsx        # エントリーポイント
├── vite.config.ts      # Vite 設定 (プロキシ含む)
├── package.json
└── tsconfig.json
```
