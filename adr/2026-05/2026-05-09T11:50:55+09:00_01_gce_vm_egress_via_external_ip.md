# ADR: cc-tunnel-managed VM の egress を ephemeral external IP で確保する

**日時**: 2026-05-09T11:50:55+09:00
**ステータス**: 採用 (実装済み)
**関連ブランチ**: `claude/fix-remote-agent-container-GJjvW`
**関連 PR**: #57
**関連 ADR**:
- `2026-05-06T11:00:45+09:00_01_container_manager_on_vm.md` (container-manager 導入)
- main にマージ済 PR #58: `feat(gce): attach a dedicated SA to cc-tunnel-managed VMs`

---

## 背景

### 発生中の不具合

cc-tunnel が `PrepareForRelogin` で VM 上の container-manager に
`POST /v1/agents` を送ると、container-manager が AR からの image pull
で以下のエラーを返す。

```
failed to pull image us-central1-docker.pkg.dev/.../cc-remote-agent:latest:
Get "https://us-central1-docker.pkg.dev/v2/": dial tcp 142.251.x.x:443:
i/o timeout
```

### 段階的に判明した根本原因

1. **第一段階 (PR #58 で解消済み)**: VM に Service Account がアタッチ
   されていなかった。`gcloud compute instances create` と異なり Compute
   Engine API は SA を暗黙付与しないため、metadata server からトークンを
   取れず AR 認証以前に SA が無い状態だった。
2. **第二段階 (本 ADR)**: SA を付けても **i/o timeout** が解消しない。
   container-manager のログから VM 自体に外部到達経路が無いことが判明:
   - VM は `NetworkInterface.AccessConfigs` 未設定で **外部 IP なし**で
     起動している (`apps/cc-tunnel/internal/gce/sdk_client.go`)
   - 配置先の `default` network のリージョン subnet は **Private Google
     Access が無効** (auto-mode 既定)
   - Cloud NAT / Cloud Router のリソースも存在しない

つまり VM は GCP の内部ネットワーク内に閉じており、`*.pkg.dev` を含む
あらゆる外部到達ができない状態だった。

---

## 決定事項

VM の `NetworkInterface` に `AccessConfigs` (`ONE_TO_ONE_NAT` + ephemeral
external IP) を付与し、VM 単独で外部に出られるようにする。
Cloud NAT は採用しない。

### 1. ephemeral external IP の付与

`apps/cc-tunnel/internal/gce/sdk_client.go` の `networkInterface` ヘルパー
で全 VM に以下を付ける:

```go
AccessConfigs: []*computepb.AccessConfig{
    {
        Name: proto.String("External NAT"),
        Type: proto.String("ONE_TO_ONE_NAT"),
    },
},
```

- ephemeral IP のためアドレスは VM 起動毎に変動。cc-tunnel → VM の通信は
  **内部 IP (Direct VPC Egress 経由)** を引き続き使うのでアドレス変動の
  影響を受けない。
- 使用中 VM の ephemeral IP には追加課金が発生しない (GCP 料金体系)。
  cc-tunnel は idle で VM を削除する設計のため、停止中保持に伴う課金も
  問題にならない。

### 2. cc-remote-agent-vm subnet (PGA 有効) の保持

外部 IP を付ける方針に切り替えた後も、PGA を有効にした dedicated subnet
`cc-remote-agent-vm` (`terraform/modules/cc-tunnel/vpc.tf`) は残す。

- 採用理由:
  - 将来 NAT 採用 / 外部 IP 廃止に切り替える際の選択肢を温存
  - subnet を分けておくことで cc-tunnel-managed VM の IP レンジ管理が明確
  - PGA は無効化するコストが低いので残しても運用負荷は無い
- 注意: 外部 IP がある状態では Google API 宛トラフィックは public 経路で
  解決される。**PGA は実質効かない**が無害。

`GCE_VM_SUBNETWORK` env を Cloud Run に渡し、`DockerGCEConfig.VMSubnetwork`
→ `gce.CreateInstanceRequest.Subnetwork` の経路で VM 作成時に subnet を
明示する。

### 3. 認証情報の流れ

PR #58 で導入された VM SA (`vm_runtime_sa` / `roles/artifactregistry.reader`)
の構成は維持する。

- container-manager は `/var/run/docker.sock` 経由の `ImagePull` で AR の
  認証情報を metadata server から取得 → `X-Registry-Auth` として dockerd
  に渡す。
- cc-tunnel ↔ container-manager の通信路に AR 認証情報は流れない
  (ADR `2026-05-06_container_manager_on_vm` の方針継承)。

---

## トレードオフと却下案

### 却下: Cloud NAT (Cloud Router + NAT Gateway)

- メリット:
  - VM が外部 IP を持たないのでネットワーク露出がゼロに近い
  - 静的な egress IP が得られる (許可リスト連携が必要ならメリット)
- 却下理由:
  - **コスト**: NAT Gateway は 1 GB あたりのデータ処理料金 + Cloud Router
    のアイドル課金が発生する。本サービスは VM 数が少なく PoC 段階のため
    固定費が割に合わない。
  - 当面 egress 制限を IP ベースで掛ける要件もない。

### 却下: VM image / cc-remote-agent image にすべて焼き込む

- メリット: VM が外部に出られなくても動く (PGA だけで AR pull は可能)
- 却下理由:
  - cc-remote-agent コンテナ内で `git clone` / `pip install` /
    `npm install` 等が走る前提。事前に全依存をイメージに焼き込むのは現実
    的でない。
  - 開発者体験 (DX) を著しく損なう。

### 却下: PGA だけで運用 (本 ADR の前段で実装したが取り下げ)

- メリット: 外部 IP を持たないのでネットワーク露出がない
- 却下理由:
  - PGA 対象は Google API のみ。container-manager 自身の image pull は
    解決するが、cc-remote-agent コンテナの実行時依存 (上述) を満たせない。
  - PoC 段階で「外部 API を一切使わない」前提を置けない。

---

## セキュリティ考慮事項

外部 IP を付ける以上、VM は public internet から疎通可能な状態になる。
firewall 構成を再確認すること。

### 現状の firewall

| ルール | 由来 | source | dest port | 影響 |
|--------|------|--------|-----------|------|
| `cc-tunnel-container-manager` | 本リポ | VPC connector subnet | 9090/tcp on tag `cc-tunnel-agent` | OK (内部のみ) |
| `default-allow-ssh` | GCP デフォルト | `0.0.0.0/0` | 22/tcp on `default` 全 VM | **要検討** |
| `default-allow-rdp` | GCP デフォルト | `0.0.0.0/0` | 3389/tcp on `default` 全 VM | 影響軽微 (Linux VM) |
| `default-allow-icmp` | GCP デフォルト | `0.0.0.0/0` | ICMP on `default` 全 VM | 影響軽微 |

### 推奨ハードニング (本 ADR スコープ外 / フォローアップ)

- `default-allow-ssh` を **target_tags ベースに絞る** か、cc-tunnel-managed
  VM に `no-ssh` 系の tag + deny rule を当てる。SSH 不要な運用なら 22 を
  完全閉鎖したい。
- container-manager の 9090 は target_tags `cc-tunnel-agent` で
  VPC connector subnet 限定 ingress になっており、外部 IP の有無に
  関わらず保護されている (要件: container-manager 側の HTTP 認証は
  別レイヤで担保)。

---

## 影響範囲

### 改修

| パス | 変更内容 |
|------|---------|
| `apps/cc-tunnel/internal/gce/client.go` | `CreateInstanceRequest.Subnetwork` 追加 |
| `apps/cc-tunnel/internal/gce/sdk_client.go` | `networkInterface` ヘルパー新設 / `AccessConfigs` (ephemeral external IP) 付与 / `Subnetwork` を反映 |
| `apps/cc-tunnel/internal/provider/dockergce/provider.go` | `DockerGCEConfig.VMSubnetwork` 追加 / `CreateInstance` 呼び出しに反映 |
| `apps/cc-tunnel/cmd/cc-tunnel/main.go` | `GCE_VM_SUBNETWORK` env を読み取り `DockerGCEConfig` に流す |
| `terraform/modules/cc-tunnel/variables.tf` | `cc_remote_agent_subnet_cidr` 変数追加 |
| `terraform/modules/cc-tunnel/vpc.tf` | `google_compute_subnetwork.cc_remote_agent_vm` (PGA 有効) を追加 / zone から region 派生の `local.gce_region` を導入 |
| `terraform/modules/cc-tunnel/main.tf` | Cloud Run の env に `GCE_VM_SUBNETWORK` を追加 |

### 新規追加・削除

なし。

---

## 残課題・確認事項

- [ ] `default-allow-ssh` を tag scoping するか、cc-tunnel-managed VM 側に
      deny rule を当てる対応を別 PR で検討する。
- [ ] 将来 VM 数が増えて Cloud NAT のコストが妥当になったタイミングで、
      外部 IP 廃止 + Cloud NAT 採用にスイッチする。本 ADR で温存した
      `cc-remote-agent-vm` subnet (PGA 有効) は **そのまま再利用可能**。
- [ ] cc-remote-agent コンテナの外部到達要件 (どのドメイン / プロトコル
      が必要か) を整理し、必要なら egress firewall (denyRules) で絞る。

---

## 警告

- ephemeral external IP は **VM 再作成のたびに変動**する。許可リスト方式
  での外部 API 利用には不向き。許可リストが必要になったら static IP +
  Cloud NAT に切り替えること。
- `default` network の auto-mode 既定 firewall (特に SSH) が外部 IP 付き
  VM に直撃する。新環境を立てる際は必ず firewall を点検する。
