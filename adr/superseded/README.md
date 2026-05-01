# superseded ADRs

このディレクトリには、後続の ADR によって明確に置き換えられた決定記録を集約している。

検索ノイズ低減と「どれが現状の正か」の判断コスト削減が目的。
履歴的価値はあるためファイルは削除せず、ここに移動する。

## 移動の基準

以下のいずれかを満たすものを移動対象とする:

1. **ファイル名のチェーン**: `NN_<同名>.md`(NN=01,02,...) で
   後続番号を持つ ADR が存在する。最終番号のみ `2026-MM/` に残す。
2. **明示的な後継**: ADR 本文 / 後続 ADR の本文に "supersedes" / "置き換え" /
   "廃止" などの記載がある。

## 現在の収録対象

| 主題 | 移動元の最終ファイル(現役) |
|---|---|
| terraform_modules_diff_sync_001 | `2026-04/2026-04-27T01:01:33+09:00_03_terraform_modules_diff_sync_001.md` |
| terraform_frontend_deploy_001 | `2026-04/2026-04-27T14:36:20+09:00_07_terraform_frontend_deploy_001.md` |
| terraform_cloud_sql_001 | `2026-04/2026-04-27T14:56:42+09:00_04_terraform_cloud_sql_001.md` |
| terraform_frontend_backend_url_fix_001 | `2026-04/2026-04-28T11:41:01+09:00_05_terraform_frontend_backend_url_fix_001.md` |
| moby_rename_001 | `2026-04/2026-04-26T10:53:09+09:00_01_moby_rename_001_v2.md` |

## このディレクトリのファイルを参照する場合

- 履歴的経緯の確認には有用だが、現在の正としては扱わない。
- 新しい ADR を書くときは、この配下のファイルを引用するより、現役の最終 ADR を
  Related ADR として引用する方が望ましい。
