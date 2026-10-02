---
sidebar_position: 5
---

# 移行

他の Skill 管理方法から skillshare への移行方法です。

## 手動管理からの移行

これまで AI CLI 間で Skill を手動でコピーしていた場合:

### ステップ 1: skillshare を初期化する

```bash
skillshare init
```

### ステップ 2: 既存の Skill を収集する

```bash
# 各 AI CLI から収集
skillshare collect claude
skillshare collect pi
skillshare collect codex

# または一度にすべてから収集
skillshare collect --all
```

### ステップ 3: 重複を処理する

同じ Skill が複数の場所に存在する場合、`collect` が警告します。どちらを残すか選択してください。

### ステップ 4: Sync する

```bash
skillshare sync
```

これで、すべての Target が単一の Source にシンボリックリンクされます。

---

## 他のインストールツールからの移行

`npx install-skill` などを使っていた場合:

### ステップ 1: skillshare を初期化する

```bash
skillshare init
```

### ステップ 2: 既存の Skill をバックアップする

```bash
skillshare backup
```

### ステップ 3: 収集または再インストールする

**オプション A: 既存のものを収集する**（現在のバージョンを維持）
```bash
skillshare collect --all
```

**オプション B: Source から再インストールする**（最新バージョンを取得）
```bash
# メタデータを確認
cat ~/.config/skillshare/skills/.metadata.json

# 再インストール
skillshare install anthropics/skills/skills/pdf
```

### ステップ 4: Sync する

```bash
skillshare sync
```

---

## `npx skills` からの移行

[npx skills CLI](https://github.com/vercel-labs/skills)（`npx skills add ...`）で Skill をインストールしていた場合、skillshare は同じディレクトリを引き継げます。[両方のツールを併用](/docs/troubleshooting/faq#using-universal-alongside-npx-skills)し、skillshare で管理したい Skill だけを移すこともできます。

`npx skills` のファイルの保存場所:

| 項目 | Global | Project |
|------|--------|---------|
| Skill ファイル | `~/.agents/skills/<name>/`（実ディレクトリ） | `.agents/skills/<name>/` |
| Agent ディレクトリ | 上記ファイルへのシンボリックリンク（例: `~/.claude/skills/<name>`） | 同じ |
| ロックファイル | `~/.agents/.skill-lock.json`（または `$XDG_STATE_HOME/skills/.skill-lock.json`） | `skills-lock.json` |

`--copy` でインストールした Skill は、シンボリックリンクではなく各 Agent ディレクトリ内の実ディレクトリになります。

### ステップ 1: skillshare を初期化する

```bash
skillshare init
```

### ステップ 2: 既存の Skill をバックアップする

```bash
skillshare backup
```

### ステップ 3: Skill を収集する

`collect` は実ディレクトリをコピーし、シンボリックリンクはスキップします。そのため、Agent ディレクトリ内のシンボリックリンクが二重に収集されることはありません。

```bash
skillshare collect universal --dry-run   # プレビュー
skillshare collect universal             # ~/.agents/skills
```

`--copy` を使っていた場合や、他の Agent ディレクトリにも Skill がある場合は、代わりに `skillshare collect --all` を実行し、[手動管理からの移行](#手動管理からの移行)の説明に従って重複を処理してください。

収集した Skill は単なるローカルコピーです。どのリポジトリから来たかを記録していないため、`skillshare update` では更新できません。

### ステップ 4: 更新を続けたい Skill を再インストールする（任意）

ロックファイルには各 Skill の取得元が記録されています。`source` はリポジトリ、`skillPath` はその中の場所です。

```bash
cat ~/.agents/.skill-lock.json

# "source": "anthropics/skills", "skillPath": "skills/pdf/SKILL.md"
skillshare install anthropics/skills/skills/pdf
```

ステップ 3 で Skill がすでに Source に存在する場合は `--force` を使ってください。

### ステップ 5: Sync する

```bash
skillshare sync
```

merge mode では、Sync は Target（`~/.agents/skills`）内の同名の実ディレクトリを保持し、保持されたローカル Skill として報告します。`~/.claude/skills` など他の Agent ディレクトリにある `npx skills` のシンボリックリンクは、skillshare の Source を指すように張り替えられます。実ディレクトリもシンボリックリンクに置き換えるには、ステップ 3 で収集した後に次を実行します:

```bash
skillshare sync --force
```

`sync --force` は何かを置き換える前に Target をバックアップします。これ以降、それらの Skill は skillshare だけで管理してください。`npx skills` は独自のロックファイルでインストールを追跡しているため、それらの Skill に対して `npx skills update` や `npx skills remove` を実行しないでください。

`npx skills` を使っている Project では、移動するディレクトリを `.agents/skills/` として[コミット済みの Project Skill からの移行](#コミット済みの-project-skill-からの移行)に従ってください。

---

## Git サブモジュールからの移行

git サブモジュールを使っていた場合:

### ステップ 1: サブモジュールの内容をエクスポートする

```bash
# 既存の Skill リポジトリ内で
git submodule foreach 'cp -r $toplevel/$sm_path ~/temp-skills/$name'
```

### ステップ 2: skillshare を初期化する

```bash
skillshare init
```

### ステップ 3: Skill をインポートする

```bash
# Source にコピー
cp -r ~/temp-skills/* ~/.config/skillshare/skills/

# または Tracked repos としてインストール
skillshare install github.com/org/skill-repo --track
```

### ステップ 4: Sync する

```bash
skillshare sync
```

---

## コミット済みの Project Skill からの移行

すでにリポジトリに `.claude/skills/`、`.cursor/skills/` などのディレクトリに Skill がコミットされている場合:

### ステップ 1: Project mode を初期化する

```bash
cd my-project
skillshare init -p
```

### ステップ 2: Skill を `.skillshare/skills/` に移動する

```bash
# 既存の Skill を skillshare の Source にコピー
cp -r .claude/skills/my-skill .skillshare/skills/
cp -r .claude/skills/api-guide .skillshare/skills/

# 元のものを削除（Sync がシンボリックリンクとして再作成する）
rm -rf .claude/skills/my-skill .claude/skills/api-guide
```

### ステップ 3: Sync する

```bash
skillshare sync
```

これで `.claude/skills/my-skill` は `.skillshare/skills/my-skill` へのシンボリックリンクになり、他のすべての Target（Pi、Windsurf など）にも同じ Skill が自動的に反映されます。

### ステップ 4: 移行をコミットする

```bash
git add .skillshare/ .claude/skills/ .cursor/skills/
git commit -m "Migrate project skills to skillshare"
```

:::tip マルチツールのメリット
以前: Skill は1つの AI CLI でしか動作しなかった。以後: 同じ Skill が設定済みのすべての Target で自動的に利用可能になる。
:::

---

## チーム独自の仕組みからの移行

チームに独自の Skill 共有の仕組みがある場合:

### ステップ 1: 現在のアプローチを特定する

- Skill はどこに保存されているか？
- どのように共有されているか？
- どのように更新されているか？

### ステップ 2: 移行パスを選ぶ

**オプション A: Global mode** — 各マシンのすべてのプロジェクトで Skill を利用可能にする。

```bash
# チーム Skill リポジトリを作成
cp -r /current/team/skills ~/new-team-skills
cd ~/new-team-skills && git init && git add . && git commit -m "Migrate to skillshare"
git push origin main

# チームメンバーは Global にインストール
skillshare install github.com/org/team-skills --track && skillshare sync
```

**オプション B: Project mode** — 特定のリポジトリにスコープされた Skill を git 経由で共有する。

```bash
cd my-project
skillshare init -p

# チーム Skill を Project の Source に移動
cp -r /current/team/skills/* .skillshare/skills/

# Sync してコミット
skillshare sync
git add .skillshare/
git commit -m "Add team skills via skillshare"
```

新しいチームメンバーは次のコマンドですべてを取得できます。
```bash
git clone github.com/org/my-project
cd my-project
skillshare install -p && skillshare sync
```

**オプション C: 両方** — 組織全体の標準は Global、プロジェクト固有の Skill は各リポジトリで。

```bash
# 組織の標準（Global）
skillshare install github.com/org/standards --track && skillshare sync

# プロジェクト固有の Skill（Project mode）
cd my-project
skillshare init -p
skillshare install github.com/org/project-skills -p && skillshare sync
```

:::tip どちらを選ぶべきか？
- **Global**: コーディング標準、セキュリティ監査など、すべてのプロジェクトに必要なもの
- **Project**: API 規約、ドメインルール、デプロイガイドなど、1つのリポジトリに固有のもの
- **両方**: 多くのチームは成長するにつれてここに落ち着く
:::

---

## Global から Project への移行

特定のプロジェクトに属する Skill が Global mode にある場合:

### ステップ 1: Project mode を初期化する

```bash
cd my-project
skillshare init -p
```

### ステップ 2: Global の Source から Skill をコピーする

```bash
# 特定の Skill をコピー
cp -r ~/.config/skillshare/skills/api-guide .skillshare/skills/
cp -r ~/.config/skillshare/skills/deploy-rules .skillshare/skills/
```

### ステップ 3: Global から削除する（任意）

```bash
skillshare uninstall api-guide
skillshare uninstall deploy-rules
skillshare sync   # Global のシンボリックリンクをクリーンアップ
```

### ステップ 4: Sync してコミットする

```bash
skillshare sync   # Project mode を自動検出
git add .skillshare/
git commit -m "Move project-specific skills to project mode"
```

これで Skill はこのリポジトリにスコープされ、git 経由でチームと共有されます — もう Global の設定を圧迫することはありません。

---

## 履歴の保持

git の履歴を維持したい場合:

### 個人の Skill の場合

```bash
# 既存のリポジトリを skillshare の場所に clone
git clone your-existing-repo ~/.config/skillshare/skills

# 既存の Source で skillshare を初期化
skillshare init --source ~/.config/skillshare/skills
```

### チームのリポジトリの場合

```bash
# --track を使って .git を保持
skillshare install github.com/team/skills --track
```

---

## ロールバック

移行がうまくいかなかった場合:

### バックアップから復元する

```bash
skillshare restore claude
skillshare restore cursor
```

### 最初からやり直す

```bash
rm ~/.config/skillshare/config.yaml
skillshare init
```

---

## チェックリスト

移行前:

- [ ] 現在の Skill の場所をすべてリストアップする
- [ ] 重複を特定する
- [ ] カスタム設定をメモする
- [ ] バックアップを作成する

移行後:

- [ ] `skillshare list` にすべての Skill が表示されることを確認する
- [ ] 各 AI CLI で Skill をテストする
- [ ] git リモートをセットアップする（必要であれば）
- [ ] 新しいワークフローをチームと共有する

---

## 関連項目

- [既存 Skill からの移行](/docs/getting-started/from-existing-skills) — 素早い移行パス
- [collect](/docs/reference/commands/collect) — Target からの収集
- [比較](/docs/understand/philosophy/comparison) — アプローチの比較
