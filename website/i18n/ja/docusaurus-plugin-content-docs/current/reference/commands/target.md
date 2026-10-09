---
sidebar_position: 1
---

# target

同期 Target（AI CLI の Skill ディレクトリ）を管理します。

```bash
skillshare target add <name> <path>    # Target を追加
skillshare target remove <name>        # Target を削除
skillshare target list                 # すべての Target を一覧表示
skillshare target <name>               # Target の情報を表示
skillshare target <name> --mode merge  # sync モードを変更
skillshare target <name> --target-naming standard  # 命名方式を変更
skillshare target <name> --skills=false    # Skill の同期を停止
```

## 使うタイミング

- 新しい AI CLI ツールをインストールした後に新しい Target を追加する
- 使わなくなった Target を削除する
- Target の sync モード（merge、copy、または symlink）を変更する
- Target の命名方式（flat、standard、または prefixed）を変更する
- 1 つのグローバルモードを強制するのではなく、Target ごとに互換性を調整する
- 選択的な Skill 同期のための include/exclude フィルタを設定する
- 別の Target のフォルダーをすでに読んでいるツールへの Skill 同期を停止し、その agents、MCP サーバー、instructions は引き続き管理する

## サブコマンド

### target add

Skill 同期のための新しい Target を追加します。

```bash
skillshare target add windsurf ~/.windsurf/skills
```

このコマンドは以下を検証します。
- パスが存在するか、親ディレクトリが存在すること
- パスが Skill ディレクトリらしいこと
- Target 名が一意であること

`--no-skills` を付けると、Skill を同期しない Target として追加します。agents、MCP サーバー、instructions は引き続き管理され、skills フォルダーが存在する必要もありません。

```bash
skillshare target add gemini ~/.gemini/skills --no-skills
# Added target: gemini -> ~/.gemini/skills (skills off)
```

[Skill のオン／オフ](#skills-off) を参照してください。

#### Agent の別のアカウント {#another-account}

Agent の 2 つ目のアカウントを専用の config ディレクトリで動かしている場合（例: Claude Code を `CLAUDE_CONFIG_DIR=~/.claude-work` で、Codex を `CODEX_HOME` で、Pi を `PI_CODING_AGENT_DIR` で起動している場合）、そのディレクトリを Target として追加します。Skillshare はそこから skills と agents のパスを導き出します。

```bash
skillshare target add claude-work --agent claude --config-dir ~/.claude-work
# Added target: claude-work -> ~/.claude-work/skills
```

アカウントの数だけ、それぞれ別の名前で追加できます。この名前は [MCP Target](./mcp.md#accounts) としても使えるため、1 回の sync ですべてのアカウントの Skill、Agent、MCP サーバーに反映されます。

アカウントが互換 CLI（Pi なら omo など）を使う場合は、`--cli` を付けるとその [plugin コマンド](./plugin.md#accounts)がその CLI で実行されます。

```bash
skillshare target add omo --agent pi --config-dir ~/.omo/agent --cli omo
```

`--agent` は `claude`（`CLAUDE_CONFIG_DIR`）、`codex`（`CODEX_HOME`）、`pi` と `omp`（どちらも `PI_CODING_AGENT_DIR`）を受け付けます。Codex、Pi、OMP のアカウントは Skill を `<config_dir>/skills` に sync します。agents ディレクトリを持つのは Claude だけです。OMP のアカウントは Skill、instructions、files、MCP、ネイティブのコード hooks に対応しますが、plugin コマンドには対応しません。ディレクトリは絶対パスか `~` で始まる必要があり、Agent のデフォルトのディレクトリであってはならず、2 つの Target で共有することもできません。

この種の Target の削除が MCP を理由に失敗することはありません。`mcp.targets` やサーバーの `targets` にまだその名前が残っていても、`skillshare target remove` は Target を削除し、そちらからも名前を取り除くよう警告します。

### target remove

Target を削除し、その Skill を通常のディレクトリに復元します。

```bash
skillshare target remove cursor           # 単一の Target を削除
skillshare target remove --all            # すべての Target を削除
skillshare target remove cursor --dry-run # プレビュー
```

**実行される内容:**
1. Target のバックアップを作成
2. sync モードを検出:
   - **Symlink モード:** ディレクトリのシンボリックリンクを削除し、source の内容を実ディレクトリとしてコピーし直す
   - **Merge モード:** source を指すシンボリックリンクのみを（パスのプレフィックスで）削除し、各 Skill を実ファイルとしてコピーし直す。ローカル（非シンボリックリンク）の Skill は保持される
   - **Copy モード:** `.skillshare-manifest.json` を削除する。管理対象のコピーとローカルの Skill は通常のディレクトリとして保持される
3. Target を config から削除

同じ skills フォルダーに書き込む別の Target がある場合（たとえば `codex` と `universal` はどちらも `~/.agents/skills` を使う）、手順 2 はスキップされます。skills はその Target 用にリンクされたまま残り、削除した Target だけが config から外れます。

[Skill がオフ](#skills-off) の Target には何も同期されていないため、手順 2 は同様にスキップされ、そのフォルダーはそのまま残ります。

### target list

設定済みのすべての Target を一覧表示します。

```bash
skillshare target list                 # インタラクティブ TUI（TTY 上のデフォルト）
skillshare target list --no-tui        # プレーンテキスト出力
skillshare target list --json          # CI／スクリプト向けの JSON 出力
```

#### インタラクティブ TUI

TTY 上では、`target list` はインタラクティブな画面を開きます。左側にターゲット、右側に選択したターゲットのパス、モード、フィルタが表示されます。ここからターゲットの sync モード、命名、include/exclude フィルタの変更や、ターゲットの削除（`target remove` と同様にバックアップしてからリンク解除）ができます。キーは画面下部に表示されます。

変更は即座に config に保存されます。反映するには `skillshare sync` を実行してください。

`--no-tui` を使うと TUI をスキップし、代わりにプレーンテキストを出力します。

```
claude
  Skills    ~/.claude/skills  merge · flat · merged · 43 shared
  Agents    ~/.claude/agents  merge · 2/2 linked

cursor
  Skills    ~/.cursor/skills  merge · flat · merged · 43 shared, 1 local
  Agents    ~/.cursor/agents  merge · 2/2 linked

codex
  Skills    ~/.openai-codex/skills  symlink · flat · linked

3 targets
```

#### JSON 出力

```bash
skillshare target list --json
```

```json
{
  "targets": [
    {
      "name": "claude",
      "path": "~/.claude/skills",
      "mode": "merge",
      "targetNaming": "flat",
      "include": [],
      "exclude": [],
      "skillsEnabled": true
    },
    {
      "name": "cursor",
      "path": "~/.cursor/skills",
      "mode": "merge",
      "targetNaming": "standard",
      "include": [],
      "exclude": [],
      "skillsEnabled": true
    }
  ]
}
```

`warning` は、sync がその Target を拒否する場合（例：copy 以外の mode での `prefixed` naming）にのみ追加され、修正方法も示されます。

### target info / settings

Target の詳細を表示、または設定を変更します。

```bash
# 情報を表示
skillshare target claude

# モードを変更
skillshare target claude --mode symlink
skillshare target claude --mode merge

# 命名方式を変更
skillshare target claude --target-naming standard
skillshare target claude --target-naming flat

skillshare sync  # 変更を適用
```

## Sync モード

| モード | 動作 |
|------|------|
| `merge` | 各 Skill を個別にシンボリックリンク。ローカルの Skill を保持する。**デフォルト。** |
| `copy` | 各 Skill を実ファイルとしてコピーする。シンボリックリンクを辿れない AI CLI 向け。 |
| `symlink` | ディレクトリ全体を 1 つのシンボリックリンクにする。どこでも完全な複製になる。 |

`target --mode` は主要な互換性制御の窓口です。グローバルなデフォルトはシンプルに保ち、必要な箇所だけ上書きしてください。

## Target の命名方式

| 命名方式 | 動作 |
|--------|--------|
| `flat` | ネストした Skill を `__` 区切りでフラット化する（例: `frontend__dev`）。**デフォルト。** |
| `standard` | SKILL.md の `name` フィールドをそのまま使用する（例: `dev`）。[Agent Skills spec](https://agentskills.io/specification) に準拠する。 |
| `prefixed` | copy mode 専用。`standard` と同じだが、tracked repo 内の Skill はフォルダー名とコピー先の `name:` の両方が `<repo>-<name>` になる（例: `mattpocock-skills-prototype`）。 |

`target --target-naming` は Target 内で Skill ディレクトリがどのように命名されるかを制御します。`standard` および `prefixed` モードでは、無効または衝突する名前を持つ Skill は警告付きでスキップされます。`flat` と `standard` は symlink モードでは無視されます。`--target-naming prefixed` は、Target が copy mode で Skill を sync していない限り拒否され、Target が `prefixed` を使っている間は `--mode` で copy mode から外れることも拒否されます。両方を一度に切り替えるには、まとめて指定します: `skillshare target cursor --mode copy --target-naming prefixed`。`--mode`、`--agent-mode`、`--target-naming` はこのように組み合わせられ、まとめて検証して一度だけ保存します。Target がすでに持つ値は「変更なし」と表示されます。`--skills` や include/exclude フラグとは組み合わせられないため、別のコマンドで実行してください。[Target の命名規則](/docs/understand/sync-modes#target-naming) を参照してください。

```bash
# Target を copy モードに設定する（Cursor、Copilot CLI などに向けて）
skillshare target cursor --mode copy
skillshare sync  # 変更を適用
```

### 混在戦略の例

```bash
# ほとんどの Target ではデフォルトの merge の動作を維持する
skillshare target claude --mode merge

# 1 つの Target には互換性優先の設定を行う
skillshare target cursor --mode copy

# 別の Target には完全なミラーリングを行う
skillshare target codex --mode symlink

skillshare sync
```

## Target フィルタ（include/exclude）{#target-filters-includeexclude}

CLI から、Skill と agent の両方について Target ごとの include/exclude フィルタを管理できます。

```bash
# Skill
skillshare target claude --add-include "team-*"
skillshare target claude --add-exclude "_legacy*"
skillshare target claude --remove-include "team-*"
skillshare target claude --remove-exclude "_legacy*"

# Agent
skillshare target claude --add-agent-include "team-*"
skillshare target claude --add-agent-exclude "draft-*"
skillshare target claude --remove-agent-include "team-*"
skillshare target claude --remove-agent-exclude "draft-*"
```

フィルタを変更した後は、`skillshare sync` を実行して適用してください。

フィルタは **merge モードと copy モード**で機能します。パターンは Go の `filepath.Match` の構文（`*`, `?`, `[...]`）を使用します。symlink モードではフィルタは無視されます。

Agent フィルタは、組み込みの Target 定義、または config の明示的な `agents.path` オーバーライドのいずれかによって agents パスを持つ Target でのみ利用できます。

パターンのチートシートとシナリオについては、[Configuration](/docs/reference/targets/configuration#include--exclude-target-filters) を参照してください。

:::tip
Target フィルタは 3 つのフィルタリング階層の 1 つです。`.skillignore` や SKILL.md の `targets` とどのように連携するかは [Filtering Reference](/docs/reference/filtering) を参照してください。
:::

## Skill のオン／オフ {#skills-off}

一部のツールは、自身のフォルダーに加えて別の Target のフォルダーからも Skill を読み込みます。たとえば Pi は `~/.pi/agent/skills` に加えて、`universal` Target のフォルダーである `~/.agents/skills` も読み込みます。両方に Skill を同期すると、Pi は各 Skill を 2 回見つけます。Pi は最初に見つけたものを残してもう一方について警告し、両方を一覧表示するツールもあります。その Target の Skill をオフにすると、skillshare は agents、MCP サーバー、instructions の管理を続けつつ、skills フォルダーには手を触れません。

```bash
skillshare target pi --skills=false --dry-run   # プレビュー
skillshare target pi --skills=false
```

```
✓ Removed   2 links  alpha, beta
  Kept      1 local skill  my-notes

✓ Skills off for pi
  Agents, MCP servers and instructions are still managed
```

Skill をオフにすると、config に `skills.enabled: false` が保存され、その後フォルダーが整理されます。

- **Merge モード:** source を指すリンクを削除する。自分の Skill は残る。
- **Symlink モード:** フォルダーから source へのリンクを削除する。リンク先の内容は削除しない。
- **Copy モード:** コピーは編集済みかもしれない実フォルダーなので保持し、別枠で一覧表示する。ツールはこれらを引き続き読み込むため、同じ Skill を別のフォルダーからも読む場合は、コピーを自分で削除する:

  ```
  ! Kept      2 copied skills  alpha, beta

  ✓ Skills off for pi
    The tool still loads these copies; delete them if it reads the same skills elsewhere
    Agents, MCP servers and instructions are still managed
  ```

- **共有フォルダー:** 有効な Target が同じフォルダーに書き込んでいる場合は、何も削除しない。

以降、`sync`、`diff`、`status`、`doctor` はその Target の Skill をスキップし、`status` と `sync` では `skills off` と表示されます。`--skills=true` で Skill を再びオンにすると、次の `skillshare sync` で再び同期されます。

`--skills` は 1 つのコマンド内で include/exclude フラグと組み合わせられません。別々に実行してください。project モード（`-p`）でも同じように動作します。

Web ダッシュボードでは、Target の Skills タブにある **Skills の同期を停止** を使います。何かを削除する前に、削除されるものと残るものを一覧表示し、他のツールが同じフォルダーを読んでいる場合は警告します。

## オプション

### target add

| フラグ | 説明 |
|------|-------------|
| `--agent <agent>` | パスの代わりに、この Agent の[別のアカウント](#another-account)を追加する。`--config-dir` と併用 |
| `--config-dir <dir>` | そのアカウントが使う config ディレクトリ |
| `--cli <executable>` | そのアカウントの plugin コマンドを、Agent 本体ではなくこの互換 CLI で実行する。`PATH` 上の名前か絶対パス |
| `--no-skills` | [Skill をオフ](#skills-off)にして Target を追加 |

### target remove

| フラグ | 説明 |
|------|-------------|
| `--all, -a` | すべての Target を削除 |
| `--dry-run, -n` | 変更を加えずにプレビュー |

### target list

| フラグ | 説明 |
|------|-------------|
| `--json` | JSON として出力 |
| `--no-tui` | インタラクティブ TUI を無効化し、プレーンテキスト出力を使用 |

### target info / settings

| フラグ | 説明 |
|------|-------------|
| `--mode, -m <mode>` | sync モードを設定（merge、copy、または symlink） |
| `--agent-mode <mode>` | agent の sync モードを設定（merge、copy、または symlink） |
| `--target-naming <naming>` | Target の命名方式を設定（flat、standard、または prefixed。prefixed は copy mode が必要） |
| `--skills <true\|false>` | Skill の同期を[オンまたはオフ](#skills-off)にする。`--skills=false` の形でも指定可能 |
| `--dry-run, -n` | `--skills=false` と併用し、削除される内容をプレビュー |
| `--add-include <pattern>` | include フィルタパターンを追加 |
| `--add-exclude <pattern>` | exclude フィルタパターンを追加 |
| `--remove-include <pattern>` | include フィルタパターンを削除 |
| `--remove-exclude <pattern>` | exclude フィルタパターンを削除 |
| `--add-agent-include <pattern>` | agent の include フィルタパターンを追加 |
| `--add-agent-exclude <pattern>` | agent の exclude フィルタパターンを追加 |
| `--remove-agent-include <pattern>` | agent の include フィルタパターンを削除 |
| `--remove-agent-exclude <pattern>` | agent の exclude フィルタパターンを削除 |

## サポートされる AI CLI

skillshare は `init` の際に以下を自動検出します。

| CLI | デフォルトパス |
|-----|-------------|
| Claude Code | `~/.claude/skills` |
| Cursor | `~/.cursor/skills` |
| OpenCode | `~/.opencode/skills` |
| Windsurf | `~/.windsurf/skills` |
| Codex | `~/.openai-codex/skills` |
| Antigravity（アプリ） | `~/.gemini/config/skills` |
| Antigravity CLI | `~/.gemini/antigravity-cli/skills` |
| Gemini CLI | `~/.gemini/skills` |
| Amp | `~/.amp/skills` |
| ... その他 45 種類以上 | [supported targets](/docs/reference/targets/supported-targets) を参照 |

## 例

```bash
# カスタム Target を追加
skillshare target add my-tool ~/my-tool/skills

# Target の状態を確認
skillshare target claude

# copy モードに切り替える（symlink を読めない AI CLI 向け）
skillshare target cursor --mode copy
skillshare sync

# symlink モードに切り替える
skillshare target claude --mode symlink
skillshare sync

# agent の sync モードを設定する
skillshare target claude --agent-mode copy
skillshare sync

# Skill フィルタを追加／削除する
skillshare target claude --add-include "team-*"
skillshare target claude --add-exclude "_legacy*"
skillshare target claude --remove-include "team-*"
skillshare sync

# agent フィルタを追加／削除する
skillshare target claude --add-agent-include "team-*"
skillshare target claude --add-agent-exclude "draft-*"
skillshare sync

# Target を削除する（Skill を復元する）
skillshare target remove cursor
```

## Project モード

現在の project の Target を管理します。

```bash
skillshare target add windsurf -p                                # 既知の Target を追加
skillshare target add custom ./tools/ai/skills -p                # カスタムパスを追加
skillshare target remove cursor -p                                # Target を削除
skillshare target list -p                                         # project の Target を一覧表示
skillshare target claude -p                                  # Target の情報を表示
skillshare target claude --add-include "team-*" -p          # フィルタを追加
skillshare target claude --add-agent-include "team-*" -p    # agent フィルタを追加
```

### 違いについて

| | グローバル | Project（`-p`） |
|---|---|---|
| Config | `~/.config/skillshare/config.yaml` | `.skillshare/config.yaml` |
| パス | 絶対パス（例: `~/.claude/skills`） | 相対パスまたは絶対パス（例: `.claude/skills`） |
| Sync モード | merge、copy、または symlink | merge、copy、または symlink（デフォルト merge） |
| モードの変更 | `--mode` フラグ | `--mode` フラグ |

### Project の Target 一覧の例

```
claude
  Skills    .claude/skills  merge · flat · merged · 3 shared

cursor
  Skills    .cursor/skills  merge · flat · merged · 3 shared

custom-tool
  Skills    ./tools/ai/skills  merge · flat · merged · 3 shared

3 targets
```

project モードでの Target は以下をサポートします。
- **既知の Target 名**（例: `claude`, `cursor`） — project ローカルのパスに解決される
- **カスタムパス** — project ルートからの相対パス、または `~` 展開を伴う絶対パス

## 関連項目

- [sync](/docs/reference/commands/sync) — Skill を Target に同期
- [status](/docs/reference/commands/status) — Target の状態を表示
- [Targets](/docs/reference/targets) — Target 管理ガイド
- [Project Skills](/docs/understand/project-skills) — project モードの概念
