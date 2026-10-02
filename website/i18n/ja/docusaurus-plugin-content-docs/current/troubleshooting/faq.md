---
sidebar_position: 4
---

# FAQ

skillshare に関するよくある質問です。

## 一般

### これは単なる `ln -s` ではないですか？

本質的にはそうです。ですが skillshare は以下を処理します。
- 複数 Target の検出
- バックアップ/復元
- merge モード（Skill ごとのシンボリックリンク）
- クロスデバイス Sync
- 壊れたシンボリックリンクの復旧

これらをあなたが自分でやる必要はありません。

### Target ディレクトリ内で Skill を変更するとどうなりますか？

Target はシンボリックリンクなので、変更は直接 Source に対して行われます。すべての Target が
即座にその変更を見ます。

### CLI 固有の Skill を維持するには？

`merge` モード（デフォルト）を使ってください。Target 内のローカルの Skill は上書きされたり
Sync されたりしません。

```bash
skillshare target claude --mode merge
skillshare sync
```

その後 `~/.claude/skills/` に直接 Skill を作成してください — それらには触れられません。

### dotfiles マネージャー（stow/chezmoi/yadm）を使っていますが、skillshare は私のシンボリックリンクを壊しますか？

いいえ。skillshare は Source と Target の両方のディレクトリで外部のシンボリックリンクを検出し、
それらを保持します。sync、update、uninstall、list、diff、install を含むすべてのコマンドは、
シンボリックリンク自体を削除することなく、シンボリックリンクを解決してその実体となる
ディレクトリに対して操作します。詳細は
[Dotfiles Manager との互換性](/docs/reference/commands/sync#dotfiles-manager-compatibility)
を参照してください。

dotfiles 経由で `config.yaml` をバージョン管理している場合は、パスを絶対パスではなく
`~/...` のまま保つために `preserve_tilde_on_save: true` を有効にすることを検討してください —
[Configuration](/docs/reference/targets/configuration#preserve_tilde_on_save) を参照してください。

---

## インストール

### カスタムまたは珍しいツールに Skill を Sync できますか？

できます。ツールの Skill ディレクトリを指定して `skillshare target add <name> <path>` を
使ってください。

```bash
mkdir -p ~/.myapp/skills
skillshare target add myapp ~/.myapp/skills
skillshare sync
```

### プライベートな git リポジトリで skillshare を使えますか？

使えます。SSH URL を使用してください。

```bash
skillshare init --remote git@github.com:you/private-skills.git
```

---

## Sync

### なぜ install/update のたびに `sync` を実行する必要があるのですか？

Sync は意図的に別のステップになっています。`install`、`update`、`uninstall` のような操作は
**Source** ディレクトリのみを変更します — `sync` がその変更をすべての Target に伝播します。

これにより次のことが可能になります。
- **変更をまとめる** — 5つの Skill をインストールしてから、5回ではなく1回だけ sync する
- **先にプレビューする** — 適用する前に `sync --dry-run` を実行する
- **主導権を保つ** — Target をいつ更新するかを自分で決める

**注記:** `pull` は自動的に sync する唯一のコマンドです。その意図が「すべてを最新の状態に
する」ことだからです。

設計思想の全体像は
[Sync が別のステップである理由](/docs/understand/source-and-targets#why-sync-is-a-separate-step)
を参照してください。

### 複数のマシン間でどうやって Sync しますか？

git ベースのクロスマシン Sync を使ってください。

```bash
# マシンA: 変更を push する
skillshare push -m "Add new skill"

# マシンB: pull して sync する
skillshare pull
```

完全なセットアップは [クロスマシン Sync](/docs/how-to/sharing/cross-machine-sync) を
参照してください。

### 誤ってシンボリックリンク経由で Skill を削除してしまった場合は？

git が初期化されていれば（推奨）、以下で復旧できます。

```bash
cd ~/.config/skillshare/skills
git checkout -- deleted-skill/
```

またはバックアップから復元してください。
```bash
skillshare restore claude
```

### 誤って Skill をアンインストールしてしまった場合は？

アンインストールされた Skill はゴミ箱に移動され、7日間保持されます。以下で復元してください。

```bash
skillshare trash list                  # ゴミ箱の中身を確認する
skillshare trash restore my-skill      # Source に復元する
skillshare sync                        # Target に Sync し直す
```

Skill がリモート Source からインストールされたものであれば、再インストールもできます。

```bash
skillshare install github.com/user/repo/my-skill
skillshare sync
```

プロジェクトモードでは、ゴミ箱はプロジェクトディレクトリ内の `.skillshare/trash/` にあります。
trash コマンドには `-p` フラグを使用してください。

現在のゴミ箱の状態（アイテム数、サイズ、経過日数）を確認するには `skillshare doctor` を
実行してください。

### backup と trash の違いは何ですか？

| | backup | trash |
|---|---|---|
| **保護対象** | Target ディレクトリ（Sync のスナップショット） | Source の Skill（アンインストール） |
| **場所** | `~/.local/share/skillshare/backups/` | `~/.local/share/skillshare/trash/` |
| **トリガー** | `sync`、`target remove` | `uninstall` |
| **復元方法** | `skillshare restore <target>` | `skillshare trash restore <name>` |
| **自動クリーンアップ** | 手動（`backup --cleanup`） | 7日 |

これらは補完関係にあります — backup は Sync の変更から Target を保護し、trash は誤削除から
Source の Skill を保護します。

### 特定の Skill を特定の CLI にだけ Sync できますか？

できます。例えば Skill A は Claude にのみ、Skill B は Antigravity と Codex に、Skill C は
すべてに Sync する場合:

**オプション1: SKILL.md の `targets` フィールド**（Skill 作者が設定）

```yaml
# skills/skill-a/SKILL.md
---
name: skill-a
targets: [claude]
---
```

```yaml
# skills/skill-b/SKILL.md
---
name: skill-b
targets: [antigravity, codex]
---
```

```yaml
# skills/skill-c/SKILL.md — targets フィールドなし = すべてに Sync される
---
name: skill-c
---
```

**オプション2: Config の `include`/`exclude` フィルター**（利用者が設定）

```yaml
# ~/.config/skillshare/config.yaml
targets:
  claude:
    path: ~/.claude/skills
    include: [skill-a, skill-c]
  codex:
    path: ~/.codex/skills
    include: [skill-b, skill-c]
```

両方のアプローチを組み合わせることもできます — 先に Config フィルターが適用され、その後
Skill 単位の `targets` フィールドが適用されます。
[Skill フォーマット — `targets`](/docs/understand/skill-format#targets) と
[Configuration — フィルター](/docs/reference/targets/configuration#skill-level-targets) を
参照してください。

---

## Targets

### npx skills と universal を併用する {#using-universal-alongside-npx-skills}

`universal` Target は `~/.agents/skills` を指しており、これは
[npx skills CLI](https://github.com/vercel-labs/skills) が使用するのと同じディレクトリです。
両方のツールは、いくつかの注意点はありますが、このディレクトリを同時に管理できます。

**うまくいくこと:**
- merge モード（デフォルト）では、skillshare は `~/.agents/skills/` に **シンボリックリンク**
  を作成し、npx skills は **実際のディレクトリ** を作成します。Skill 名が衝突しない限り両者は
  共存します。
- skillshare の prune ロジックは、自分が管理しているエントリのみを削除します — npx skills が
  インストールしたファイルを削除することはありません。
- Agent CLI（Claude Code、Pi など）はディレクトリを直接読み取るため、両方のツールからの
  Skill が見えます。

**注意すべきこと:**
- **名前の衝突** — 両方のツールが同じ名前の Skill をインストールした場合、最後に sync/install
  した方が勝ちます。同じ Skill を両方のツールでインストールすることは避けてください。
- **copy モードはより積極的** — copy モード（`skillshare target universal --mode copy`）では、
  skillshare は sync のたびに管理下のディレクトリを上書きします。npx skills が sync の間に
  同名の Skill を変更した場合、skillshare がそれを置き換えてしまいます。merge モード
  （デフォルト）はシンボリックリンクを作成するだけなので、共存にはより安全です。
- **`npx skills list` に skillshare の Skill は表示されない** — npx skills CLI はディレクトリを
  スキャンするのではなく、ロックファイル（`~/.agents/.skill-lock.json`）でインストールを追跡
  します。skillshare が Sync した Skill は `npx skills list -g` には表示されませんが、Agent CLI
  からは**見えます**。
- **他の Agent 固有の Target も引き続き有用** — `universal` と `claude` は異なるパス
  （`~/.agents/skills` と `~/.claude/skills`）を指します。両方を選択しても安全で、冗長では
  ありません。

**推奨ワークフロー:**
```bash
# skillshare を主な Skill マネージャーとして使う
skillshare install github.com/user/skills --track
skillshare sync

# Sync する必要のない、その場限りのコミュニティインストールにのみ npx skills を使う
npx skills add someone/skill -g
```

:::tip
npx skills との最も安全な共存のために、universal Target を **merge モード**（デフォルト）の
ままにしてください。npx skills をまったく使わない場合を除き、copy モードへの切り替えは
避けてください。
:::

### プロジェクトの Target として `claude-code`（や `gemini-cli` など）を使っていましたが、まだ有効ですか？

はい。`claude-code`、`gemini-cli`、`github-copilot` のような古いプロジェクト Target 名は、
引き続きエイリアス経由で解決されます。例えば `gemini-cli` は `gemini` に解決されます。
`.skillshare/config.yaml` を正式名称を使うように更新することをお勧めします。

```yaml
# 変更前
targets:
  - claude-code

# 変更後
targets:
  - claude
```

### `target remove` はどう動作しますか？安全ですか？

はい、安全です。

1. **バックアップ** — Target のバックアップを作成する
2. **モードを検出する** — symlink モードか merge モードかを確認する
3. **リンク解除** — skillshare が管理するすべてのシンボリックリンクを削除し、Source の内容を
   実ファイルとしてコピーし戻す。merge モードでは、Source ディレクトリを指すシンボリックリンク
   のみが削除され、ローカルの（非シンボリックリンクの）Skill は保持される
4. **Config を更新する** — config.yaml から Target を削除する

これが、`rm -rf ~/.claude/skills` は Source ファイルを削除してしまうのに対し、
`skillshare target remove` が安全である理由です。

### なぜ Target に対する `rm -rf` は危険なのですか？

symlink モードでは、Target ディレクトリ全体が Source へのシンボリックリンクです。それを削除すると
Source が削除されます。

merge モードでは、各 Skill がシンボリックリンクです。シンボリックリンク経由で Skill を削除すると
Source ファイルが削除されます。

**常に以下を使ってください:**
```bash
skillshare target remove <name>   # 安全
skillshare uninstall <skill>      # 安全
```

---

## Tracked リポジトリ

### Tracked リポジトリは通常の Skill とどう違いますか？

| 観点 | 通常の Skill | Tracked リポジトリ |
|--------|---------------|--------------|
| Source | Source にコピーされる | `.git` 付きで clone される |
| 更新 | `install --update` | `update <name>`（git pull） |
| プレフィックス | なし | `_` プレフィックス |
| ネストされた Skill | フラット化される | `__` でフラット化される |

### なぜアンダースコアのプレフィックスがあるのですか？

`_` プレフィックスは Tracked リポジトリを識別します。
- 通常の Skill と区別しやすくする
- 名前の衝突を防ぐ
- 一覧表示で分かりやすく表示する

---

## Skills

### SKILL.md のフォーマットは何ですか？

```markdown
---
name: skill-name
description: Brief description
---

# Skill Name

Instructions for the AI...
```

完全な詳細は [Skill フォーマット](/docs/understand/skill-format) を参照してください。

### 「unknown target」警告は何を意味しますか？

`skillshare check` または `skillshare doctor` を実行すると、次のような表示が出ることがあります。

```
! Skill targets: my-skill: unknown target "*"
```

これは、Skill の `SKILL.md` frontmatter の `targets` フィールドに認識できない名前 — よくあるのは
`"*"`（ワイルドカード）— が設定されていることを意味します。skillshare は glob パターンではなく、
**正確な Target 名**（例: `claude`、`cursor`、`codex`）を想定しています。

**Skill をすべての Target に Sync したい場合**、`targets` フィールドを完全に省略してください。

```yaml
---
name: my-skill
description: Works everywhere
# targets フィールドなし = すべての Target に Sync される
---
```

**この警告がサードパーティの Skill から出ている場合**、その Skill の作者が対応していない構文を
使用しています。以下のいずれかができます。
1. **警告を無視する** — Skill は引き続きインストールされ、特定の Target に自動フィルタリング
   されないだけです
2. **フォークして修正する** — Skill の `SKILL.md` から `targets` フィールドを削除または修正する

完全な仕様は [Skill フォーマット — `targets`](/docs/understand/skill-format#targets) を
参照してください。

### Skill は複数のファイルを持てますか？

持てます。Skill ディレクトリには以下を含められます。
- `SKILL.md`（必須）
- 追加の任意のファイル（例、テンプレートなど）

SKILL.md の指示内でそれらを参照してください。

---

## パフォーマンス

### Sync が遅いようです

skills ディレクトリ内に大きなファイルがないか確認してください。ignore パターンを追加してください。

```yaml
# ~/.config/skillshare/config.yaml
ignore:
  - "**/.DS_Store"
  - "**/.git/**"
  - "**/node_modules/**"
  - "**/*.log"
```

### Skill はいくつまで持てますか？

明確な上限はありません。パフォーマンスは以下に依存します。
- Skill の数
- Skill ファイルのサイズ
- Target の数

数千の小さな Skill でも問題なく動作します。

---

## バックアップ

### バックアップはどこに保存されますか？

```
~/.local/share/skillshare/backups/<timestamp>/
```

### バックアップはどのくらいの期間保持されますか？

デフォルトでは無期限です。以下でクリーンアップしてください。
```bash
skillshare backup --cleanup
```

---

## Agents

### agents と skills の違いは何ですか？

Skill は `SKILL.md` ファイル（および任意でヘルパー、例、テンプレート）を含む**ディレクトリ**です。
Agent は frontmatter を持つ**単一の `.md` ファイル**で、ネストされた構造はありません。どちらも
install、sync、audit、check、backup、trash に対応しています。

完全な比較と Agent ファイルフォーマットは [Agents](/docs/understand/agents) を参照してください。

### どの Target が Agent に対応していますか？

標準で対応しているのは `claude`、`cursor`、`augment`、`opencode`（および `universal` エイリアス）
です。他の Target は Agent の Sync 中に `target(s) skipped for agents (no agents path)` 警告
とともに黙ってスキップされます。`config.yaml` を編集して手動で Agent パスを追加できます。

```yaml
targets:
  myapp:
    path: ~/myapp/skills
    agents:
      path: ~/myapp/agents
```

### 削除せずに単一の Agent を無効化するには？

`disable` コマンドを使うか（または `.agentignore` を直接編集してください）。

```bash
skillshare disable my-agent --kind agent     # .agentignore にエントリを追加する
skillshare enable my-agent --kind agent      # そのエントリを削除する
```

`.agentignore` は Agent Source のルート（グローバルでは
`~/.config/skillshare/agents/.agentignore`、プロジェクトモードでは
`.skillshare/agents/.agentignore`）にあり、
[gitignore 構文](https://git-scm.com/docs/gitignore) を使用します。ローカル限定の上書きのために
`.agentignore.local` オーバーレイにも対応しています。

### プロジェクトモードで Agent をバックアップできますか？

できます — ただし Agent の**みです**。`backup` は Skill に対してはプロジェクトモードで許可されて
いませんが、Agent のフローは明示的な例外です。

```bash
skillshare backup -p agents     # プロジェクトの Agent Target をバックアップする
skillshare backup -p --all      # 上と同じ。プロジェクトモードでは --all は Agent に絞り込まれる
```

`agents` フィルターを忘れると `backup is not supported in project mode (except for agents)`
と表示されます。同じルールが `restore` にも適用されます。Agent のバックアップは通常の Skill の
バックアップの隣にある `<target>-agents/` 配下に保存されます。

---

## セキュリティ

### サードパーティの Skill は信頼できますか？

Skill は AI Agent への指示であり、悪意のある Skill は AI にシークレットを持ち出させたり
破壊的なコマンドを実行させたりする可能性があります。skillshare は組み込みのセキュリティ
スキャナーでこれを軽減します。

- **インストール時の自動スキャン** — `skillshare install` の実行中にすべての Skill がスキャン
  される
- **CRITICAL の検出結果はブロックされる** — プロンプトインジェクション、データ持ち出し、
  認証情報へのアクセスはデフォルトでブロックされる
- **手動スキャン** — いつでも `skillshare audit` を実行して、インストール済みのすべての Skill を
  スキャンできる

検出パターンの完全なリストは [audit コマンド](/docs/reference/commands/audit) を参照してください。

### audit がインストールをブロックした場合は？

Skill が CRITICAL の検出結果をトリガーすると、インストールがブロックされます。2つの選択肢が
あります。

1. **検出結果を確認する** — 誤検知（例: ドキュメントの例）かどうかを確認する
2. **強制インストールする** — Source を信頼する場合は `--force` を使ってチェックをバイパスする

```bash
skillshare install suspicious-skill --force
```

### audit はすべてを検出しますか？

完璧なスキャナーは存在しません。`skillshare audit` は、プロンプトインジェクション、シークレットを
伴う `curl`/`wget`、認証情報ファイルへのアクセス、難読化されたペイロードなど、よくあるパターンを
検出します。信頼できない Source からの Skill は常に手動で確認してください。

---

## ヘルプを得る

### バグはどこに報告しますか？

[GitHub Issues](https://github.com/runkids/skillshare/issues)

### 質問はどこでできますか？

[GitHub Discussions](https://github.com/runkids/skillshare/discussions)

---

## 関連項目

- [よくあるエラー](./common-errors.md) — エラーの解決方法
- [Windows](./windows.md) — Windows 固有の FAQ
- [トラブルシューティングワークフロー](./troubleshooting-workflow.md) — 段階的なデバッグ
