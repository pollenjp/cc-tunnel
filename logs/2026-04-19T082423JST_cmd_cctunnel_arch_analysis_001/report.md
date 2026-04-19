# cc-tunnel アーキテクチャ分析レポート

**タスクID**: gunshi_task_arch_analysis_001
**分析日**: 2026-04-19
**分析者**: 軍師 (Gunshi)

## 1. 結論（Executive Summary）

cc-tunnel の現在のアーキテクチャは **Transaction Script パターン + 二層構成（Handler-Repository）** に分類される。三層アーキテクチャやクリーンアーキテクチャには該当しない。

全体のデプロイメントトポロジーは **マイクロサービス構成**（2つの独立 Go サービス + React SPA）であるが、各サービスの内部構造は抽象化レイヤーを持たない手続き的なスタイルである。

---

## 2. ディレクトリ構造・パッケージ構成

### 2.1 cc-tunnel（Go バックエンド - API Gateway）

```
apps/cc-tunnel/
  cmd/cc-tunnel/main.go          ← Composition Root（依存組み立て + サーバー起動）
  internal/
    api/
      handler.go                  ← HTTP ハンドラー（ServerInterface 実装）
      handler_test.go             ← ユニットテスト（helper 関数のみ）
      gen.go                      ← oapi-codegen 生成（型 + ルーティング + OpenAPI spec）
    db/
      db.go                       ← DB 接続プール + goose マイグレーション実行
      repository.go               ← SQL クエリ実行（Conversation / Message CRUD）
      migrations/                 ← SQL マイグレーションファイル（001, 002）
    logging/
      handler.go                  ← slog.Handler ラッパー（エラー時スタックトレース自動付与）
    remoteclient/
      client.go                   ← cc-remote-agent への HTTP クライアント
  go.mod                          ← 独立モジュール (github.com/pollenjp/cc-tunnel/apps/cc-tunnel)
```

**Go モジュール**: 独立した `go.mod` を持ち、cc-remote-agent とはコードを一切共有しない。

**cmd/ の役割**: 典型的な Go の cmd パターン。`main.go` が依存を生成し手動でワイヤリングする Composition Root。DI フレームワークは不使用。

**internal/ の構造**: Go の `internal` パッケージで外部からのインポートを制限。4 パッケージ（api, db, logging, remoteclient）がフラットに並ぶ。

### 2.2 cc-remote-agent（Go - Claude CLI ラッパー）

```
apps/cc-remote-agent/
  cmd/cc-remote-agent/main.go    ← Composition Root
  internal/
    api/
      handler.go                  ← HTTP ハンドラー
    auth/
      manager.go                  ← PTY を用いた認証管理（状態マシン）
    claude/
      executor.go                 ← claude CLI exec + ndjson ストリーミング
    logging/
      handler.go                  ← slog.Handler ラッパー（cc-tunnel と同一コード）
  go.mod                          ← 独立モジュール (github.com/pollenjp/cc-tunnel/apps/cc-remote-agent)
```

**依存**: `go.mod` に外部依存は `creack/pty` のみ。非常に軽量。

### 2.3 frontend（React SPA）

```
apps/frontend/src/
  main.tsx                       ← ReactDOM エントリーポイント
  App.tsx                        ← ルートコンポーネント（全状態管理 + ビジネスロジック）
  api/
    client.ts                    ← API クライアント（openapi-fetch ベース + 手書き SSE）
    schema.d.ts                  ← openapi-typescript 生成型
  hooks/
    useAuth.ts                   ← 認証状態フック
  components/
    ChatView.tsx                 ← チャット画面全体
    MessageBubble.tsx            ← メッセージ描画
    MessageInput.tsx             ← テキスト入力
    Sidebar.tsx                  ← 会話リスト
    AuthGuard.tsx                ← 認証ガード
    AuthTerminal.tsx             ← PTY ターミナル表示（xterm.js）
    ToolCallCard.tsx             ← ツール呼び出し表示
```

**状態管理**: React の `useState` / `useRef` のみ。Redux, Zustand 等の外部状態管理ライブラリなし。
**ルーティング**: なし。単一画面 SPA。

---

## 3. 責務分離の状況

### 3.1 cc-tunnel の依存関係の方向

```
main.go
  ├─→ db.NewPool()          → *pgxpool.Pool
  ├─→ db.NewRepository()    → *db.Repository      ← 具象型
  ├─→ remoteclient.NewClient() → *remoteclient.Client  ← 具象型
  └─→ api.NewHandler(repo, remote) → *api.Server   ← 具象型を直接注入
```

**依存方向**: 一方向・上から下へフラット。インターフェースによる依存性逆転はない。

### 3.2 HTTP 層 / ビジネスロジック / データ層の分離

| 層 | 存在するか | 実装箇所 | コード証拠 |
|----|-----------|---------|-----------|
| **HTTP 層 (Presentation)** | ○ | `api/handler.go` + `api/gen.go` | `ServerInterface` の実装、リクエストパース、レスポンス書き出し |
| **ビジネスロジック (Service/UseCase)** | **×（独立層なし）** | `api/handler.go` 内にインライン | `SendMessage()` に会話履歴取得→ユーザーメッセージ保存→リモート実行→SSE変換→アシスタントメッセージ保存が 230 行以上にわたって手続き的に記述 |
| **データアクセス (Repository)** | ○ | `db/repository.go` | `CreateConversation`, `ListMessages`, `CreateMessage` 等の SQL ベース CRUD |

**根拠コード**:

- `handler.go:182-619` (`SendMessage`) — 1メソッド内に以下の責務が混在:
  - リクエスト JSON パース（L183-190）
  - 会話存在確認 + 履歴取得（L196-205）
  - ユーザーメッセージ DB 保存（L208-212）
  - session_id 探索ロジック（L216-224）
  - 会話履歴の remoteclient 用変換（L228-249）
  - SSE ヘッダー設定 + Flusher 取得（L251-259）
  - remoteclient.Execute コールバック内での SSE イベント変換・送信（L286-569）
  - エラーハンドリング（L571-582）
  - アシスタントメッセージの DB 保存（L585-618）

### 3.3 インターフェース（抽象）の使用状況

| インターフェース | 定義場所 | 用途 | DI/テスト用か |
|----------------|---------|------|-------------|
| `ServerInterface` | `gen.go:552` | HTTP ルーティング（oapi-codegen 生成） | **No** — フレームワーク用。`Server` 構造体がこれを満たす |
| `http.Flusher` | 標準ライブラリ | SSE 送信用 | **No** — 標準型アサーション |

**ビジネスロジックのテスト用インターフェースは定義されていない**。`handler.go` の `Server` 構造体は `*db.Repository` と `*remoteclient.Client` の具象型を直接保持する:

```go
// handler.go:17-20
type Server struct {
    repo   *db.Repository
    remote *remoteclient.Client
}
```

---

## 4. 既知パターンとの照合

### 4.1 Transaction Script ★★★★★（最も一致）

> Martin Fowler, Patterns of Enterprise Application Architecture:
> "Organizes business logic by procedures where each procedure handles a single request from the presentation."

**一致根拠**:
- 各 HTTP ハンドラーメソッド（`CreateConversation`, `SendMessage`, `GetConversation` 等）が「1リクエスト = 1手続き」として完結
- ドメインモデル層がない（`Conversation` / `Message` は DTO / Row 構造体であり、メソッドを持たない純粋なデータ構造）
- ビジネスロジックがハンドラー内の手続きコードとして直接記述される

### 4.2 三層アーキテクチャ ★★☆☆☆（部分的一致）

```
三層アーキテクチャ:
  Presentation → Business Logic → Data Access

cc-tunnel の実態:
  Presentation (handler) ──────→ Data Access (repository)
       └── Business Logic がここに混在
```

**不一致根拠**:
- Business Logic 層が独立パッケージとして存在しない
- `handler.go` が Presentation と Business Logic の両方の責務を持つ
- `internal/` 配下に `service/` や `usecase/` ディレクトリがない

### 4.3 クリーンアーキテクチャ ★☆☆☆☆（不一致）

**不一致根拠**:
- 依存性逆転原則（DIP）が適用されていない — handler が具象型に直接依存
- Entities / Use Cases / Interface Adapters / Frameworks & Drivers の4層構造ではない
- ポートとアダプターの概念がない
- Repository のインターフェース定義がない

### 4.4 ヘキサゴナルアーキテクチャ（Ports & Adapters）★☆☆☆☆（不一致）

**不一致根拠**:
- Port（インターフェース）が定義されていない
- Adapter の明示的分離がない
- ドメインコアが外部依存から保護されていない

### 4.5 MVC / MVVM ★☆☆☆☆（不一致）

- バックエンドは REST API サーバーであり、テンプレートベースの View を持たない
- フロントエンドは React コンポーネントベースであり、MVC よりもコンポーネント指向

### 4.6 Active Record ★★☆☆☆（部分的要素あり）

- `db.Conversation` / `db.Message` 構造体は DB テーブルと 1:1 マッピングだが、構造体自身に永続化メソッドがない
- 永続化は `Repository` 経由 → **Active Record ではなく Data Mapper 寄り**（ただし正式な Data Mapper パターンとも言い難い）

### 4.7 デプロイメントアーキテクチャ

| パターン | 評価 |
|---------|------|
| モノリシック | **No** — 2サービスに分離、独立 go.mod |
| マイクロサービス | **部分的 Yes** — 2サービス + SPA、HTTP 通信、独立デプロイ可能 |
| モジュラーモノリス | **No** — コード共有なし、同一プロセスでない |

→ **軽量マイクロサービス**（2サービス構成）が最も適切。

---

## 5. 現状評価

### 5.1 強み

| # | 強み | 根拠 |
|---|------|------|
| 1 | **シンプルさ・明快さ** | Go ソースファイル計 10 本（テスト含む）。各ハンドラーメソッドを読めばリクエストの全処理フローが分かる。新規参加者の学習コストが低い |
| 2 | **開発速度** | 抽象レイヤーが少ないため、機能追加時に修正箇所が少ない。handler.go に直接書けば完了 |
| 3 | **OpenAPI 駆動の型安全性** | `openapi.yaml` → Go 型生成（oapi-codegen）+ TypeScript 型生成（openapi-typescript）のパイプラインにより、フロントエンド・バックエンド間の型不整合を防止 |
| 4 | **Go internal パッケージ** | 外部からのインポートを構造的に禁止。Go の言語機能によるアクセス制御 |
| 5 | **独立デプロイ** | 各サービスが独立 go.mod + Docker で、独立してビルド・デプロイ可能 |
| 6 | **依存の軽量さ** | cc-remote-agent は外部依存 1 つ（creack/pty）のみ。サプライチェーンリスクが低い |

### 5.2 弱み・改善余地

| # | 弱み | 根拠 | 影響 |
|---|------|------|------|
| 1 | **テスタビリティの制約** | `Server` 構造体が `*db.Repository` / `*remoteclient.Client` の具象型に直接依存。インターフェースがないためモック注入が困難 | `handler_test.go` は `writeJSON` / `writeError` ヘルパーのみテスト。`SendMessage` 等のビジネスロジックの自動テストが書きにくい |
| 2 | **Handler の肥大化** | `SendMessage()` が 230 行超。リクエストパース→DB操作→リモート呼び出し→SSE変換→DB保存が 1 メソッドに集約 | 責務の混在により、部分的な変更（例: SSE フォーマット変更）が他の責務に影響するリスク |
| 3 | **ビジネスロジックの再利用不可** | ロジックが handler 内にインラインのため、CLI やバッチ処理など別インターフェースから同じロジックを呼べない | 現状は HTTP API のみなので問題なし。将来的な WebSocket 対応や CLI 対応時に障害 |
| 4 | **コード重複** | `logging/handler.go` が cc-tunnel と cc-remote-agent で完全に同一内容 | メンテナンスコスト。一方だけ修正して他方を忘れるリスク |
| 5 | **フロントエンド状態管理の集中** | `App.tsx` に全グローバル状態（conversations, messages, streamBlocks, hookEvents, streamMeta 等 10+ の useState）が集約 | 機能追加に伴い App.tsx が肥大化する。コンポーネント間の状態共有が props drilling に依存 |

---

## 6. 分析まとめ

### 一言で表すと

> **「OpenAPI 駆動の軽量マイクロサービスで、各サービス内部は Transaction Script + 二層（Handler-Repository）で手続き的に実装されたアーキテクチャ」**

### 分類表

| 観点 | パターン名 | 確信度 |
|------|-----------|-------|
| ビジネスロジック構成 | **Transaction Script** | ★★★★★ |
| サービス内部構造 | **二層（Handler + Repository）** | ★★★★☆ |
| デプロイメント | **軽量マイクロサービス（2サービス + SPA）** | ★★★★☆ |
| データアクセス | **Repository / Table Data Gateway** | ★★★★☆ |
| フロントエンド | **コンポーネントベース SPA** | ★★★★★ |
| API 設計 | **OpenAPI-First（Contract-First）** | ★★★★★ |

### 該当しないパターン

- クリーンアーキテクチャ（依存性逆転なし）
- ヘキサゴナルアーキテクチャ（ポート定義なし）
- 三層アーキテクチャ（ビジネスロジック層が独立していない）
- DDD（ドメインモデル・集約・値オブジェクトなし）
- CQRS / Event Sourcing（読み書き分離なし）
- MVC / MVVM（REST API + SPA のためカテゴリ不一致）
