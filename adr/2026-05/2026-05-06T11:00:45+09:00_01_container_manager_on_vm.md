# ADR: VM 上 container-manager の導入による Docker 操作の集約

**日時**: 2026-05-06T11:00:45+09:00
**ステータス**: 提案
**関連ブランチ**: `claude/fix-docker-image-error-yzFWL`
**関連 ADR**: `2026-04-29T15:55:56+09:00_01_docker_gce_multi_container.md`

---

## 背景

### 発生中の不具合

Cloud Run (`local-ehfv-cr`) 上の `cc-tunnel` が `PrepareForRelogin` を実行する際、
GCE VM の Docker daemon に対する `ContainerCreate` が以下のエラーで失敗している。

```
dockerhost: create container "cc-remote-agent-…": Error response from daemon:
No such image: us-central1-docker.pkg.dev/cc-tunnel-local/cc-tunnel/cc-remote-agent:latest
```

### 根本原因

1. VM 起動時 `startup-script` の `docker pull %s || true`
   (`apps/cc-tunnel/internal/provider/dockergce/provider.go:428-432`) が、
   VM 上の Docker daemon に **Artifact Registry 用の credential helper が
   未設定**であるため認証失敗 → `|| true` により握り潰されている。
2. `apps/cc-tunnel/internal/dockerhost/client.go` の `RunAgentContainer` は
   `docker pull` 相当を行わず `ContainerCreate` を直接発行するため、
   ローカルにイメージが無いと「No such image」で失敗する。

### 既存アーキテクチャの構造的課題

ADR `2026-04-29_docker_gce_multi_container` で採用された
「Cloud Run → VM の `dockerd tcp://0.0.0.0:2375` 直叩き」方式は、
今回の不具合をきっかけに以下の課題が顕在化した。

| ID | 課題 | 影響 |
|----|------|------|
| C1 | dockerd の TCP 2375 が **無認証**で VPC 内に晒されている | 内部からの権限昇格・コンテナ任意起動のリスク |
| C2 | レジストリ認証ロジックを置く場所が曖昧 (Packer / startup-script / dockerhost クライアント) | 認証方式の更新でVM image 再焼きが必要になりがち |
| C3 | Cloud Run → VM のリクエストに毎回 `X-Registry-Auth` でトークンを乗せると、cc-tunnel と VM の間で credentials が常に流れ続ける | 殿の指示「認証情報を VM に受け渡したくない」と整合しない |
| C4 | dockerd Engine API は表面積が大きく、用途が pull/run/stop に限定されているのに不必要な操作が露出する | 攻撃面の最小化原則に反する |

---

## 決定事項

VM 上に `apps/container-manager` という常駐コンテナを配置し、
**Docker 操作 (image pull / agent コンテナ lifecycle) をすべてここに集約する。**

cc-tunnel は dockerd を直接叩かず、container-manager の専用 API を呼ぶ。

### 1. 配置と役割

- **アプリ名**: `container-manager`
- **コードパス**: `apps/container-manager/`
- **配置場所**: 各 GCE VM 上に常駐するコンテナ (1 VM につき 1 個)
- **責務**:
  - cc-remote-agent イメージの pull (Artifact Registry 認証込み)
  - cc-remote-agent コンテナの起動 / 停止 / 削除
  - 必要に応じてコンテナ状態の参照
- **非責務**:
  - 任意イメージの pull / run (= dockerd Engine API のフル機能を提供しない)
  - Docker network / volume の動的管理 (今のところ不要)

### 2. レジストリ認証

- container-manager コンテナは **VM の Service Account の権限を継承**して
  metadata server 経由でアクセストークンを取得する。
- VM SA には現状どおり `roles/artifactregistry.reader` のみを付与
  (`terraform/modules/cc-tunnel/cc-remote-agent.tf:99` 既存設定)。
- 取得したトークンを Docker Engine API の `X-Registry-Auth` ヘッダに乗せて
  Unix socket 経由でローカルの dockerd に `ImagePull` を発行する。
- **cc-tunnel ↔ container-manager の通信路には Artifact Registry 認証情報を
  一切流さない。** (殿の指示への直接対応 / C3 解消)

### 3. dockerd への接続方式

- container-manager は `/var/run/docker.sock` (Unix socket) でローカル dockerd
  に接続する。
- これに伴い **VM の `dockerd -H tcp://0.0.0.0:2375` リスナーを廃止する**。
  (Packer の systemd drop-in を変更)
- VPC 内の TCP 2375 ファイアウォールルール
  (`allow-cc-tunnel-docker-api`) も削除する。
- Cloud Run → VM の通信は container-manager が listen するポート (例: 9090)
  のみ。**ここは認証付き**。(C1 解消)

### 4. API 形式: ユースケース特化 RPC

Docker Engine API のプロキシではなく、用途を絞った RPC を提供する。
攻撃面最小化と意図の明示性のため。(C4 解消)

最小エンドポイント:

| メソッド | パス | 用途 |
|---------|------|------|
| `POST` | `/v1/agents` | 指定 image を pull し、cc-remote-agent コンテナを起動 |
| `DELETE` | `/v1/agents/{name}` | コンテナを停止・削除 |
| `GET` | `/v1/agents/{name}` | コンテナ状態 (任意 / future) |
| `GET` | `/healthz` | container-manager 自身の生存確認 |

`POST /v1/agents` のリクエスト body:

```json
{
  "image": "us-central1-docker.pkg.dev/.../cc-remote-agent:latest",
  "name": "cc-remote-agent-<conversation_id>",
  "host_port": 9091,
  "container_port": 9090,
  "memory_mib": 512,
  "nano_cpus": 500000000
}
```

レスポンスでコンテナ ID と起動結果を返す。

### 5. cc-tunnel ↔ container-manager 認証

既存の `cc-tunnel ↔ cc-remote-agent` の認証方式に揃える。
詳細は実装フェーズで再確認するが、ベース方針:

- VPC 内通信前提 (Direct VPC Egress / VPC Connector)
- 共有 secret によるトークンヘッダ、もしくは Cloud Run SA の ID トークン検証
- container-manager は VM SA とは **別の認証層**で cc-tunnel を識別

(注: 既存 cc-remote-agent の auth 廃止 ADR
`2026-04-29T12:55:24_cc_remote_agent_auth_retire.md` の方針との整合は
実装フェーズで確認・必要なら別 ADR を起こす。)

### 6. container-manager イメージの配布

**Packer イメージへの焼き込み (`docker save` / `docker load`)** を採用する。

- 理由:
  - chicken-and-egg 回避 (registry 認証なしで起動できる必要がある)
  - container-manager は更新頻度が低いコンポーネントのため Packer 再焼きで許容可能
  - ネットワーク疎通や public registry 可用性に依存しない
- 起動方式: Packer で `systemd` unit を仕込み、VM 起動時に
  `docker run --restart=always --network=host -v /var/run/docker.sock:/var/run/docker.sock ...`

### 7. 既存 dockerhost パッケージの扱い

- `apps/cc-tunnel/internal/dockerhost/` は **container-manager クライアントに置き換え** (新パッケージ名候補: `internal/cmclient`)。
- インターフェース `ContainerManager` は維持し、実装を差し替える形にしたい。
- mock も併せて差し替える。

---

## 影響範囲

### 新規追加

| パス | 内容 |
|------|------|
| `apps/container-manager/` | 新規 Go アプリ。Docker SDK + HTTP server |
| `apps/container-manager/Dockerfile` | container-manager イメージビルド |
| `apps/cc-tunnel/internal/cmclient/` (仮) | container-manager 向け HTTP クライアント |
| `adr/2026-05/2026-05-06T11:00:45+09:00_01_container_manager_on_vm.md` | 本 ADR |

### 改修

| パス | 変更内容 |
|------|---------|
| `apps/vm-image/packer.pkr.hcl` | container-manager イメージの `docker load` / systemd unit の追加 / `dockerd -H tcp://0.0.0.0:2375` の削除 |
| `apps/cc-tunnel/internal/dockerhost/client.go` | container-manager クライアントへ置き換え (or 新パッケージへ移設) |
| `apps/cc-tunnel/internal/provider/dockergce/provider.go` | `buildStartupScript` の `docker pull` 削除、新クライアント呼び出しに変更 |
| `terraform/modules/cc-tunnel/main.tf` 他 | FW ルール `allow-cc-tunnel-docker-api` の削除 / 新規ポート許可ルール追加 / VM SA への AR reader 維持 |
| `apps/cc-tunnel/internal/dockerhost/mock.go` | 新クライアント向け mock に差し替え |

### 削除

| パス | 理由 |
|------|------|
| dockerd の TCP 2375 リスナー (Packer 設定) | container-manager 経由に統一 |
| FW ルール `allow-cc-tunnel-docker-api` | 上記に伴い不要 |
| `provider.buildStartupScript` 内の `docker pull` | container-manager が pull を担うため不要 |

---

## トレードオフと却下案

### 却下: Packer に `docker-credential-gcr` を仕込む案

- メリット: 実装量最小。既存アーキの延長
- 却下理由:
  - C1 (TCP 2375 無認証) を解消しない
  - 認証ロジックが Packer image に焼き付き、更新が重い
  - 殿の方針 (管理コンテナ常駐) と整合しない

### 却下: cc-tunnel から `X-Registry-Auth` を VM に送る案

- メリット: VM 側の認証セットアップ不要
- 却下理由:
  - **cc-tunnel → VM に認証情報が流れる** (殿の明示的 NG)
  - Cloud Run SA に Artifact Registry 権限を付ける必要があり、責務が分散

### 却下: dockerd Engine API のプロキシ (認証だけ被せる案)

- メリット: API を新規定義しなくて良い
- 却下理由:
  - 攻撃面が広い (任意 image 起動・コンテナ任意操作)
  - 用途が pull/run/stop に限定されているのに不要機能を露出させる
  - container-manager 内の権限境界を薄くしてしまう

---

## セキュリティ考慮事項

1. **dockerd TCP 2375 廃止により C1 解消**。dockerd は Unix socket のみ
   listen し、container-manager コンテナだけが操作可能。
2. **container-manager の API 認証**は cc-tunnel ↔ cc-remote-agent と同等水準で必須。実装時に共有 secret / ID トークン検証のどちらかを選定する。
3. **`/var/run/docker.sock` マウントは特権相当**。container-manager コンテナの実行ユーザ・ネットワーク露出を最小化し、外部 (cc-tunnel 以外) からの API アクセスを FW で遮断する。
4. **VM SA の AR reader 権限のみ**で動作させる。container-manager は AR writer や他 GCP API を要求しない。

---

## 残課題・確認事項

- [ ] container-manager と cc-tunnel 間の認証方式を確定する
      (既存 cc-remote-agent との整合性を含む)。
- [ ] container-manager の更新フロー: Packer 再焼き必須か、cc-tunnel から
      アップグレード API を持たせるか (今回は前者で十分と判断)。
- [ ] `internal/dockerhost` を残す or `internal/cmclient` に rename する
      かの最終判断。インターフェース名 `ContainerManager` は維持する。
- [ ] 本 ADR スコープが大きいため、本ブランチ
      (`claude/fix-docker-image-error-yzFWL`) で全実装するか、別ブランチに
      切り出すかを決定する。

---

## 警告

- container-manager の API 仕様変更は **VM image 再焼き + Cloud Run 再デプロイの両方**を要するため、**後方互換性を意識した version path (`/v1/`)** を最初から導入する。
- Packer image に container-manager を焼き込む方式のため、container-manager の致命的バグは VM image を新しいバージョンに切り替えるまで解消できない。リリース前の動作確認を慎重に行うこと。
