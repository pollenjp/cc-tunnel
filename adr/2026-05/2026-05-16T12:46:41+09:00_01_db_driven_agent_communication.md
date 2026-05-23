# ADR: cc-remote-agent の `claude -p` を廃し DB 経由の Hook 駆動通信に切り替える

**日時**: 2026-05-16T12:46:41+09:00
**ステータス**: 提案
**関連ブランチ**: `claude/db-agent-communication-Pr60j`
**関連 ADR**:
- `2026-04-29T11:50:04+09:00_01_cc_login_redesign_session_container.md` (session container 化)
- `2026-04-29T15:55:56+09:00_01_docker_gce_multi_container.md` (per-session container)
- `2026-05-06T11:00:45+09:00_01_container_manager_on_vm.md` (container-manager 導入)
**関連参照**:
- `multi-agent-shogun` リポジトリ `.claude/settings.json` および
  `scripts/stop_hook_inbox.sh` / `scripts/session_start_hook.sh`

---

## 背景

### 現状の実装

`apps/cc-remote-agent/internal/claude/executor.go` は、`POST /execute` を
受けるたびに `os/exec` で `claude -p --output-format=stream-json --verbose
--dangerously-skip-permissions [--resume <session_id>] -- <prompt>` を
起動し、stdout の ndjson を HTTP レスポンスへそのままストリームしている
(`runStream`)。

データフローは ADR `2026-04-29 docker_gce_multi_container` で確立した
通り、以下のとおり:

```
Frontend (DB polling 1s)
  ↓ POST /messages
cc-tunnel (Go) ── INSERT messages(status='streaming') → DB
  ↓ POST /execute (JSON)
cc-remote-agent ── fork/exec `claude -p ...`
  ↓ ndjson (HTTP response body)
cc-tunnel goroutine ── 2秒間隔で content_blocks を UPDATE → DB
  ↓ exit
cc-tunnel ── UPDATE messages SET status='completed'
```

### 現状の課題

| ID | 課題 | 影響 |
|----|------|------|
| P1 | 1 メッセージごとに `claude` プロセスを fork/exec する | プロセス起動・モデル初期化のコールドスタートが毎ターン発生。体感レイテンシ悪化。 |
| P2 | `claude -p` の一発実行モードは hook (SessionStart/Stop/PreToolUse 等)・MCP 永続接続・sub-agent などインタラクティブモード前提の機能と相性が悪い | サブエージェントや MCP を使うシナリオを取りこぼす。設計余地が狭い。 |
| P3 | cc-remote-agent ↔ cc-tunnel 間で HTTP ndjson を常時保持しなければならない | コネクション切断 (Cloud Run の idle timeout / VPC connector の再接続 / GCE VM 再起動) で実行中の応答が丸ごと失われる。`--resume` 経路は CLI 側のセッションファイルが container の tmpfs 上にあり、container 落ちと同時に消える。 |
| P4 | container が落ちた場合に「途中まで生成された assistant 応答」を再構築できない | 部分結果は cc-tunnel goroutine のメモリ + DB に 2 秒間隔で書き出されるが、`claude -p` 側の状態 (session_id, hook 結果) は HTTP ストリームでしか伝わらず、リトライの基準点が存在しない。 |
| P5 | `--resume` のフォールバック (prompt stuffing) が executor 内に閉じている | `claude` 起動失敗・session not found 検知ロジックが二重実装になっており、運用観測しにくい。 |

### multi-agent-shogun での先行事例

`multi-agent-shogun` は同様の問題 (= 1ターン毎の `claude` 起動コストと、
コンテキストを跨いだ persona 維持) を Claude Code 公式 hook で解いている:

- **Stop hook** (`scripts/stop_hook_inbox.sh`) が agent の停止を捕まえて
  `inotifywait` で 55 秒間 mailbox ファイルの更新を待ち、新規 instruction
  があれば `{"decision":"block","reason":"<prompt>"}` を stdout に出し
  Claude Code を「同一セッション内で再応答」させる。プロセスは終了せず、
  context を保持したまま次のターンに突入できる。
- **SessionStart hook** (`scripts/session_start_hook.sh`) が起動/`/clear`
  /compaction の全 matcher で発火し、persona と forbidden_actions を
  毎回確定的に注入する。これにより compaction 後の役職誤認を防いでいる。
- 通信は file ベース (`queue/inbox/<agent>.yaml`) で flock 排他。
  `inbox_watcher.sh` の `inotifywait` が pane に短い nudge (`inboxN`) を
  送り、agent はファイル本体を Read する。

cc-tunnel に持ち込むべき発想:

1. agent プロセスを **長寿命の対話セッション**にして、ターン間で
   再起動しない。
2. 入出力は **hook + 永続ストレージ** で授受し、HTTP の生存に依存させない。

そのまま file ベースを採用すると、cc-tunnel の per-session container は
GCE VM 上のローカル FS / tmpfs にしか書けないので、container 落ち時に
情報が欠損する (P4 と同型の問題)。よって永続化先は **PostgreSQL** とする。

---

## 決定事項

cc-remote-agent から `claude -p` 起動方式を廃止し、container 内で
**長寿命の対話モード `claude` プロセス**を 1 本だけ起動する。
agent への入出力は Claude Code の hook 機構を用い、**PostgreSQL の
agent 通信用テーブル**を介して cc-tunnel と双方向にやり取りする。

### 1. プロセス構成

per-session container (`cctunnel-session-<convID[:8]>`) の中で:

| プロセス | 役割 | ライフタイム |
|----------|------|--------------|
| `claude` (対話モード) | 長寿命の Claude Code セッション。`--output-format=stream-json` は使わず、hook で I/O する | container 寿命 (idle 検知での停止まで) |
| `cc-remote-agent` (Go) | HTTP API。`/execute` は廃止し、`/health` `/auth/*` と (新設) DB プロキシ・claude 起動制御のみ持つ | container 寿命 |
| hook scripts (bash/python) | `~/.claude/settings.json` から呼ばれる UserPromptSubmit / Stop / SessionStart / PreToolUse / PostToolUse | 各 hook 発火ごと |

`claude` プロセスは container 起動時 (entrypoint) に 1 回だけ起動する。
hook の `Stop` で `{"decision":"block","reason":<next_prompt>}` を返す
ことで、新しいユーザー入力をそのまま同一セッションへ注入し続ける。

### 2. DB スキーマ (新規 migration `010_create_agent_dispatch.sql`)

> `009_add_zero_agents_since.sql` は VM reap 用に main で既に採番済み
> (ADR `2026-05-20_vm_reap_dual_path`)。本 ADR の DDL は 010 から始める。

既存 `messages` テーブルは「ユーザーに見せる会話ログ」として責務を
維持し、agent との物理通信路は専用テーブルに分離する。

#### `agent_dispatches` (cc-tunnel → agent への命令キュー)

| カラム | 型 | 制約 | 説明 |
|--------|-----|------|------|
| `id` | UUID | PK, `gen_random_uuid()` | 命令の一意 ID |
| `conversation_id` | UUID | NOT NULL, FK→conversations(id) CASCADE | 対象会話 |
| `assistant_message_id` | UUID | NOT NULL, FK→messages(id) CASCADE | 紐づく assistant メッセージ (status='streaming') |
| `prompt` | TEXT | NOT NULL | agent に渡すユーザー prompt |
| `system_prompt` | TEXT | nullable | 上書きしたい場合のみ |
| `status` | TEXT | NOT NULL, CHECK ∈ {`pending`,`delivered`,`consumed`,`error`} | hook 側から更新 |
| `delivered_at` | TIMESTAMPTZ | nullable | Stop hook が block で投入した時刻 |
| `consumed_at` | TIMESTAMPTZ | nullable | 同 dispatch に対する応答終了時刻 |
| `created_at` | TIMESTAMPTZ | NOT NULL, `NOW()` | |
| `updated_at` | TIMESTAMPTZ | NOT NULL, `NOW()` | |

インデックス:
- `idx_agent_dispatches_conv_status ON agent_dispatches(conversation_id, status)` — 「この会話の pending 命令」高速取得
- `idx_agent_dispatches_pending ON agent_dispatches(created_at) WHERE status='pending'` — agent の poll 用

#### `agent_outputs` (agent → cc-tunnel への部分応答)

| カラム | 型 | 制約 | 説明 |
|--------|-----|------|------|
| `id` | UUID | PK | |
| `dispatch_id` | UUID | NOT NULL, FK→agent_dispatches(id) CASCADE | どの命令への応答か |
| `assistant_message_id` | UUID | NOT NULL, FK→messages(id) CASCADE | 表示先 |
| `event_seq` | BIGINT | NOT NULL | 同一 dispatch 内の hook 発火順 (PreToolUse=1, PostToolUse=2, ...) |
| `event_type` | TEXT | NOT NULL, CHECK ∈ {`session_start`,`user_prompt_submit`,`pre_tool_use`,`post_tool_use`,`stop`,`assistant_text`,`thinking`,`error`} | |
| `payload` | JSONB | NOT NULL | hook から受け取った JSON。content_blocks の追記単位 |
| `status` | TEXT | NOT NULL, CHECK ∈ {`partial`,`final`,`error`} | `final` は Stop hook での確定 |
| `created_at` | TIMESTAMPTZ | NOT NULL, `NOW()` | |

`UNIQUE (dispatch_id, event_seq)` — hook が冪等にリトライしても重複しない。

このテーブルは「生イベントの append-only ログ」として位置づける。
`messages.message_data.content_blocks` への反映は cc-tunnel 側で
agent_outputs を fold して行う (既存の 2 秒バッチ集約ロジックを流用)。

#### 既存 `messages` テーブル

スキーマ変更なし。assistant メッセージは従来通り
`status` ∈ {`streaming`,`completed`,`error`} で frontend に提示する。
`message_data.session_id` は Claude Code の `session_id` (SessionStart
hook で `payload.session_id` から取得) を入れる。

### 3. 通信フロー

```
┌──────────┐                ┌──────────┐                ┌────────────┐
│ Frontend │                │ cc-tunnel│                │ PostgreSQL │
└────┬─────┘                └────┬─────┘                └─────┬──────┘
     │ POST /messages            │                            │
     ├──────────────────────────►│                            │
     │                           │ INSERT messages            │
     │                           │   (status='streaming')     │
     │                           ├───────────────────────────►│
     │                           │ INSERT agent_dispatches    │
     │                           │   (status='pending')       │
     │                           ├───────────────────────────►│
     │  202 Accepted             │                            │
     │◄──────────────────────────┤                            │
     │                           │                            │
     │                           │              ┌─────────────┴────────────┐
     │                           │              │ cc-remote-agent container │
     │                           │              │  ┌───────────────────┐   │
     │                           │              │  │ claude (long-lived)│  │
     │                           │              │  └─────────┬─────────┘   │
     │                           │              │   Stop hook │             │
     │                           │              │   (poll DB) │             │
     │                           │              │             ▼             │
     │                           │   SELECT pending dispatch                │
     │                           │◄──────────── (psql / Go DB プロキシ) ───┤
     │                           │   UPDATE status='delivered'              │
     │                           │              │  hook returns             │
     │                           │              │  {"decision":"block",     │
     │                           │              │   "reason":<prompt>}      │
     │                           │              │             │             │
     │                           │              │   claude 次ターン応答    │
     │                           │              │             │             │
     │                           │              │  PreToolUse / PostToolUse│
     │                           │              │  hook → INSERT outputs   │
     │                           │   INSERT agent_outputs (partial)         │
     │                           │◄─────────────┤                          │
     │                           │              │   Stop hook (応答完了)   │
     │                           │   INSERT agent_outputs (final)           │
     │                           │   UPDATE dispatch status='consumed'      │
     │                           │◄─────────────┤                          │
     │ GET /conversations/{id}   │                            │             │
     ├──────────────────────────►│                            │             │
     │                           │ SELECT messages, fold      │             │
     │                           │   agent_outputs            │             │
     │                           ├───────────────────────────►│             │
     │  ConversationDetail       │                            │             │
     │◄──────────────────────────┤                            │             │
```

### 4. Hook 仕様

`~/.claude/settings.json` を container image に焼き込み、以下を登録する。

| Hook | matcher | 役割 | 期待 stdin | 出力 |
|------|---------|------|------------|------|
| `SessionStart` | startup / resume / clear / compact | `agent_outputs` に `event_type='session_start'` を 1 件 insert。`payload.session_id` を `messages.message_data.session_id` へ反映。`additionalContext` として system prompt を stdout に出す | Claude Code 生成 JSON | additionalContext (plain text) |
| `UserPromptSubmit` | (全プロンプト) | dispatch の `delivered_at` が NULL なら埋める。`payload` を `agent_outputs` に append | Claude Code 生成 JSON | exit 0 |
| `PreToolUse` / `PostToolUse` | 全ツール | tool 呼び出しイベントを `agent_outputs` に append (event_type `pre_tool_use` / `post_tool_use`) | Claude Code 生成 JSON | exit 0 |
| `Stop` | — | (a) 直近 dispatch を `consumed`、`agent_outputs` に `event_type='stop', status='final'` を insert。(b) 同会話に pending dispatch があれば、それを `delivered` にして `{"decision":"block","reason":<prompt>}` を stdout。 (c) 無ければ最大 N 秒 LISTEN/NOTIFY で待機 → なお無ければ exit 0 (=正常停止して container は idle に入る) | Claude Code 生成 JSON | block JSON or exit 0 |

実装言語は bash + `psql` ではなく **Go 製の単一バイナリ
`cc-hook-bridge`** を image に同梱する方針とする。理由:

- psql の認証 (Cloud SQL IAM auth) をシェルから扱うと複雑化する
- multi-agent-shogun の bash hook 経由で起きた quoting バグ
  (`stop_hook_inbox.sh` 内のコメント参照) を構造的に回避できる
- `messages.message_data` の JSONB 部分更新を型安全に書ける

`cc-hook-bridge <event-name>` を `settings.json` の command として登録する。

### 5. cc-tunnel 側の変更

| ファイル | 変更 |
|----------|------|
| `apps/cc-tunnel/internal/db/migrations/010_create_agent_dispatch.sql` | 新規。`agent_dispatches` / `agent_outputs` テーブル DDL |
| `apps/cc-tunnel/internal/api/message_service.go` | `POST /messages` で `agent_dispatches` を INSERT。`remoteclient.Execute` 呼び出しは削除し、cc-remote-agent へは「container を起動・claude を生かしておく」指示のみ送る |
| `apps/cc-tunnel/internal/agent_outputs/` (新設) | `agent_outputs` の polling/集約。message_data.content_blocks への fold |
| `apps/cc-tunnel/internal/remoteclient/client.go` | `Execute` を削除。代わりに `EnsureAgentRunning(conversationID)` を追加 |

frontend (`GET /conversations/{id}` 1 秒ポーリング) は変更不要。
`messages.status` の遷移条件だけが「ndjson 終了」から
「`agent_dispatches.status = consumed`」に変わる。

### 6. cc-remote-agent 側の変更

| ファイル | 変更 |
|----------|------|
| `apps/cc-remote-agent/cmd/cc-remote-agent/main.go` | 起動時に entrypoint として `claude --resume <stored_session_id>` (もしくは新規) をバックグラウンドで起動。stdin/stdout は PTY (`creack/pty`) で保持し、container 内で生かし続ける |
| `apps/cc-remote-agent/internal/claude/executor.go` | `runStream` / `StreamToWriter` を削除。代わりに「初回 prompt を PTY stdin に書き込む」だけのヘルパに縮退 |
| `apps/cc-remote-agent/internal/api/handler.go` | `/execute` を削除。`/agent/start` (PTY 起動) と `/agent/kick` (初回 prompt 注入) に置き換える |
| `apps/cc-remote-agent/cmd/cc-hook-bridge/main.go` | 新規。Cloud SQL に接続して dispatch/outputs を読み書きする CLI |
| `apps/cc-remote-agent/Dockerfile` | `cc-hook-bridge` を `/usr/local/bin/` へ COPY。`~/.claude/settings.json` を image に焼き込み |

PTY を採用するのは、`claude` がインタラクティブ TUI として TTY を期待
するため (auth フローと同じ理由)。stdout は hook 経由で DB に流すので
読み捨てて良い。

### 7. 認証情報の取り扱い

`cc-hook-bridge` が Cloud SQL に接続する経路:

- **本番 (Cloud Run + GCE)**: container は GCE VM の SA を継承する。
  Cloud SQL Auth Proxy を container-manager サイドで起動し、Unix socket
  を bind-mount する。cc-hook-bridge は localhost socket 経由で接続。
- **ローカル開発 (compose)**: 既存 `postgres` サービスに `pgx` で直接
  接続。DSN は環境変数で渡す。

cc-tunnel の credentials テーブル (migration 007) には触れない。
agent 通信用テーブルは独立した責務とする。

---

## 検討した代替案

### A. file ベース mailbox (multi-agent-shogun 方式そのまま)

container 内の `/workspace/queue/inbox/<conv>.yaml` を flock で読み書き
する案。実装が一番軽い (cc-hook-bridge の DB ロジックが不要)。

却下理由:

- container は session 単位で短命 (idle 検知で停止)。停止直前に flush
  しても tmpfs の内容は失われる。
- per-session container 同士は GCE VM が違えば file を共有できない。
  multi-VM スケジューリング (`session_endpoints.vm_instance_id`) と相性
  が悪い。
- frontend は cc-tunnel 経由で DB を見ているので、結局 file → DB の
  二重書き込みが必要。

### B. 現状維持 + `claude` プロセスを `bash -i` 的に常駐させて stdin pipe で複数 prompt 流し込み

`claude -p` を捨て、`script(1)` 等で PTY を確保した対話モードを長寿命
化するだけで HTTP ストリームは現状の仕組みを維持する案。

却下理由:

- HTTP コネクションが切れたら同様に部分応答が消える (P3 解消せず)。
- hook を使わないと sub-agent / MCP の遷移を観測できない。
- frontend は依然として「ndjson 全部受け切る」前提でしか動かない。

### C. Redis Streams / Pub/Sub の採用

PostgreSQL の代わりにストリーム特化したミドルウェアを採用する案。

却下理由:

- 既に `pgxpool` + goose の運用が確立しており、新規依存を増やすコスト
  が高い。
- agent_outputs は「同一 dispatch_id にぶら下がる順序付き事象」なので
  RDB の `(dispatch_id, event_seq)` UNIQUE で十分。スループット要件
  (1 会話で秒間数十イベント) は PostgreSQL で問題なく捌ける。
- 故障時の再構築 (replay) を考えると、関連エンティティ (messages,
  conversations) と同じ DB に居る方が運用が楽。

### D. Stop hook で polling せず NOTIFY/LISTEN にする

cc-tunnel が `pg_notify('agent_inbox_<convID>', '')` を打ち、Stop hook
内の `cc-hook-bridge listen` がブロッキング受信する案。

→ **採用方向**だが、ADR 範囲としては「Stop hook 内で N 秒の wait を
する」抽象に留め、polling/NOTIFY の切替は実装フェーズで判断する。
LISTEN は connection-bound で longrunning なので Cloud SQL connection
枠を消費する点だけ留意する。

---

## 期待される効果

| 番号 | 効果 |
|------|------|
| E1 | P1 解消: `claude` の起動回数が「container 起動時の 1 回」になる。ターン間レイテンシが純粋にモデル推論時間に縮む |
| E2 | P2 解消: hook と長寿命セッションが揃うので、MCP server や sub-agent (`Task` ツール) の永続接続を活用できる |
| E3 | P3/P4 解消: 部分応答は `agent_outputs` に append-only で残り、container が落ちても DB から再構築できる。frontend の polling 経路に変更不要 |
| E4 | P5 解消: `--resume` の session_id は SessionStart hook が DB に書き込むので、cc-remote-agent 内のフォールバックロジックを削除できる。フォールバックは「container 再作成 + 同 conversation_id で `SessionStart` 経由で resume」という統一フローになる |
| E5 | 副次効果: hook 出力が DB に蓄積されるので、ツール呼び出し履歴・コスト・思考ブロックの観測が SQL クエリで完結する |

---

## 残課題・リスク

| ID | 内容 | 軽減策 |
|----|------|--------|
| R1 | Stop hook が DB に到達できない (Cloud SQL 障害) 場合、claude プロセスが「無条件 block」で固まる | `cc-hook-bridge` に最大 wait timeout を必ず設ける。timeout 後は exit 0 で正常停止 |
| R2 | hook 内から `claude` の動作を書き換えるため、Claude Code の minor update で hook 契約 (stdin JSON 形式) が壊れるリスク | image の `claude` バージョンを pin。`cc-hook-bridge` 側で受信 JSON のバージョン assertion を実装 |
| R3 | container が idle 検知で停止 → 次ターンで cold start する場合、`SessionStart` hook が `--resume` 用の session_id を DB から拾えるかが鍵 | migration で `conversations` に `last_session_id` カラムを足すか、`messages.message_data.session_id` を最新 assistant メッセージから引く既存ロジックを維持する。後者で十分なので前者は採用しない |
| R4 | agent_outputs の event_seq を hook 側でどう採番するか | dispatch 単位で `MAX(event_seq)+1` を `SELECT ... FOR UPDATE` で取得する。同一 dispatch を並行 hook が触ることは無い (Claude Code は逐次的に hook を発火) ので競合は事実上起きない |
| R5 | `claude -p` を使った既存 e2e テスト (`apps/cc-remote-agent/internal/api/e2e_test.go` 等) が全滅する | 本 ADR の実装フェーズで PTY ベースの e2e に差し替える。テスト用に「hook が即時 exit して固定 JSON を返す」モック版 `cc-hook-bridge` を用意する |
| R6 | container を長寿命化すると、メモリ消費・credential 漏洩面積が増える | per-conversation 隔離は維持。idle timeout (既存の `session_endpoints.last_activity`) を agent_dispatches.consumed_at と連動させ、最終応答から N 分で確実に container を破棄する |

---

## 実装ステップ (見通し)

1. **migration 010**: `agent_dispatches` / `agent_outputs` テーブル追加。
   既存コードからは未参照のまま入れる (リバート容易)。
2. **`cc-hook-bridge` の skeleton**: SessionStart / Stop だけ実装し、
   ローカル compose 環境で長寿命 `claude` + hook → DB INSERT を確認。
3. **cc-tunnel 側の dispatch 投入**: `POST /messages` で
   `agent_dispatches` への INSERT を「ndjson 経路と並走」させる
   (shadow write)。差分を観察。
4. **cc-tunnel 側の output 集約**: `agent_outputs` → `messages.message_data`
   の fold ロジックを実装。frontend には触らない。
5. **`/execute` の廃止**: shadow write の整合性を確認後、`StreamToWriter`
   と `remoteclient.Execute` を削除。`/agent/start` `/agent/kick` を新設。
6. **PTY 常駐**: cc-remote-agent の entrypoint を変更し、container 起動
   時に `claude` を PTY 経由で常駐させる。auth フロー (creack/pty 経由)
   の既存実装を流用する。

各ステップで rollback ポイントを必ず確保する。

---

## 決定の根拠 (要約)

- `claude -p` は「ステートレス・1 ターン完結」モデルに最適化されている
  が、本サービスは「同一会話を跨いだ長時間セッション」を提供する。
  モデルのライフタイムと API 契約をそろえる方が素直。
- hook + DB は multi-agent-shogun での運用実績がある (file 部分のみ DB
  に差し替える)。ゼロから設計するわけではない。
- frontend は DB ポーリング前提のため、agent 側を DB 駆動に揃えると
  全体が「DB を真とする」一貫した state machine になる。
