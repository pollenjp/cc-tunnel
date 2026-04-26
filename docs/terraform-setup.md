# Terraform セットアップガイド

## 概要

cc-tunnel の GCP インフラは Terraform (Terragrunt) で管理されている。
本ドキュメントは初回セットアップ手順と既知の注意点を説明する。

## ディレクトリ構成

```
terraform/
├── root.hcl              # 共通設定（impersonation、provider 生成）
├── modules/
│   ├── prepare_terraform_sa/  # Terraform Runner SA とその IAM を管理
│   └── artifact_registry/     # Artifact Registry リポジトリを管理
├── prepare/
│   └── local/
│       └── terraform_sa/      # SA の apply（ADC 直接、impersonation なし）
└── live/
    └── local/
        ├── init/              # GCP API 有効化
        └── artifact_registry/ # Artifact Registry リポジトリ作成
```

## Apply 順序

必ず以下の順序で apply すること:

### 1. prepare/local/terraform_sa（ADC 直接で実行）

Terraform Runner SA を作成し、必要な IAM ロールを付与する。
この unit は impersonation を使わず殿の ADC 直接で実行される（root.hcl の例外条件）。

```bash
cd terraform/prepare/local/terraform_sa
terragrunt plan   # 差分確認（必須）
terragrunt apply
```

### 2. live/local/init（SA impersonation で実行）

GCP API（Artifact Registry、Compute Engine 等）を有効化する。

```bash
cd terraform/live/local/init
terragrunt apply
```

### 3. live/local/artifact_registry（SA impersonation で実行）

Artifact Registry リポジトリを作成する。

```bash
cd terraform/live/local/artifact_registry
terragrunt plan
terragrunt apply
```

## 前提条件と必要な権限

### 殿の ADC ユーザー（polleninjp@gmail.com）に必要なロール

| ロール | 用途 |
|--------|------|
| roles/iam.serviceAccountTokenCreator | SA の impersonation |
| roles/resourcemanager.projectIamAdmin | SA への IAM 付与（prepare 再 apply 時） |

### Terraform Runner SA に付与されるロール

`terraform/modules/prepare_terraform_sa/main.tf` の `sa_roles` で管理:

| ロール | 用途 |
|--------|------|
| roles/compute.admin | GCE VM 管理 |
| roles/compute.osLogin | GCE VM SSH |
| roles/iam.serviceAccountUser | VM へのSA アタッチ |
| roles/artifactregistry.admin | Artifact Registry 管理 |

## 既知の問題と対処

### IAM_PERMISSION_DENIED: artifactregistry.repositories.create

**原因**: Terraform Runner SA に `roles/artifactregistry.admin` が付与されていない。
`terraform/modules/prepare_terraform_sa/main.tf` の `sa_roles` にコメントアウトされた行がないか確認する。

**解決手順**:
1. `terraform/modules/prepare_terraform_sa/main.tf` で `roles/artifactregistry.admin` がコメントアウトされていれば解除
2. `prepare/local/terraform_sa` を再 apply（SA への権限追加）
3. `live/local/artifact_registry` を再 apply

### SA 名の変更を防ぐ

`random_string.unique_id` は terraform state で管理される。
state が正常であれば再 apply しても SA 名は変わらない。
`terragrunt plan` で差分を必ず確認すること。

## docker_gce Provider との関係

`cmd_cctunnel_docker_gce_impl` で実装された DockerGCEProvider は、
Artifact Registry に push された `cc-remote-agent` イメージを使用して
GCE VM 上でコンテナを起動する。

Artifact Registry セットアップが完了していることが docker_gce Provider
本番運用の前提条件となる。
