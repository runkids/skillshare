---
sidebar_position: 3
---

# check

変更を適用せずに、トラッキング対象のリポジトリとインストール済み Skill の更新有無を確認します。

```bash
skillshare check                      # すべてのリポジトリと Skill を確認
skillshare check my-skill             # 単一の Skill を確認
skillshare check a b c                # 複数の Skill を確認
skillshare check --group frontend     # frontend/ 配下のすべての Skill を確認
skillshare check x -G backend         # 名前とグループを混在させる
skillshare check --json               # 機械可読な出力
```

## 使うタイミング

### 更新前に

`update` を実行する前に、何が変わるかをプレビューする。

```bash
skillshare check         # 更新があるものを確認
skillshare update --all  # 更新を適用
skillshare sync          # 変更を配布
```

### CI/CD パイプライン

CI 内で古くなった Skill を確認する。

```bash
result=$(skillshare check --json)
# JSON をパースして古い Skill を検出する
```

## 実行内容

`check` は Source ディレクトリを調べ、以下について更新状況を報告します。

1. **トラッキング対象のリポジトリ** — origin から fetch し、何コミット遅れているかを表示
2. **メタデータ付きのインストール済み Skill** — インストール済みのバージョンをリモートの HEAD と比較
3. **Stale な Skill** — アップストリームリポジトリからサブディレクトリが削除された Skill を検出
4. **ローカルパスからのインストール** — インストール元のパスにあるファイルを、インストール時に記録したファイルと比較（[ローカルパスからのインストール](#local-path-installs)を参照）
5. **ローカルの Skill** — インストールメタデータのない Skill を「local source」としてマーク（比較対象がない）
6. **Skill レベルの `targets` 検証** — SKILL.md の `targets` フロントマターフィールドにある未知の Target 名について警告

`update` とは異なり、`check` はファイルを一切変更しません。

## ローカルパスからのインストール {#local-path-installs}

ディスク上のディレクトリからインストールした Skill（`skillshare install /path/to/skill`）は、そのパスとコピーした各ファイルのハッシュを記録します。`check` はそのパスのファイルを再度ハッシュ化し、次のように報告します。

- **up to date** — ファイルがインストール時と一致している
- **update available** — インストール元のパスでファイルが変更、追加、または削除された
- **error** — インストール元のパスが存在しない（`local source not found: <path>`）

これは、App バンドル内の Skill のように、別のアプリケーションが配布・更新する Skill に便利です。

```bash
skillshare install /Applications/Surge.app/Contents/Resources/Skills/surge

# App の更新後:
skillshare check surge     # → Update available
skillshare update surge    # パスから再コピーし、セキュリティ監査を実行
skillshare sync
```

ファイルハッシュが記録される前にインストールされた Skill は、update または再インストールするまで「local source」のままです。 Project mode では、相対パス（`./vendor/my-skill` など）は project root を基準に解決されます。

ダッシュボードで、子 Skill を含むディレクトリのルートをインストールすると、そのルートの `SKILL.md` だけがコピーされます。その場合 `check` は `SKILL.md` だけを比較し、`update` も `SKILL.md` だけを再コピーします。

## 出力例

```
$ skillshare check
! _shared-rules   3 commits behind
! _design-system  uncommitted changes
! commit          update available · github.com/anthropics/skills
! old-helper      stale — no longer in the upstream repository

! Updates available for 1 repo, 1 skill, 2 up to date, 1 local skipped, 1 stale · 3.4s

Next
  skillshare update --all          pull the updates
  skillshare update --all --prune  remove stale skills
```

## 特定の Skill を確認する

すべてをスキャンする代わりに、名前を指定して 1 つ以上の Skill を確認できます。

```bash
skillshare check my-skill                # 単一の Skill
skillshare check skill-a skill-b         # 複数の Skill
```

グループディレクトリ内の更新可能な Skill をすべて確認するには `--group` / `-G` を使います。

```bash
skillshare check --group frontend        # frontend/ 配下のすべての Skill
skillshare check -G frontend -G backend  # 複数のグループ
skillshare check my-skill -G frontend    # 名前とグループを混在させる
```

位置引数がリポジトリや Skill そのものではなくグループディレクトリに一致する場合、自動的に展開されます。

```bash
skillshare check frontend               # グループとして自動検出
```

メタデータのない Skill（ローカルのみ）は、グループ展開時にスキップされます。

## オプション

| フラグ | 説明 |
|------|-------------|
| `--group`, `-G` `<name>` | グループ内の更新可能な Skill をすべて確認（繰り返し指定可） |
| `--project`, `-p` | Project レベルの Skill（`.skillshare/`）を確認 |
| `--global`, `-g` | グローバルの Skill（`~/.config/skillshare`）を確認 |
| `--json` | JSON として出力（スクリプト/CI 向け） |
| `--help`, `-h` | ヘルプを表示 |

:::tip 自動検出
`--project` も `--global` も指定しない場合、skillshare は自動検出します。カレントディレクトリに `.skillshare/config.yaml` が存在すれば Project mode、それ以外はグローバルモードになります。
:::

## JSON 出力

```bash
skillshare check --json
```

```json
{
  "tracked_repos": [
    {"name": "_team-skills", "status": "up_to_date", "behind": 0, "branch": "main"},
    {"name": "_shared-rules", "status": "behind", "behind": 3, "branch": "develop"}
  ],
  "skills": [
    {"name": "pdf", "source": "anthropics/skills", "version": "a1b2c3d",
     "status": "up_to_date", "installed_at": "2024-06-01T10:00:00Z"},
    {"name": "commit", "source": "anthropics/skills", "version": "x9y8z7w",
     "status": "update_available", "installed_at": "2024-05-15T08:30:00Z"},
    {"name": "old-helper", "source": "anthropics/skills", "version": "d4e5f6g",
     "status": "stale", "installed_at": "2024-03-10T09:00:00Z"},
    {"name": "local-skill", "source": "", "version": "",
     "status": "local", "installed_at": "2024-04-20T12:00:00Z"}
  ]
}
```

`"status": "error"` の Skill には、原因がわかる場合に `message` フィールドが含まれます（例: `"message": "local source not found: /path/to/skill"`）。

## ステータス表示

| アイコン | 意味 |
|------|---------|
| `✓` | 最新 |
| `⬇` | 更新あり（トラッキング対象のリポジトリ: 遅れているコミット数、Skill: 新しいバージョン） |
| `⚠` | Stale — アップストリームでサブディレクトリが削除またはリネームされた |
| `!` | 未コミットの変更がある |
| `•` | ローカルソース（比較対象のインストールメタデータがない） |

:::info Stale な Skill
Skill のサブディレクトリがアップストリームでリネームまたは削除された場合、`check` はそれを **stale** として報告します。stale な Skill を掃除するには `update --prune` を使用してください。
:::

:::tip モノレポでの挙動
サブディレクトリからインストールされた Skill については、`check` はそのディレクトリ自体が変更された場合にのみ「update available」を報告します。リポジトリの無関係な部分に新しいコミットがあっても報告されません。
:::

## Project mode

```bash
skillshare check -p                    # すべての Project の Skill を確認
skillshare check -p my-skill           # 特定の Project の Skill を確認
skillshare check -p --group frontend   # Project のグループを確認
skillshare check -p --json             # Project 向けの JSON 出力
```

## Agent 対応

`skillshare check agents` は確認対象を Agent のみに絞り、Agents source ディレクトリ内の `.md` ファイルについて drift と更新状況を報告します。

```bash
skillshare check agents              # すべての Agent を確認
skillshare check agents --json       # Agent 向けの JSON 出力
skillshare check agents -p           # Project の Agent を確認
```

`agents` 引数を指定しない場合、`check` は Skill のみを対象にします（デフォルトの挙動）。背景については [Agents](/docs/understand/agents) を参照してください。

## 関連項目

- [update](/docs/reference/commands/update) — 更新を適用
- [list](/docs/reference/commands/list) — インストール済みの Skill を表示
- [status](/docs/reference/commands/status) — sync 状態を表示
- [Agents](/docs/understand/agents) — Agent の概念
