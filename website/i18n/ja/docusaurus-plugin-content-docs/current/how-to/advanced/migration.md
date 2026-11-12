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

## `npx skills` からの移行 {#from-npx-skills}

既存 skills は `npx skills` で管理し続けても、選んだ skills を Skillshare に引き継いでも構いません。両方とも複数 agent とグローバル／プロジェクトのインストールに対応しています。移行の理由は Skillshare の広いリソース管理が必要だからであり、もう一方がインストール専用だからではありません。[比較](/docs/understand/philosophy/comparison)を参照してください。

**npx lock の自動インポートや管理権の自動移行はありません。** Collect はローカルの skill ファイルを保存しますが、取得元の追跡情報は引き継ぎません。リポジトリから再インストールすると Skillshare の追跡情報が作られますが、既存インストールとは内容が異なる場合があります。

以下のパスと lock フィールドは **skills v1.8.1** に基づきます：[グローバル lock のソース](https://github.com/vercel-labs/skills/blob/e781cb4d07433dfe1d19e924ed45cc61738c55ec/src/skill-lock.ts)、[プロジェクト lock のソース](https://github.com/vercel-labs/skills/blob/e781cb4d07433dfe1d19e924ed45cc61738c55ec/src/local-lock.ts)、[CLI リファレンス](https://github.com/vercel-labs/skills/blob/e781cb4d07433dfe1d19e924ed45cc61738c55ec/docs/commands.md)。

| 項目 | Global | Project |
|------|--------|---------|
| 共通 skill の場所 | `~/.agents/skills/<name>/` | `.agents/skills/<name>/` |
| Agent ディレクトリ | 通常は共通 skills へのリンク。`--copy` は独立したコピーを作成 | 同じ |
| Lock ファイル | 設定されていれば `$XDG_STATE_HOME/skills/.skill-lock.json`、それ以外は `~/.agents/.skill-lock.json` | `skills-lock.json` |

`experimental_sync` が作る skills は、実ディレクトリではなく npm 依存関係へのリンクの場合があります。収集前に実際のパスを確認してください。

### ステップ 1: 範囲を決め、元の構成を保存する

ステップ 2–5 は明示的に**グローバルモード**（`-g`）を使います。プロジェクトのインストールには下のプロジェクト向け説明を使ってください。

初期化や同期の前に：

- 各ツールが管理する skills を決め、移行中は更新・削除・自動同期ジョブを停止します。
- 元の skill ディレクトリ、実際の内容、symlink 構成を別途スナップショットとして保存します。上書きする可能性がある既存の Skillshare source も含めてください。
- 該当する npx lock を別途保存します。npm プロジェクトでは `package.json` とパッケージマネージャーの lock も保存します。これらは取得元と依存関係の記録であり、ローカル編集のバックアップではありません。

引き継ぎを検証するまでスナップショットを保持してください。古い lock を削除したり、後片付けとして `npx skills remove` を実行したりしないでください。

### ステップ 2: コピーせずに初期化し、バックアップを確認する

Skillshare が未初期化の場合、**stdin と stdout の両方が端末につながった対話環境**で以下を実行してください。パイプや自動化では実行しないでください。これらのフラグは初回同期を無効にせず、非対話の初期化では targets が選択されると自動同期します。

```bash
skillshare init -g --no-copy --no-skill --no-git
```

管理する targets を選び、`~/.agents/skills` に対応する `universal` を含めてください。この手順では merge mode を使用します。初回同期を尋ねられたら断ってください。設定済みの場合はリセットせず、source、[targets](/docs/reference/commands/target)、モードを確認します。

```bash
skillshare backup -g --dry-run
skillshare backup -g
skillshare backup -g --list
```

出力の失敗を確認し、実際の skill ファイルが含まれていることを確かめます。Target バックアップは **symlink をスキップ**し、リンク先の内容や target ディレクトリ外の npx lock は保存しません。ステップ 1 のスナップショットを補うもので、代わりにはなりません。

### ステップ 3: ローカルコピーか取得元の追跡を選ぶ

**現在のファイルを保持する：**実ディレクトリを持つ target からプレビューして収集します。

```bash
skillshare collect universal -g --dry-run
skillshare collect universal -g
```

これは target 内の対象となる skills すべてを収集し、選んだ 1 つだけを収集するものではありません。特定 agent のディレクトリにコピーがある場合は、その設定済み target 名に置き換えます。`collect --all -g` は全 targets から収集する場合だけ使ってください。一部だけ引き継ぐ場合は、選んだ実 skill ディレクトリだけを Skillshare source にコピーします。

Collect は `node_modules` へのリンクを含む skill symlink をスキップします。npm 管理を続けるか、リンク先の実 skill ディレクトリを明示的にローカルスナップショットとしてコピーしてください。そのコピーはパッケージ更新に追従しなくなります。強制しない限り、source に既存の名前もスキップします。スキップ／失敗の結果を確認し、重複内容は自分で比較してください。プレビューは lock を解釈する競合レポートではありません。`collect --force` で無条件に勝者を決めないでください。

収集した skills にはリポジトリの追跡情報が取り込まれないため、`skillshare update` で元の取得元から更新できません。

**取得元からの更新を続ける：**インストール前に保存した lock の各項目を確認します。`sourceType` は Git、npm、ローカルなどを区別し、`sourceUrl`、`ref`、`skillPath` に追加情報がある場合があります。すべての項目にリポジトリや `skillPath` があるわけではありません。

たとえば GitHub の項目 `source: "anthropics/skills"`、`skillPath: "skills/pdf/SKILL.md"` は、そのファイルを含む skill ディレクトリに対応します。

```bash
skillshare install anthropics/skills/skills/pdf -g --dry-run
skillshare install anthropics/skills/skills/pdf -g
```

Lock に Git ref が記録されている場合は、上の両コマンドに `--branch <recorded-ref>` を追加してください（[install](/docs/reference/commands/install) を参照）。指定しない場合、この例はリモートのデフォルトブランチを使います。これは新規インストールであり、npx の内容 hash を再現するものではありません。保存済みファイルと比較し、ローカル編集を統合してください。収集した skill がすでに宛先にある場合は、いったん止めて解決します。強制インストールは編集を上書きし audit のブロックも上書きする可能性があるため、安易に使わないでください。npm やローカルの項目は別途ソースを判断し、単純に `owner/repo` に変換しないでください。

### ステップ 4: プレビューし、明示的に管理を引き継ぐ

```bash
skillshare sync -g --dry-run
```

`universal` だけでなく、**設定済みの全 targets** を確認します。Merge mode は同名のローカルディレクトリを保持しますが、同名の agent symlink は force なしでも Skillshare source に張り替えられる場合があります。引き継ぐ予定の skills だけに影響することを確認してから適用します。

```bash
skillshare sync -g
```

保持された実ディレクトリもリンクに置き換える場合は、source が希望のバージョンであり、元のスナップショットが完全であることを先に確認してください。

```bash
skillshare sync -g --force --dry-run
# Apply only after reviewing every replacement:
skillshare sync -g --force
```

Force は設定済み targets 全体に作用し、skill 単位の移行スイッチではありません。まだ npx skills が管理するリソースがプレビューに含まれていたら停止してください。管理対象を分離し、[併用の説明](/docs/troubleshooting/faq#using-universal-alongside-npx-skills)を参照してください。

### ステップ 5: 検証し、管理者を 1 つにする

結果のファイル／リンクを確認し、各 AI ツールで期待した skills が読み込まれることを確かめます。`skillshare status -g` を実行し、保持されたローカル skills や警告を確認してください。Sync の成功だけでは全 skills の移行を証明できません。

Skillshare は古い npx lock の項目を削除しません。引き継ぎ後は移行済み skills に `npx skills update`、`remove`、`experimental_install`、`experimental_sync` を使わないでください。一部移行の場合は、それらを含む無指定の一括コマンドを避けてください。

元に戻す場合は両ツールを停止し、保存した元ファイル、リンク構成、lock をまとめて復元します。Skillshare の target バックアップはローカルファイルの復旧に役立ちますが、元の npx symlink や lock を単独では再現できません。

### プロジェクトのインストールと npm 同期

変更前に、未コミット・無視対象のローカル編集も含めてプロジェクトファイルと lock を保存します。`skillshare init -p` で初期化し、プロジェクト targets を設定してから、上記のグローバルコマンドではなく `collect`、`install`、`sync` に `-p` を付けて使います。追跡情報と symlink の制約は同じです。プロジェクトモードでは skill-target のバックアップを作成しないため、検証済みスナップショットか、必要なファイルを実際に含むバージョン管理を使ってください。

`package.json` の scripts や CI が `experimental_sync` を呼び出す場合、重複する skills を引き継ぐ前にジョブと skill 宣言を調整します。そうしないと次の依存関係インストールで再リンクされる可能性があります。npm の skills を npx skills に任せ続けても構いません。リンクを `.skillshare/skills/` にコピーするだけで独立した管理になったと思わないでください。

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
