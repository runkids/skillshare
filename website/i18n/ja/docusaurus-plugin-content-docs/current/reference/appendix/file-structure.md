---
sidebar_position: 3
---

# ファイル構造

skillshare のディレクトリレイアウトとファイルの場所です。

## 概要

```
~/.config/skillshare/        # XDG_CONFIG_HOME
├── config.yaml              # Configuration file
├── audit-rules.yaml         # Custom audit rules (optional)
├── mcp.yaml                 # MCP servers, if sources.mcp points here (optional)
├── skills/                  # Skills source (skills + metadata)
│   ├── .metadata.json       # Installed skill metadata (auto-managed)
│   ├── .skillignore         # Optional: exclude skills from sync
│   ├── my-skill/            # Regular skill
│   │   ├── SKILL.md         # Skill definition (required)
│   ├── code-review/         # Another skill
│   │   └── SKILL.md
│   └── _team-skills/        # Tracked repository
│       ├── .git/            # Git history preserved
│       ├── frontend/
│       │   └── ui/
│       │       └── SKILL.md
│       └── backend/
│           └── api/
│               └── SKILL.md
├── agents/                  # Agents source (single .md files)
│   ├── .agentignore         # Optional: exclude agents from sync
│   ├── reviewer.md          # Agent file
│   └── auditor.md           # Another agent
├── rules/                   # Extras source (if configured)
│   ├── coding.md
│   └── testing.md
└── commands/                # Extras source (if configured)
    └── deploy.md

~/.local/share/skillshare/   # XDG_DATA_HOME
├── backups/                 # Backup directory
│   ├── 2026-01-20_15-30-00/
│   │   ├── claude/          # Skills backup for claude
│   │   ├── claude-agents/   # Agents backup for claude
│   │   └── cursor/
│   └── 2026-01-19_10-00-00/
│       └── claude/
└── trash/                   # Uninstalled skills/agents (7-day retention)
    ├── my-skill_2026-01-20_15-30-00/
    │   └── SKILL.md
    └── old-skill_2026-01-19_10-00-00/
        └── SKILL.md

~/.local/state/skillshare/   # XDG_STATE_HOME
├── logs/                    # Operation logs (JSONL)
│   ├── operations.log       # install, sync, update, etc.
│   └── audit.log            # Security audit scans
├── mcp/                     # MCP sync state (auto-managed)
│   ├── state.json           # Which native entries Skillshare owns
│   └── backups/             # Agent files before each write (newest 20 per file)
└── plugins/                 # Reviewed local copies of plugin sources

~/.cache/skillshare/         # XDG_CACHE_HOME      
├── version-check.json       # Version check cache (24h TTL)
└── ui/                      # Web UI dist cache
    └── 0.13.0/              # Per-version cached assets
        ├── index.html
        └── assets/
```

---

## 設定ファイル

### 場所

```
~/.config/skillshare/config.yaml
```

**XDG での上書き:**
```
XDG_CONFIG_HOME=/custom/path → /custom/path/skillshare/config.yaml
```

**Windows のデフォルト:**
```
%AppData%\skillshare\config.yaml
```

### 内容

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/runkids/skillshare/main/schemas/config.schema.json
source: ~/.config/skillshare/skills
agents_source: ~/.config/skillshare/agents  # Optional; defaults to <source parent>/agents
mode: merge
targets:
  claude:
    path: ~/.claude/skills
    agents:                                 # Optional; enables agent sync for this target
      path: ~/.claude/agents
  cursor:
    path: ~/.cursor/skills
ignore:
  - "**/.DS_Store"
  - "**/.git/**"
```

完全なリファレンスは [Configuration](/docs/reference/targets/configuration) を参照してください。

---

## Metadata ファイル

### 場所

```
~/.config/skillshare/skills/.metadata.json
```

インストール済み・Tracked された Skill に関するメタデータを保存します。マルチマシン構成で git 経由の
Sync ができるよう、Source ディレクトリ内に置かれます。`install`、`uninstall`、`update` によって
**自動管理**されます — 手動で編集しないでください。

### 内容

```json
{
  "skills": [
    {
      "name": "pdf",
      "source": "anthropics/skills/skills/pdf"
    },
    {
      "name": "_team-skills",
      "source": "github.com/team/skills",
      "tracked": true
    }
  ]
}
```

各エントリは Skill の名前とそのインストール Source を記録します。Tracked repos（`_` プレフィックス
付き）は `update` と `check` の操作のためにリポジトリの完全な URL を含みます。

---

## Source ディレクトリ

### 場所

```
~/.config/skillshare/skills/
```

**Windows:**
```
%AppData%\skillshare\skills\
```

### 構造

```
skills/
├── .metadata.json                # Centralized skill metadata (auto-managed)
├── skill-name/                   # Skill directory
│   ├── SKILL.md                  # Required: skill definition
│   ├── examples/                 # Optional: example files
│   └── templates/                # Optional: code templates
├── frontend/                     # Category folder (via --into or manual)
│   └── react-skill/              # Skill in subdirectory
│       └── SKILL.md              # Synced as frontend__react-skill
└── _tracked-repo/                # Tracked repository
    ├── .git/                     # Git history
    └── ...                       # Skill subdirectories
```

---

## Skill ファイル

### SKILL.md（必須）

Skill の定義ファイルです。

```markdown
---
name: skill-name
description: Brief description
---

# Skill Name

Instructions for the AI...
```

詳細は [Skill フォーマット](/docs/understand/skill-format) を参照してください。

### .skillfollow / .skillfollow.local {#skillfollow-optional}

**設定された skills source ルート**に直接の子リンク名（例 `_team-skills`）を宣言。Local はマシン固有名を追加し、ignore pattern や入れ子 repo 設定ではありません。Source `.gitignore` に `/_team-skills`、`/.skillfollow.local`（末尾 `/` なし）を追加し、各マシンでリンクを作成。`_` 接頭辞と `.git` は tracked repo、ルート `SKILL.md` は未対応。[設定・厳密な名前・安全性](../skillfollow.md)参照。

### .skillignore（任意） {#skillignore-optional}

Discovery から Skill を除外します。2つの場所に対応しています。

**リポジトリレベル** — Tracked された Skill リポジトリのルートに置きます。インストール時の
Discovery とインストール後のすべてのコマンド（`doctor`、`status`、`list`、`sync` など）に影響します。

```text title="_team-skills/.skillignore"
# Hide vendored packages from discovery
.venv
node_modules

# Exclude internal tooling
validation-scripts
prompt-eval-*
```

**Source ルート** — Source ディレクトリのルート（`~/.config/skillshare/skills/.skillignore`）に
置きます。すべての Skill（Tracked と非 Tracked の両方）にグローバルに適用されます。

```text title="~/.config/skillshare/skills/.skillignore"
# Temporarily mute a skill
my-experimental-skill

# Exclude all drafts
draft-*
```

[gitignore の構文](https://git-scm.com/docs/gitignore) を使用します — 1行につき1パターンです。
`*`（1セグメント）、`**`（任意の深さ）、`?`、`[abc]`（文字クラス）、`!pattern`（否定）、
`/pattern`（アンカー付き）、`pattern/`（ディレクトリのみ）、`\#`/`\!`（リテラルのエスケープ）に
対応しています。`#` で始まる行はコメントです。`internal-tools` のようなグループ名はそのディレクトリ
配下のすべての Skill を除外します。`internal-tools/helper` は特定の Skill のみを除外します。
両方のレイヤーが適用されます — どちらかが一致すれば、その Skill は除外されます。

:::tip
`.skillignore` は3つのフィルタリングレイヤーのうちの1つです。Target ごとのフィルターや SKILL.md の
`targets` を含むすべてのシナリオについては [Skill のフィルタリング](/docs/how-to/daily-tasks/filtering-skills)
を参照してください。
:::

### .skillignore.local（任意） {#skillignorelocal-optional}

`.skillignore` と併用するローカル専用のオーバーライドファイルです。`.skillignore` と同じ
ディレクトリ（Source ルートまたは Tracked repo のルート）に置きます。`.skillignore.local` の
パターンは `.skillignore` の後に追加されるため、否定パターン（`!pattern`）でベースファイルを
オーバーライドできます。

```text title="_team-skills/.skillignore.local"
# The repo's .skillignore blocks private-*, but I need my own
!private-mine
```

このファイルはバージョン管理に**コミットしないでください** — `.gitignore` に追加します。これは、
リポジトリの利用者がリポジトリのメンテナーの `.skillignore` を変更することなくローカルで
オーバーライドできるようにするために存在します。

有効な場合、`sync -v`、`status`、`doctor` は `.local active` インジケーターを表示します。


---

## Agent ファイル

Agent は Skill とは別のリソース種別です。隣接する Source ディレクトリに存在し、Agent 対応の
Target（Claude、Cursor、Augment、OpenCode）に Sync されます。

### Agent の Source ディレクトリ

```
~/.config/skillshare/agents/      # Global mode
.skillshare/agents/               # Project mode
```

Agent の Source は、`skills/` と共に `skillshare init` によって自動的に作成されます。Global の場所は
`agents_source` config フィールドで上書きできます。Project mode は常に `.skillshare/agents/` を
使います。

### Agent ファイルのフォーマット

各 Agent はフロントマター付きの単一の Markdown ファイルです。

```markdown title="~/.config/skillshare/agents/reviewer.md"
---
name: reviewer
description: Reviews pull requests for security and style issues.
---

# Reviewer

Instructions for the AI agent...
```

Agent のファイル名は `a-z`、`0-9`、`_`、`-`、`.` のみを使用する必要があります。Skill とは異なり、
Agent は**単一ファイル**です — サブディレクトリを含みません。

完全なファイルフォーマットと発見ルールについては [Agents](/docs/understand/agents) を参照してください。

### .agentignore（任意）

Sync から Agent を除外します。Agent の Source ルートに置きます。

```text title="~/.config/skillshare/agents/.agentignore"
# Hide drafts
draft-*

# Disable a specific agent
experimental-reviewer
```

[gitignore の構文](https://git-scm.com/docs/gitignore) を使用します。`.skillignore` と同じ
パターンルール（`*`、`**`、`!negation`、`#` コメントなど）が適用されます。無効化された Agent は
Source ディレクトリに残りますが、Sync からは除外されます。

`skillshare disable <agent>` と `skillshare enable <agent>` はエントリを自動的に追加/削除します。

### .agentignore.local（任意）

ローカル専用のオーバーライドファイルです（`.skillignore.local` と同じパターン）。`.agentignore` の
隣に置きます。パターンは `.agentignore` の後に追加されるため、`!negation` パターンでベースファイルが
無効化した Agent を再度有効化できます。バージョン管理にはコミットしないでください。

---

## バックアップディレクトリ

### 場所

```
~/.local/share/skillshare/backups/
```

### 構造

```
backups/
└── <timestamp>/             # YYYY-MM-DD_HH-MM-SS
    ├── claude/              # Backup of target
    │   ├── skill-a/
    │   └── skill-b/
    └── cursor/
        └── ...
```

バックアップは以下のタイミングで作成されます。
- `sync` と `target remove` の前に自動的に
- `skillshare backup` で手動に

---

## Trash ディレクトリ

### 場所

```
~/.local/share/skillshare/trash/
```

**Project mode:**
```
<project>/.skillshare/trash/
```

### 構造

```
trash/
└── <skill-name>_<timestamp>/    # skill-name_YYYY-MM-DD_HH-MM-SS
    ├── SKILL.md
    └── ...                      # All original files preserved
```

Trash に入れられた Skill は:
- `skillshare uninstall` によって作成される
- 7日間保持された後、自動的にクリーンアップされる
- 元の Skill 名とタイムスタンプで命名される

---

## ログディレクトリ

### 場所

```
~/.local/state/skillshare/logs/
```

**Project mode:**
```
<project>/.skillshare/logs/
```

---

## Target ディレクトリ

Target は AI CLI の Skill ディレクトリです。Sync 後、それらには Source へのシンボリックリンク
（またはコピー）が含まれます。

### Merge モード

各 Skill が個別にシンボリックリンクされます。マニフェストが孤立したリンクのクリーンアップのために
管理対象の Skill を追跡します。
```
~/.claude/skills/
├── my-skill -> ~/.config/skillshare/skills/my-skill
├── code-review -> ~/.config/skillshare/skills/code-review
├── local-only/              # Not symlinked (user-created, preserved)
└── .skillshare-manifest.json  # Tracks managed skills
```

### Copy モード

各 Skill が実ファイルとしてコピーされます。マニフェストは差分 Sync のためにチェックサムを
追跡します。
```
~/.cursor/skills/
├── my-skill/                  # Real files (copied from source)
├── code-review/               # Real files
├── local-only/                # User-created, preserved
└── .skillshare-manifest.json  # Tracks managed skills + checksums
```

### Symlink モード

ディレクトリ全体がシンボリックリンクされます。
```
~/.claude/skills -> ~/.config/skillshare/skills/
```

### ファイルリンクの代わりのコピー（Windows） {#copies-in-place-of-file-links-windows}

Developer Mode がオフの Windows では、agent の Target と `merge` モードのディレクトリ Extras は、ファイルのリンクの代わりにコピーを受け取ります。その場合、Target ディレクトリには各コピーのチェックサムを記録する `.skillshare-manifest.json` が置かれ、後の Sync でコピーを更新・削除しつつ、自分のファイルはそのまま残せます。
```
~/.claude/agents/
├── reviewer.md                # Copy of the source agent
├── local-agent.md             # User-created, preserved
└── .skillshare-manifest.json  # Tracks copied agents + checksums
```

単一ファイルの Extras（共有 `AGENTS.md` など）は、コピーを Target ディレクトリではなく skillshare の [extras バックアップフォルダー](../commands/extras.md#single-file-extras)に記録します。

---

## Tracked Repositories

Tracked repos（`--track` でインストール）は git の履歴を保持します。

```
_team-skills/
├── .git/                    # Git preserved
├── frontend/
│   └── ui/
│       └── SKILL.md
└── backend/
    └── api/
        └── SKILL.md
```

### 命名規則

- `_` プレフィックス: Tracked repository
- フラット化された名前の `__`: パスセパレータ

**Source では:**
```
_team-skills/frontend/ui/SKILL.md
```

**Target では（フラット化後）:**
```
_team-skills__frontend__ui/SKILL.md
```

---

## プラットフォームの違い

:::tip XDG Base Directory
skillshare は XDG Base Directory Specification に従います。`XDG_CONFIG_HOME`、`XDG_DATA_HOME`、
`XDG_STATE_HOME`、`XDG_CACHE_HOME` でベースディレクトリを上書きできます。

詳細は [環境変数](./environment-variables.md#xdg_config_home) を参照してください。
:::

### macOS / Linux

| 項目 | パス |
|------|------|
| Config | `~/.config/skillshare/config.yaml` |
| Metadata | `~/.config/skillshare/skills/.metadata.json` |
| Skills source | `~/.config/skillshare/skills/` |
| Agents source | `~/.config/skillshare/agents/` |
| Backups | `~/.local/share/skillshare/backups/` |
| Trash | `~/.local/share/skillshare/trash/` |
| Logs | `~/.local/state/skillshare/logs/` |
| Version cache | `~/.cache/skillshare/version-check.json` |
| UI cache | `~/.cache/skillshare/ui/{version}/` |
| リンクの種類 | シンボリックリンク |

### Windows

| 項目 | パス |
|------|------|
| Config | `%AppData%\skillshare\config.yaml` |
| Metadata | `%AppData%\skillshare\skills\.metadata.json` |
| Skills source | `%AppData%\skillshare\skills\` |
| Agents source | `%AppData%\skillshare\agents\` |
| Backups | `%AppData%\skillshare\backups\` |
| Trash | `%AppData%\skillshare\trash\` |
| Logs | `%AppData%\skillshare\logs\` |
| Version cache | `%AppData%\skillshare\version-check.json` |
| UI cache | `%AppData%\skillshare\ui\{version}\` |
| リンクの種類 | フォルダーは NTFS Junctions、単一ファイルはシンボリックリンク（Developer Mode が必要、ない場合はコピー） |

## XDG Base Directory のレイアウト

skillshare は Unix システム上で [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir/latest/)
に従います。

| XDG 変数 | デフォルトパス | skillshare の用途 |
|-------------|-------------|---------------------|
| `XDG_CONFIG_HOME` | `~/.config` | `skillshare/config.yaml`、`skillshare/skills/`（`.metadata.json` を含む）、`skillshare/agents/` |
| `XDG_DATA_HOME` | `~/.local/share` | `skillshare/backups/`、`skillshare/trash/` |
| `XDG_STATE_HOME` | `~/.local/state` | `skillshare/logs/` |
| `XDG_CACHE_HOME` | `~/.cache` | `skillshare/ui/`（ダウンロードされた Web ダッシュボード） |

### Windows のパス

| 用途 | パス |
|---------|------|
| Config + Skills | `%AppData%\skillshare\` |
| Data（バックアップ、trash） | `%AppData%\skillshare\` |
| State（ログ） | `%AppData%\skillshare\` |
| Cache（UI） | `%AppData%\skillshare\` |

### 移行に関する注意

XDG 分割前のバージョンからアップグレードする場合、skillshare は初回実行時に古い場所
（`~/.config/skillshare/`）から正しい XDG ディレクトリへ自動的にデータを移行します。

---

## 関連項目

- [Configuration](/docs/reference/targets/configuration) — Config ファイルの詳細
- [Skill フォーマット](/docs/understand/skill-format) — SKILL.md フォーマット
- [Agents](/docs/understand/agents) — Agent ファイルフォーマットと発見
- [Tracked Repositories](/docs/understand/tracked-repositories) — Tracked repos
