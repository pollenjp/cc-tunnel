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

http://localhost:5173 で起動する。`/sessions` へのリクエストは `localhost:8080` (Server B) にプロキシされる。

Server A・Server B を先に起動しておくこと:

```bash
# Server A
cd ../cc-tmux-tunnel && go run ./cmd/cc-tmux-tunnel/ -addr :9090

# Server B
cd ../cc-tunnel && go run ./cmd/cc-tunnel/ -addr :8080 -runner-url http://localhost:9090
```

## ビルド

```bash
npm run build
```

`dist/` に静的ファイルが出力される。

## 機能

- セッションの作成・一覧・削除 (サイドバー)
- セッションタイプの選択 (`claude_code` / `multi_agent_shogun`)
- tmux ペインの出力表示 (ターミナル風ビュー、2 秒ポーリング)
- multi_agent_shogun の 3x3 グリッドビュー + 個別ペインタブ切り替え
- テキスト入力の送信 (Enter キーまたは Send ボタン)
- Auto-refresh のオン/オフ切り替え
- 未管理 tmux セッションの検出とアタッチ (Discover)
- セッション作成時のローディングインジケーター

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
