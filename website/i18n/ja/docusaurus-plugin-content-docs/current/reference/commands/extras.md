---
sidebar_position: 2
---

# extras

Skill と一緒に sync される、Skill 以外のリソース（rules、commands、prompts など）を管理します。

## 概要

Extras は skillshare が管理する追加のリソースタイプです — いわば「Skill 以外のコンテンツ用の Skill」と考えてください。よくある用途としては、AI の rules、エディタの commands、prompt テンプレートをツール間で sync することが挙げられます。

各 Extras は以下を持ちます。
- **名前**（例: `rules`、`prompts`、`commands`）
- **Source ディレクトリ** — `extras_source` または Extras ごとの `source` で設定可能。デフォルトは `~/.config/skillshare/extras/<name>/`（グローバル）または `.skillshare/extras/<name>/`（Project）
- 同期先となる 1 つ以上の **Target**

ダッシュボードの **Extras → Folders & files** には、各 Extras とその Target、モードが並びます。

![Extras › Folders & files：rules と commands をそれぞれの Target に同期](/img/extras-folders.png)

## コマンド

### `extras memory` {#extras-memory}

`memory` extra の共有 Markdown ノートを管理します。任意のテキストエディターを使えます。
[スクリーンショット付きガイド](../../how-to/daily-tasks/sharing-memory)で作成から Agent への接続まで確認できます。

| サブコマンド | 動作 |
|---|---|
| `init` | Target のない memory extra を登録し、不足する `INDEX.md` と `LEARNED.md` を作成。既存のファイルと設定は保持 |
| `list` | ノート一覧。`--search <text>` でサブフォルダーを含むパスと内容を大文字小文字を区別せず検索 |
| `show <note.md>` | ノートを読む。`--json` は `version` hash を含む |
| `write <note.md> --from <file\|->` | ファイルまたは stdin から書き込む。新規作成では `--version` を省略し、更新には最後に読んだ version が必要 |
| `delete <note.md> --version <hash>` | バックアップ後に指定 version を削除。古い version や未指定の version は拒否 |
| `instructions` | 実際の source フォルダーを指す読み込み指示を出力 |

各サブコマンドは `--json`、`-g` / `--global`、`-p` / `--project`、`--help` に対応します。
Scope は未指定なら自動検出。既定の global パスは `~/.config/skillshare/extras/memory/`、
Project は `.skillshare/extras/memory/` です。既存の extras source 設定が適用されます。

ノートは相対 `.md` パスの UTF-8 ファイルで、最大 1 MiB。隠しファイル、隠しフォルダー、内部のシンボリックリンクは除外します。大きすぎるファイルや非 UTF-8 ファイルは未対応として一覧に残り、他の有効なノートは使えます。`wiki/architecture.md` は不足するフォルダーを自動作成します。Dashboard はツリー、**Preview** / **Source**、**Copy path**、**Edit**、**Delete note**、**History** を提供します。**Move or rename** は新しい相対 `.md` パスを指定し、不足するフォルダーを作成します。内容と権限を保持し、既存の移動先や古い version は拒否します。移動前に元のパスをバックアップします。Markdown リンクは手動で修正してください。Agent のガイダンスが参照する source 直下の `INDEX.md` はその場所に維持してください。

保存は最後に読んだ version を確認します。競合時は下書きを保持し、最新の保存内容を比較用に表示します。**Save my draft** は確認後に更新された version を使い、保存済み内容をバックアップして置換します。削除も確認、version 検証、バックアップを行います。**History** と削除後の復元リンクは、ノートの絶対パスで絞り込んだ **Backup Files** を開きます。CLI では `backup files show <absolute-path>` と `backup files restore <absolute-path> <id>` を使います。

**New note** の **Link from INDEX.md** はインデックスが読み取り可能な場合に表示され、既定でオンです。末尾にリンクを追加し、version を確認してバックアップします。失敗しても新しいノートは残ります。**Add to INDEX** で未登録のノートを追加できます。リンク切れは警告を表示しますが、自動削除はしません。CLI の書き込みはリンクを追加しません。

**Connect to agents** でツールを選び、**Review changes** → **Apply changes** を実行します。既存の指示ファイルまたは共有 source に scope/hash マーカー付きブロックを追加・更新し、他の内容と割り当ては保持します。変更、他の読み取りツール、既知の文字数制限を確認できます。既存ファイルをバックアップし、古いプランを拒否します。変更されていない古いブロックはレビュー後に更新でき、手動変更済みや不正なブロックは保持します。未同期または読めない指示ファイルはスキップします。

**Configured** は現在の指示が読み取り経路にある状態で、読み取り済みを意味しません。**Copy verification prompt** を新しいセッションで使い、`INDEX.md` と関連ノートを読み、完全なパスとユーザーが加えた一時的な検証値を報告させます。実際の読み取りイベントを手動で確認してください。読み取り telemetry は保証しません。

**Copy guidance** は手動貼り付けの代替手段で、**Open AGENTS.md** から編集できます。Project 内の source は指示ファイルの場所に関係なく **project root** からの相対パス、Project 外や global の source は絶対パスです。移動後は再生成してください。CLI の `instructions` も同じ scope/hash ブロックを出力します。CLI フラグは変わりません。Native automatic memory、自動学習、Obsidian 連携は有効になりません。

### `extras init`

新しい Extras リソースタイプを作成します。

```bash
# インタラクティブウィザード
skillshare extras init

# CLI フラグ
skillshare extras init <name> --target <path> [--target <path2>] [--mode <mode>]

# 単一ファイルの Extras
skillshare extras init <name> --file <filename> [--as <filename>] --target <path> [--source <dir>] [--mode <mode>]
```

ウィザードは名前の後に **What do you want to sync?** と尋ねます。**Folder** または **Single file** を選びます。

**オプション:**

| フラグ | 説明 |
|------|-------------|
| `--target <path>` | Target ディレクトリのパス（複数指定可） |
| `--file <filename>` | Source ディレクトリ内のこのファイルだけを sync し、[単一ファイルの Extras](#single-file-extras) にする。`/` や `\` を含まない単純なファイル名 |
| `--as <filename>` | すべての Target で書き出すファイル名（デフォルト: `--file` の名前）。`--file` が必要 |
| `--mode <mode>` | sync モード: `merge`（デフォルト）、`copy`、または `symlink`。`import` は `--file` 指定時のみ |
| `--flatten` | サブディレクトリ内のファイルを Target のルート直下に sync する（`symlink` モードや `--file` とは併用不可） |
| `--source <path>` | この Extras 用のカスタム Source ディレクトリ（`extras_source` とデフォルトを上書き。Project モードではプロジェクトルートからの相対パス） |
| `--force` | すでに存在する Extras を上書き |
| `--no-tui` | インタラクティブウィザードをスキップし、CLI フラグのみを使用 |
| `--project, -p` | Project 設定（`.skillshare/`）内に作成 |
| `--global, -g` | グローバル設定内に作成 |

:::note
`--source` はグローバルモードでのみサポートされています。Project mode では常に `.skillshare/extras/<name>/` が Source ディレクトリとして使われます。
:::

**例:**

```bash
# rules を Claude と Cursor に sync
skillshare extras init rules --target ~/.claude/rules --target ~/.cursor/rules

# カスタムの Source ディレクトリを使う
skillshare extras init rules --target ~/.claude/rules --source ~/company-shared/rules

# 既存の Extras を新しい Target で上書きする
skillshare extras init rules --target ~/.cursor/rules --force

# copy モードの Project スコープ Extras
skillshare extras init prompts --target .claude/prompts --mode copy -p

# agents をフラットに sync（Claude Code のようなツールはフラットなファイルしか検出しない）
skillshare extras init agents --target ~/.claude/agents --flatten

# 1 つのファイルを sync し、Target では名前を変える
skillshare extras init pi-prompt --file system.md --as APPEND_SYSTEM.md \
  --source ~/dotfiles/prompts --target ~/.pi/agent
```

`extras init` は設定を書き込むだけです。Source ファイルの作成や sync は行いません。単一ファイルの Extras では、Source と Target のファイルのフルパスを表示します。

```
  Source    ~/dotfiles/prompts/system.md
  Target    ~/.pi/agent/APPEND_SYSTEM.md · merge

✓ Created extra pi-prompt (single file)

Next
  skillshare sync extras  sync it
```

Source ファイルがまだ存在しない場合、Source の行の末尾に `(not found)` が付き、最後の行は `Create the source file, then run 'skillshare sync extras'.` になります。

### `extras list`

設定済みのすべての Extras と、その sync 状態を一覧表示します。デフォルトでインタラクティブ TUI を起動します。

```bash
skillshare extras list [--json] [--no-tui] [-p|-g]
```

**オプション:**

| フラグ | 説明 |
|------|-------------|
| `--json` | JSON 出力（`source_type`: `per-extra` / `extras_source` / `default`、および設定されている場合は Target ごとの `extension` フィールドを含む） |
| `--no-tui` | インタラクティブ TUI を無効化し、プレーンテキスト出力を使用 |
| `--project, -p` | Project モードの Extras（`.skillshare/`）を使用 |
| `--global, -g` | グローバルの Extras（`~/.config/skillshare/`）を使用 |

#### インタラクティブ TUI

TTY 上では、`extras list` はインタラクティブな画面を開きます。左側に extras、右側に選択した extra のターゲットとファイルが表示されます。ここから extras の作成、削除、sync、collect、およびターゲットのモードや flatten 設定の変更ができます。キーは画面下部に表示されます。

`skillshare tui off` で TUI を恒久的に無効化できます。

#### プレーンテキスト出力

TUI が無効な場合（`--no-tui`、`skillshare tui off`、またはパイプされた出力）:

```
$ skillshare extras list --no-tui
rules  ~/.config/skillshare/extras/rules · 2 files
✓ ~/.claude/rules  merge
✓ ~/.cursor/rules  copy

codex-agents  ~/.config/skillshare/agents · 3 files
✓ ~/.codex/agents  extension: codex-agents

2 extras
```

[単一ファイルの Extras](#single-file-extras) では、Source と各 Target にディレクトリではなくファイルのフルパスが表示されます。

sync 済みの行にはアイコン、パス、モードのみが表示されます。未 sync の行にはステータス語（`drift`、`modified`、`not synced`、`no source`）が追記されます。変換拡張子（extension）を持つ Target は、sync モードの代わりに `extension: <name>` と表示されます（実際のモードは常に `copy` です）。

### `extras source`

グローバルな `extras_source` ディレクトリを表示または設定します。これは Extras の Source ファイルが格納されるデフォルトの親ディレクトリです。

```bash
skillshare extras source            # 現在の値を表示
skillshare extras source <path>     # 新しい値を設定
```

引数なしの場合、現在の `extras_source` パスを表示します（自動検出された場合は `(default)` が付きます）。パスを引数に渡すと、グローバル設定の `extras_source` を更新します。

:::note
このコマンドはグローバル専用です。Project mode では常に `.skillshare/extras/` が使われ、`extras_source` はサポートされません。
:::

**例:**

```bash
# 現在の extras_source を表示
skillshare extras source

# 共有ディレクトリに設定
skillshare extras source ~/company-shared/extras
```

### 既存の Extras を操作する

`extras <name>` で Target の sync モードや flatten 設定を変更し、Target を追加・削除できます。モード、flatten、追加した Target の変更は `skillshare sync extras` で適用します。`--remove-target --prune` は管理対象ファイルの復元や削除もすぐに行います。

```bash
skillshare extras <name> --mode <mode> [--target <path>] [-p|-g]
skillshare extras <name> --flatten | --no-flatten [--target <path>]
skillshare extras <name> --add-target <path> [--as <filename>] [--mode <mode>] [--flatten] [-p|-g]
skillshare extras <name> --remove-target <path> [--prune] [-p|-g]
skillshare extras <name> --help
```

**オプション:**

| フラグ | 説明 |
|------|-------------|
| `--mode <mode>` | 新しい sync モード: `merge`、`copy`、または `symlink`。`import` は[単一ファイルの Extras](#single-file-extras) でのみ使用可 |
| `--flatten` | flatten を有効化（サブディレクトリのファイルを Target ルートに sync） |
| `--no-flatten` | flatten を無効化 |
| `--add-target <path>` | Extras に新しい Target を追加 |
| `--as <filename>` | `--add-target` の Target ファイル名（単一ファイルの Extras のみ。既定値は `file`） |
| `--remove-target <path>` | Extras から Target を削除（デフォルトでは設定のみ） |
| `--prune` | `--remove-target` と併用: その Target 配下の skillshare 管理ファイルも削除。単一ファイルの Extras では、代わりに Target のファイルを元に戻す |
| `--target <path>` | Target ディレクトリのパス（複数 Target を持つ Extras で `--mode` を使う場合は必須。省略時、`--flatten`/`--no-flatten` はすべての Target に適用される） |
| `--project, -p` | Project モードの Extras（`.skillshare/`）を使用 |
| `--global, -g` | グローバルの Extras（`~/.config/skillshare/`）を使用 |

**例:**

```bash
# rules のモードを変更（単一 Target — 自動解決）
skillshare extras rules --mode copy

# Target を明示的に指定する（複数 Target の Extras では必須）
skillshare extras rules --mode copy --target ~/.claude/rules

# すべての Target で flatten を一括有効化/無効化
skillshare extras agents --flatten
skillshare extras agents --no-flatten

# 既存の Extras に新しい Target を追加する（その後 sync）
skillshare extras rules --add-target ~/.cursor/rules
skillshare extras commands --add-target ~/.config/opencode/commands --mode copy
skillshare extras personal --add-target ~/.claude --as CLAUDE.md --mode import

# Target を削除する（sync 済みファイルはそのまま残す）
skillshare extras rules --remove-target ~/.cursor/rules

# Target を削除し、その sync 済みファイルも削除する
skillshare extras rules --remove-target ~/.cursor/rules --prune
```

TUI（`e` キー）と Web UI（各 Target のモードのドロップダウンと flatten チェックボックス）からも操作できます。

### `extras remove`

設定から Extras を削除します。

```bash
skillshare extras remove <name> [--force] [-p|-g]
```

Source ファイルは残ります。ディレクトリの Extras は sync 済みの Target を残します。[単一ファイルの Extras](#single-file-extras) は Target を復元してから設定エントリを削除します。復元に失敗した場合は、再試行できるよう設定を残します。

### `extras collect`

Target 内のローカルファイルを Extras の Source ディレクトリに集約します。ファイルは Source にコピーされ、シンボリックリンクに置き換えられます。copy モードの Target では、ファイルは通常のコピーのまま残ります。[単一ファイルの Extras](#single-file-extras) では collect はサポートされていません。

Source にすでに存在するファイルはスキップされます。`--force` を使うと、それらを Target 側のバージョンで上書きします。たとえば、copy モードの Target で直接行った編集を取り込みたい場合に使います。内容がすでに Source と一致するファイルは、この場合もスキップされます。

```bash
skillshare extras collect <name> [--from <path>] [--force] [--dry-run] [-p|-g]
```

**オプション:**

| フラグ | 説明 |
|------|-------------|
| `--from <path>` | collect 元の Target ディレクトリ（複数 Target がある場合は必須） |
| `--force`, `-f` | Source にすでに存在するファイルを上書き |
| `--dry-run` | 変更を加えずに、collect される内容をプレビュー |

**例:**

```bash
# rules を Claude から Source に collect する
skillshare extras collect rules --from ~/.claude/rules

# collect される内容をプレビュー
skillshare extras collect rules --from ~/.claude/rules --dry-run

# Target での編集を既存の Source ファイルに上書きして取り込む
skillshare extras collect rules --force
```

---

## Sync モード

| モード | 動作 |
|------|----------|
| `merge`（デフォルト） | Target から Source へのファイルごとのシンボリックリンク |
| `copy` | ファイルごとのコピー |
| `symlink` | ディレクトリ全体のシンボリックリンク |
| `import` | [単一ファイルの Extras](#single-file-extras) のみ: Target のファイル内の `@<source file>` 行 |

Developer Mode がオフの Windows では、`merge` は各ファイルをリンクする代わりにコピーし、`sync` は `file links need Windows Developer Mode; copying instead` と表示します。その場合、`extras list` と `status` では Target が `copy` と表示されます。コピーは追跡されるため、後の sync で更新・削除され、自分のファイルは残り、ファイルのリンクが使えるようになるとリンクに置き換えられます。[Windows のトラブルシューティング](/docs/troubleshooting/windows#file-links-need-windows-developer-mode-copying-instead) を参照してください。

内容が同じローカルファイルは `local preserved` と表示され、`sync extras` はそれらに `--force` を提案しません。管理対象のリンクにはならず、ローカルファイルのままです。

モードを切り替える場合（例: `merge` から `copy` へ）、次の `sync` で既存のシンボリックリンクが自動的に新しいモードの形式に置き換えられます。`--force` は不要です — シンボリックリンクは常に安全に置き換えられます。ローカルで作成された通常のファイルを上書きするには `--force` が必要です。

---

## フラット化（Flatten）

一部の AI ツール（例: Claude Code の `/agents`）は、設定ディレクトリの**トップレベル**にあるファイルしか検出しません — サブディレクトリを再帰的には探索しません。Extras の Source が整理用にサブディレクトリを使っている場合、sync されたファイルはそのツールから見えなくなってしまいます。

`flatten` オプションは、Source 内のサブディレクトリの深さに関係なく、すべてのファイルを Target ルート直下に sync することでこれを解決します。

```yaml
extras:
  - name: agents
    targets:
      - path: ~/.claude/agents
        flatten: true
```

**動作:**
- `flatten: true`: `source/curriculum/tactician.md` → `target/tactician.md`
- `flatten: false`（デフォルト）: `source/curriculum/tactician.md` → `target/curriculum/tactician.md`

**ファイル名の衝突:** 異なるサブディレクトリにある 2 つのファイルが同じ名前を持つ場合（例: `team-a/agent.md` と `team-b/agent.md`）、最初のファイル（パスのアルファベット順でソート）が優先されます。以降の衝突は警告付きでスキップされます。

**制約:**
- `merge` モードと `copy` モードでのみ動作します — `symlink` モードとは併用できません
- `collect` は新しく collect したファイルを Source のルートに配置します（新規ファイルにはサブディレクトリへのマッピングはありません）

---

## 拡張子変換（Extension transforms） {#extension-transforms}

一部のツールは markdown を読み込みません。Gemini CLI は TOML の commands を、Codex CLI は TOML の agents を期待します。Target の `extension` フィールドは、sync 時に各 Source ファイルを Target のネイティブ形式に変換する外部スクリプトを実行します。

```yaml
extras:
  - name: commands
    targets:
      - path: .claude/commands        # extension なし — そのまま sync
      - path: .gemini/commands
        extension: gemini-commands           # sync 時に変換
```

**解決方法** — 裸の名前は extensions ディレクトリ配下（グローバルは `~/.config/skillshare/extensions/<name>`、Project は `.skillshare/extensions/<name>`）で解決されます。パス（`./x.sh`、`/abs/x`）はそのまま使われます。

**コピーの意味論** — `extension` は `copy` モードを暗黙的に指定します。`extension` を持つ Target に `mode: merge` または `mode: symlink` を設定するとエラーになります。

**一方向** — 変換は Source → Target の方向のみ実行されます。`extras collect` は extension を持つ Target をスキップします。

**上書きの安全性** — 生成される出力は `copy` モードと同じ衝突ルールに従います。出力先に残っているシンボリックリンクは自動的に置き換えられますが、ローカルで作成した既存の通常ファイルやディレクトリはそのまま残り、`--force` を指定しない限りスキップされます（`--force` を指定すると、衝突するディレクトリは生成されたファイルで丸ごと置き換えられます）。

### Extension のレイアウト

単一の実行ファイル、またはマニフェスト付きのディレクトリのいずれかです。

```
.skillshare/extensions/gemini-commands/
├── extension.yaml
├── convert.js        # 編集するマッピングルール
└── md-toml.js        # markdown/frontmatter/TOML 用のヘルパー
```

`extension.yaml`:

```yaml
run: ["node", "convert.js"]      # 明示的なコマンド（argv）。直接 exec される
output_ext: toml                  # .md → .toml。省略すると Source の拡張子を維持
description: "Markdown command → Gemini CLI TOML"
```

マニフェストのない単一ファイルの実行ファイルは直接 exec され（Unix 上ではシェバンに依存）、Source の拡張子を維持します。拡張子を変更する変換にはディレクトリ形式を使う必要があります。

### 実行契約

- Source ファイルの内容は **stdin** で渡され、スクリプトは変換後の内容を **stdout** に書き出します。
- 環境変数: `SS_SRC_PATH`、`SS_REL_PATH`（Source ルートからの相対パス — Gemini の `/namespace:command` の命名に便利）、`SS_TARGET_DIR`、`SS_MODE`。
- 非ゼロの終了コードはそのファイルを失敗としてマークします。他のファイルの処理は継続されます。

### クロスプラットフォーム対応

この仕組み自体はクロスプラットフォームですが、extension が実行されるかどうかはそのインタープリタ次第です。`run` は明示的なコマンドであるため、`node` や `python3` 用に書かれた extension は Windows、macOS、Linux で動作します。純粋な `bash` スクリプトは、シェルが利用可能な環境（Unix、または Git Bash のある Windows）でのみ動作します。参照用の extension には Node が推奨インタープリタです。プラットフォームを問わず一様に提供されるためです。

### 参照用の Extension

skillshare リポジトリは、`extensions/` 配下にサンプル extension（`gemini-commands`、`codex-agents`、`opencode-agents`）を同梱しています。いずれかを自分の extensions ディレクトリにコピーして調整してください — これらは参照用であり、自動的にはインストールされません。各参照用 extension は、フィールドマッピングだけを編集すれば済むよう `convert.js` を短く保っています。`md-toml.js` が markdown の読み込み、簡易フロントマターの解析、TOML の書き出しを担当します。

### レシピ: Codex agents

Codex CLI は markdown ではなく TOML の agents を期待します。`source` は任意のディレクトリを指せるため、agents の Source を Extras の Source として再利用し、`codex-agents` で変換できます。

```yaml
extras:
  - name: codex-agents
    source: ~/.config/skillshare/agents   # agents の source を再利用
    targets:
      - path: ~/.codex/agents
        extension: codex-agents
```

`skillshare sync extras` は各 `<agent>.md` を `~/.codex/agents/<agent>.toml` に変換し、フロントマターの `name`、`description`、`model` をマッピングし、markdown 本文を `developer_instructions` に折り込みます（その他のフロントマターキーは破棄されます）。[Codex custom agent schema](https://developers.openai.com/codex/subagents#custom-agent-file-schema) は `name`、`description`、`developer_instructions` を必須としているため、参照用の変換スクリプトは解決された name、description、または markdown 本文が空の場合に明確なエラーを報告します。agents の別コピーを用意する必要はありません。

Agent の Target には、extras を介さず `extension` を直接設定することもできます。詳細は [extension を使った agent の変換](/docs/understand/agents#extensions) を参照してください。

---

## レシピ: 複数の Agent 間で共有する指示

:::tip ダッシュボード
Web ダッシュボードを使えば、プレビュー、バックアップ、復元ボタン付きでこれを設定できます。
[1 つの AGENTS.md をツール間で共有する](../../how-to/daily-tasks/sharing-instructions.md)を参照してください。
ディレクトリの代わりに[単一ファイルの Extras](#single-file-extras) を使います。
:::

現在、多くのコーディング Agent は標準の指示として `AGENTS.md` を読み込みますが、それぞれユーザーレベルのコピーを別々のディレクトリに保持しています。複数の Target を持つ 1 つの Extras で、単一の Source ファイルをそれらすべてに配布できます。

```bash
skillshare extras init instructions \
  --target ~/.codex \
  --target ~/.config/opencode \
  --target ~/.claude \
  --target ~/.gemini \
  --no-tui
```

`AGENTS.md` を解決済みの Source ディレクトリ（デフォルトでは `~/.config/skillshare/extras/instructions/`）に置き、`skillshare sync extras` を実行します。

| Agent | グローバルパス | `AGENTS.md` の読み方 |
|-------|-------------|-------------------|
| Codex CLI | `~/.codex/AGENTS.md` | 直接読み込む |
| opencode | `~/.config/opencode/AGENTS.md` | 直接読み込む |
| Claude Code | `~/.claude/AGENTS.md` | `CLAUDE.md` の import 経由 |
| Antigravity | `~/.gemini/AGENTS.md` | `GEMINI.md` の import 経由 |

2 つの Agent はユーザーレベルで固定のファイル名を読み込むため、それぞれ sync されたファイルの隣に 1 行だけのファイルが必要です。これらは一度書けば、skillshare が以降触れることはありません。

```markdown title="~/.claude/CLAUDE.md"
@AGENTS.md
```

```markdown title="~/.gemini/GEMINI.md"
@AGENTS.md
```

Claude Code は `AGENTS.md` ではなく `CLAUDE.md` を読み込みます。import は、他の Agent と 1 つのファイルを共有するための方法として、[memory に関するドキュメント](https://code.claude.com/docs/en/memory)が推奨しているアプローチです。Antigravity はグローバルな rules を `~/.gemini/GEMINI.md` に保持し、相対的な `@filename` を rules ファイル自体のディレクトリを基準に解決するため、同じ 1 行で sync された `AGENTS.md` が取り込まれます。`~/.gemini` の Target は、同じグローバルファイルを読み込む Antigravity CLI もカバーします。

Source ファイルの名前は `AGENTS.md` のままにしておいてください。`memory.md` のような中立的な名前でも sync は同様にできますが、読み込まれなくなります。Codex は `AGENTS.md` を名前で連結しており、import の構文を持たないため、その名前でしかファイルを認識しません。

Target はディレクトリであるため、各 Target はそれぞれの Source 名でファイルを受け取ります。余計なファイルが 4 つの Target すべてに配布されてしまわないよう、Source ディレクトリには配布したいファイルだけを置いてください。

:::note
このレシピは、あなたが書いた指示を共有するものであり、Agent 自身が書くメモリを共有するものではありません。Agent は自身の学習内容を、Claude Code ならディレクトリ内の Markdown、Codex ならデータベース、Cursor ならファイル以外のストレージといった独自の形式で保存しており、Target 間でファイルをコピーしても移植できるものではありません。
:::

---

## 単一ファイルの Extras {#single-file-extras}

`file` を持つ Extras は、ディレクトリ全体ではなく Source ディレクトリ内の 1 つのファイルだけを sync します。
各 Target は `<path>/<as>` を受け取ります。`as` のデフォルトは `file` の名前です。固定のパスにある
1 つのファイルを読み込むツールならどれにでも使えます。たとえば Pi は `~/.pi/agent/APPEND_SYSTEM.md` を
システムプロンプトに追加します。その内容を dotfiles に `system.md` として置き、リンクで取り込みます。

```yaml
extras:
  - name: pi-prompt
    source: ~/dotfiles/prompts     # project モード: プロジェクトルートからの相対パス
    file: system.md                # ~/dotfiles/prompts/system.md
    targets:
      - path: ~/.pi/agent
        as: APPEND_SYSTEM.md       # ~/.pi/agent/APPEND_SYSTEM.md がリンクになる
```

同じ Extras を CLI から作成する場合:

```bash
skillshare extras init pi-prompt --file system.md --as APPEND_SYSTEM.md \
  --source ~/dotfiles/prompts --target ~/.pi/agent
skillshare sync extras
```

`extras init` の `--as` はすべての Target に適用されます。1 つの Target だけ別のファイル名にするには、
その Target を個別に追加します。

```bash
skillshare extras pi-prompt --add-target ~/Documents/prompts --as pi-system.md
```

ダッシュボードの [共有 AGENTS.md](../../how-to/daily-tasks/sharing-instructions.md) も単一ファイルの
Extras で、通常のリンク、名前の変更、import を組み合わせられます。

```yaml
extras:
  - name: personal
    file: AGENTS.md                # ~/.config/skillshare/extras/personal/AGENTS.md
    targets:
      - path: ~/.codex             # ~/.codex/AGENTS.md がリンクになる
      - path: ~/.gemini
        as: GEMINI.md              # ~/.gemini/GEMINI.md がリンクになる
      - path: ~/.claude
        as: CLAUDE.md
        mode: import               # ~/.claude/CLAUDE.md は内容を保持したままファイルを import する
```

| モード | Target のファイル |
|------|-------------|
| `merge`（デフォルト）または `symlink` | Source ファイルへのシンボリックリンク（Developer Mode がオフの Windows ではコピー） |
| `copy` | Source ファイルのコピー |
| `import` | あなたのファイル。先頭の管理ブロック内に `@<source file>` 行が入る |

`import` は `@` 行を `<!-- skillshare:instructions:begin -->` と
`<!-- skillshare:instructions:end -->` の間に置き、ファイルの残りの部分は一切変更しません。
Claude Code のように `@` import に従うツールでのみ使ってください。

ルール:

- リンクまたは `copy` モードの Target ファイルが使える共有ファイルは 1 つだけで、別の共有ファイルを同時に import することはできません。
- `file` と `as` は `/` や `\` を含まない単純なファイル名でなければなりません。
- `as` と `import` には `file` が必要です。`flatten` と `extension` は単一ファイルの Extras
  では使えません。
- Target にすでに別の通常ファイルやシンボリックリンクがある場合、sync はそれを保存してから
  `--force` なしで置き換えます。ディレクトリがある場合はスキップされます。
- リンク後に `modified` になった Target も置き換えられます。編集されたファイルは復元ポイントではなく、
  drift バックアップとして保存されます。
- リンクが内容の異なる通常ファイルに置き換えられた場合や、管理対象のコピーが編集された場合、`extras list` は `modified` と表示します。
- Target を `merge`、`symlink`、`copy` から `import` に切り替えると、前回の `import` モードの
  自分の内容（空の内容も含む）が戻ります。未使用なら接続前の内容を使います。import ブロックが追加され、
  編集されたコピーは先に drift バックアップとして保存されます。
- `extras remove` と `--remove-target --prune` は各 Target のファイルを元に戻します。リンク、
  コピー、または import 行が取り除かれ、最初の sync の前にあったファイルやシンボリックリンクが戻ります
  （元々なかった場合はファイルなし）。`modified` の Target は、先に drift バックアップとして保存されます。
  `--prune` なしの `--remove-target` は単一ファイルの Target を残し、管理対象から外して復元ポイントを破棄します。後の sync では削除されず、再接続時に新しい復元ポイントが記録されます。
- `extras collect` はサポートされていません。Target で行った編集を残すには、Source ファイルに
  コピーし直してください。共有 `AGENTS.md` の場合は、ダッシュボードの **AGENTS.md** タブで
  共有ファイル**に取り込む**を使うとこれを自動で行えます。

ダッシュボードでは、`file` が `AGENTS.md` の単一ファイルの Extras は **AGENTS.md** タブに表示され、
それ以外の単一ファイルの Extras は **Folders & files** に表示されます。そこでは **Add extra** で
**Folder** または **Single file** を選べ、各 Target には **File name** があり、単一ファイルでは
`merge`、`copy`、`import` を使えます。単一ファイルの **Name** は、自分で入力するまで
拡張子を除いたファイル名に合わせて入力されます（`APPEND_SYSTEM.md` なら `APPEND_SYSTEM`）。ダッシュボードはファイルの内容を編集しません。Source ファイルを直接編集してください。

### 1 つのフォルダーに複数のファイル

複数の単一ファイルの Extras が 1 つの `source` ディレクトリを共有できます。
ファイルごとに Extras を 1 つ作成してください。どの Extras にも指定されていないフォルダー内のファイルは sync されません。

```yaml
extras:
  - name: pi-system
    source: ~/dotfiles/pi
    file: system.md
    targets:
      - path: ~/.pi/agent
        as: APPEND_SYSTEM.md
  - name: pi-agents
    source: ~/dotfiles/pi
    file: agents.md
    targets:
      - path: ~/.pi/agent
        as: AGENTS.md
```

```bash
skillshare extras init pi-system --source ~/dotfiles/pi --file system.md \
  --as APPEND_SYSTEM.md --target ~/.pi/agent
skillshare extras init pi-agents --source ~/dotfiles/pi --file agents.md \
  --as AGENTS.md --target ~/.pi/agent
```

Project モードでは、`source` はプロジェクトルートからの相対パスで、プロジェクト内に収まる必要があります。絶対パスは拒否されます：

```bash
skillshare extras init review -p --source .skillshare/extras/prompts \
  --file review.md --target .claude/commands
skillshare extras init plan -p --source .skillshare/extras/prompts \
  --file plan.md --target .claude/commands
```

ダッシュボードでは、共有 Extras フォルダー内の単一ファイルに **Source folder** 欄があります。デフォルトは Extras の名前です。別の Extras のフォルダーを入力すると、両方のファイルを 1 つのフォルダーにまとめられます。

バックアップは skillshare の state ディレクトリ（macOS と Linux では
`~/.local/state/skillshare/extras/backups/`）に、ファイルごとに最新 10 件まで保存されます。
drift バックアップはその中の `extras/backups/<id>/drift/` に保存されます。`<id>` は Target の
ファイルのパスから導出されます。復元でこれらが使われることはありません。保存された任意のバージョンを
一覧表示または復元するには、[`backup files`](./backup.md#file-history) を使います。

---

## ディレクトリ構造

```
~/.config/skillshare/
├── config.yaml          # extras の設定はここにあります
├── skills/              # skill のソース
└── extras/              # extras のソースルート
    ├── rules/           # extras/rules/ のソースファイル
    │   ├── coding.md
    │   └── testing.md
    └── prompts/
        └── review.md
```

---

## 設定

`config.yaml` の場合:

```yaml
# 任意: グローバルなデフォルト extras source ディレクトリを設定
extras_source: ~/my-extras

extras:
  - name: rules
    source: ~/company-shared/rules    # extras ごとの任意の上書き
    targets:
      - path: ~/.claude/rules
      - path: ~/.cursor/rules
        mode: copy
  - name: agents
    targets:
      - path: ~/.claude/agents
        flatten: true                  # サブディレクトリのファイルをフラットに sync
  - name: prompts
    targets:
      - path: ~/.claude/prompts
```

### Source の解決優先順位

各 Extras の Source ディレクトリは、3 段階の優先順位で解決されます。

1. **Extras ごとの `source`**（最優先） — 正確なパスをそのまま使用
2. **`extras_source`** — `<extras_source>/<name>/`
3. **デフォルト** — `~/.config/skillshare/extras/<name>/`（グローバル）または `.skillshare/extras/<name>/`（Project）

`extras list --json` の出力には、どのレベルでパスが解決されたかを示す `source_type` フィールド（`per-extra`、`extras_source`、または `default`）が含まれます。

:::tip 自動設定
`extras_source` は、`skillshare init` を実行したとき、または `extras init` で最初の Extras を作成したときに、デフォルトパス（`~/.config/skillshare/extras/`）へ自動的に設定されます。後で変更するには `skillshare extras source <path>` を使用してください。
:::

---

## Sync

Extras は以下で sync されます。

```bash
skillshare sync extras        # extras のみを sync
skillshare sync --all         # skills + extras を一緒に sync
```

`--json`、`--dry-run`、`--force` オプションを含む sync の完全なドキュメントは [sync extras](/docs/reference/commands/sync#sync-extras) を参照してください。

---

## ワークフロー

```bash
# 1. 新しい extras を作成
skillshare extras init rules --target ~/.claude/rules --target ~/.cursor/rules

# 1b. またはカスタムの source ディレクトリを指定
skillshare extras init rules --target ~/.claude/rules --source ~/my-rules

# 1c. 既存の extras を再構成する（上書き）
skillshare extras init rules --target ~/.cursor/rules --force

# 2. source ディレクトリにファイルを追加する
# （解決済みの source ディレクトリを編集: skillshare extras list --json で確認）

# 3. target へ sync する
skillshare sync extras

# 4. ステータスを一覧表示する（source_type で各 extras の source の解決元がわかる）
skillshare extras list

# 5. target で編集したファイルを source に collect する
skillshare extras collect rules --from ~/.claude/rules

# 6. グローバルな extras source ディレクトリを変更する
skillshare extras source ~/company-shared/extras
```

---

## 関連項目

- [sync](/docs/reference/commands/sync#sync-extras) — Extras を Target へ sync
- [status](/docs/reference/commands/status) — Extras のファイル数と Target 数を表示
- [Configuration](/docs/reference/targets/configuration#extras) — Extras の設定リファレンス
