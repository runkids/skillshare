---
sidebar_position: 8
---

# 宣言的な Skill マニフェスト

skill コレクションをコードとして定義する — 単一のマニフェストファイルからセットアップをインストール・共有・再現します。

:::tip これが重要になるのはどんなとき？
マシン間で再現可能な skill セットアップ、1 つのコマンドによるチームのオンボーディング、オープンソースプロジェクトのブートストラップを望むときに、declarative manifest を使用してください。
:::

## Skill マニフェストとは？

Skill マニフェストは、skill コレクションの**ポータブルな宣言**です。skill を 1 つずつ手動でインストールする代わりに、マニフェストファイルに列挙し、`skillshare install` を実行してすべてを揃えます。

マニフェストの場所は mode によって異なります。

| Mode | マニフェストの場所 | コミット可能か |
|------|------------------|-------------|
| **Project** | `.skillshare/config.yaml`（`skills:` セクション） | Yes — チームと共有するためコミットする |
| **Global** | `~/.config/skillshare/skills/.metadata.json` | No — 個人のマシンの状態 |

### Project Mode のマニフェスト

Project mode では、`skills:` は `targets:` と並んで `config.yaml` の中にあります。

```yaml
# .skillshare/config.yaml
targets:
  - claude
  - cursor

skills:
  - name: react-best-practices
    source: anthropics/skills/skills/react-best-practices
    group: frontend
  - name: _team-skills
    source: my-org/shared-skills
    tracked: true
  - name: commit
    source: anthropics/skills/skills/commit
```

このファイルは git にコミットされます — チームメンバーはリポジトリを clone し、`skillshare install -p` を実行して、リストされたすべての skill をインストールします。

マニフェストは*何を*インストールするかを記録します。各 skill が解決された正確なコミットは、その隣の `.skillshare/skills.lock.json` に記録されます。このファイルは自動的に書き込まれ、こちらもコミットしておく必要があります。両方のファイルがあれば、upstream が先に進んだ後でも、`skillshare install -p` はチームメンバー全員に同じコミットを提供します。[ロックファイル](./project-skills.md#lockfile)を参照してください。

### Global Mode のマニフェスト

Global mode では、skill のレコードは `.metadata.json`（一元化されたメタデータストア）に保存されます。このファイルにはランタイムのトラッキングデータ（ハッシュ、タイムスタンプ）も含まれており、自動的に管理されます。

## 仕組み

### マニフェストからのインストール

**引数なし**で `skillshare install` を実行すると、マニフェストを読み込んでリストされたすべての skill をインストールします。

```bash
# Global mode — installs all skills from ~/.config/skillshare/skills/.metadata.json
skillshare install

# Project mode — installs all skills from .skillshare/config.yaml skills: section
skillshare install -p

# Preview without installing
skillshare install --dry-run
```

すでに存在する skill は自動的にスキップされます。Project mode では、インストール済みのコミットがロックファイルと異なる skill は、代わりに固定されたコミットへ移動します。

### 自動的な整合性維持

マニフェストは実際の skill コレクションと同期し続けます。

- **`skillshare install <source>`** — インストールされた skill を自動的にマニフェストに追加します
- **`skillshare uninstall <name>...`** — マニフェストからそのエントリを自動的に削除します

Project mode では `config.yaml` と `skills.lock.json` が更新されます。Global mode では `.metadata.json` が更新されます。マニフェストを手動で編集する必要は（可能ではありますが）ありません。

このマシンにまだインストールされていない Skill のエントリ（クローン直後など）は残るため、`skillshare install` で引き続きインストールできます。エントリがマニフェストから消えるのは、その Skill をアンインストールしたとき、またはインストール済みのフォルダを削除したときだけです。

## Skill エントリのフィールド

`skills:` リストの各エントリには、次のフィールドがあります。

| フィールド | 必須 | 説明 |
|-------|----------|-------------|
| `name` | Yes | skill 名（source 内のディレクトリ名） |
| `source` | Yes | インストール元（GitHub の省略形、HTTPS URL、SSH URL） |
| `tracked` | No | tracked リポジトリの場合は `true`（`.git` を保持） |
| `group` | No | サブディレクトリのパス（例: `frontend` や `frontend/vue`）。インストール時の `--into` に対応します。 |

## ユースケース

### 個人でのセットアップ

複数のマシン間で個人の skill コレクションを維持します。

```bash
# On machine A — skills are already installed and tracked in registry
skillshare push   # backup config + registry to git

# On machine B — fresh machine
skillshare pull   # restore config + registry from git
skillshare install  # install all skills from manifest
skillshare sync   # distribute to all targets
```

### チームのオンボーディング

新しいチームメンバーは、1 つのコマンドで同じ AI コンテキストを得られます。

```bash
# .skillshare/config.yaml skills: section is committed to the repo
git clone <project-repo>
cd <project-repo>
skillshare install -p   # installs all declared skills
skillshare sync -p      # links to project targets
```

### オープンソースのブートストラップ

プロジェクトのメンテナーは、推奨する skill を `config.yaml` に宣言します。

```yaml
# .skillshare/config.yaml
targets:
  - claude
  - cursor

skills:
  - name: react-best-practices
    source: anthropics/skills/skills/react-best-practices
    group: frontend
  - name: commit
    source: anthropics/skills/skills/commit
```

:::info Group フィールドと `--into`
`--into` を指定してインストールすると、group は自動的に記録されます。

```bash
skillshare install anthropics/skills/skills/pdf --into frontend -p
# config.yaml will contain: name: pdf, group: frontend
```

`skillshare install -p`（引数なし）を実行すると、マニフェストから同じディレクトリ構造が再現されます。
:::

コントリビューターは clone して `skillshare install -p` を実行するだけで、プロジェクト固有の AI コンテキストをすぐに得られます。

## ワークフローのまとめ

```
Project mode:
1. Install skills normally      →  config.yaml skills: auto-updates
2. Commit config.yaml and skills.lock.json via git  →  same skills, same commits for the team
3. Run `skillshare install -p`  →  reproduce on clone
4. Run `skillshare sync`        →  distribute to all targets

Global mode:
1. Install skills normally      →  .metadata.json auto-updates
2. Push/pull config via git     →  portable across machines
3. Run `skillshare install`     →  reproduce on new machine
4. Run `skillshare sync`        →  distribute to all targets
```

## Extras の設定

skill に加えて、`config.yaml` は **extras**（skill ではないリソース: rules、commands、prompts）を宣言でき、これらは別のディレクトリに sync されます。Extras は `config.yaml` の `extras:` セクション（global、project の両方）で設定します。

```yaml
extras:
  - name: rules
    targets:
      - path: ~/.claude/rules
      - path: ~/.cursor/rules
        mode: copy
```

詳細は [sync extras](/docs/reference/commands/sync#sync-extras) を参照してください。

## 関連項目

- [Install command](/docs/reference/commands/install) — 引数あり・なしの `skillshare install`
- [Push/Pull](/docs/reference/commands/push) — git 経由での config のバックアップと復元
- [Project Skills](./project-skills.md) — プロジェクトレベルのマニフェスト
