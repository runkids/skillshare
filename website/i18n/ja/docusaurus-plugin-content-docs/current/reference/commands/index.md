---
sidebar_position: 1
---

# コマンド

skillshare のすべてのコマンドに関する完全なリファレンスです。

## 何をしたいですか？

| やりたいこと... | コマンド |
|--------------|---------|
| skillshare を初めてセットアップする | [`init`](./init.md) |
| GitHub から Skill をインストールする | [`install`](./install.md) |
| 自分の Skill を作成する | [`new`](./new.md) |
| すべての AI CLI に Skill を sync する | [`sync`](./sync.md) |
| 何が同期されていないか確認する | [`status`](./status.md) / [`diff`](./diff.md) |
| コミュニティの Skill を検索する | [`search`](./search.md) |
| インストール済みの Skill を更新する | [`check`](./check.md) の後に [`update`](./update.md) |
| 削除せずに一時的に Skill を隠す | [`enable` / `disable`](./enable.md) |
| git で変更を保存・sync する | [`commit`](./commit.md) / [`push`](./push.md) / [`pull`](./pull.md) |
| すべてのツールに向けて MCP サーバーを一度だけ設定する | [`mcp`](./mcp.md) |
| サポート対象のツール全体で完全な plugin を管理する | [`plugin`](./plugin.md) |
| Skill 以外のリソース（rule、command）を管理する | [`extras`](./extras.md) |
| 単一ファイルの `.md` agent を管理する | ほとんどのコマンドは `agents` または `--kind agent` を受け付けます — [Agents](/docs/understand/agents) を参照 |
| どの Skill が最もコンテキストトークンを使っているか確認する | [`analyze`](./analyze.md) |
| 壊れたものを直す | [`doctor`](./doctor.md) |
| シェルでタブ補完を有効にする | [`completion`](./completion.md) |
| web ダッシュボードを開く | [`ui`](./ui.md) |

---

## 概要

| カテゴリ | コマンド |
|----------|----------|
| **コア** | `init`, `install`, `uninstall`, `list`, `search`, `sync`, `status` |
| **Skill 管理** | `new`, `check`, `update`, `upgrade`, `enable`, `disable` |
| **MCP 接続** | `mcp`（`add`, `edit`, `import`, `list`, `remove`, `restore`）, `sync mcp` |
| **Plugin 管理** | `plugin`（`list`, `discover`, `add`, `import`, `inspect`, `sync`, `check`, `update`, `enable`, `disable`, `remove`） |
| **ターゲット管理** | `target`, `diff` |
| **Extras 管理** | `extras`（`init`, `list`, `remove`, `collect`） |
| **Sync 操作** | `collect`, `backup`, `restore`, `trash`, `commit`, `push`, `pull` |
| **セキュリティ & ユーティリティ** | `analyze`, `audit`, `hub`, `log`, `doctor`, `tui`, `ui`, `completion`, `version` |

---

## コアコマンド

| コマンド | 説明 |
|---------|-------------|
| [init](./init.md) | 初回セットアップ |
| [install](./install.md) | リポジトリまたはパスから Skill を追加 |
| [uninstall](./uninstall.md) | Skill を削除 |
| [list](./list.md) | すべての Skill を一覧表示 |
| [search](./search.md) | Skill を検索 |
| [sync](./sync.md) | すべてのターゲットに Skill を push |
| [status](./status.md) | sync の状態を表示 |

## Skill 管理

| コマンド | 説明 |
|---------|-------------|
| [new](./new.md) | 新しい Skill を作成 |
| [check](./check.md) | 利用可能な更新を確認 |
| [update](./update.md) | Skill またはトラック対象リポジトリを更新 |
| [upgrade](./upgrade.md) | CLI または組み込み Skill をアップグレード |
| [enable / disable](./enable.md) | Skill を一時的に有効化/無効化 |

## ターゲット管理

| コマンド | 説明 |
|---------|-------------|
| [target](./target.md) | ターゲットを管理 |
| [diff](./diff.md) | ソースとターゲットの差分を表示 |

## Extras 管理

| コマンド | 説明 |
|---------|-------------|
| [extras](./extras.md) | Skill 以外のリソース（rule、command、prompt）を管理 |

## MCP と Plugin

| コマンド | 説明 |
|---------|-------------|
| [mcp](./mcp.md) | MCP サーバーを一度定義し、各ツールのネイティブ config に sync |
| [hooks](./hooks.md) | ネイティブ hooks を管理 |
| [plugin](./plugin.md) | 完全な plugin をインストールし、どのツールに配布するか選択 |

## Sync 操作

| コマンド | 説明 |
|---------|-------------|
| [collect](./collect.md) | ターゲットからソースへ Skill を収集 |
| [backup](./backup.md) | ターゲットのバックアップを作成 |
| [restore](./restore.md) | バックアップからターゲットを復元 |
| [trash](./trash.md) | trash 内のアンインストール済み Skill を管理 |
| [commit](./commit.md) | push せずにローカルの git commit を作成 |
| [push](./push.md) | commit して git remote に push |
| [pull](./pull.md) | git remote から pull して sync |

## セキュリティ & ユーティリティ

| コマンド | 説明 |
|---------|-------------|
| [analyze](./analyze.md) | コンテキストウィンドウの使用状況を分析 |
| [audit](./audit.md) | Skill のセキュリティ脅威をスキャン |
| [log](./log.md) | 操作ログと監査ログを表示 |
| [doctor](./doctor.md) | 問題を診断 |
| [tui](./tui.md) | インタラクティブ TUI モードを切り替え |
| [ui](./ui.md) | web ダッシュボードを起動 |
| [hub](./hub.md) | Skill ハブのソースを管理 |
| [completion](./completion.md) | シェル補完スクリプトを生成 |
| [version](./version.md) | CLI のバージョンを表示 |

---

## 共通フラグ

ほとんどのコマンドは以下をサポートします。

| フラグ | 説明 |
|------|-------------|
| `--dry-run`, `-n` | 変更を加えずにプレビュー |
| `--help`, `-h` | ヘルプを表示 |

---

## クイックリファレンス

```bash
# Setup
skillshare init
skillshare init --remote git@github.com:you/skills.git

# Install skills
skillshare install anthropics/skills/skills/pdf
skillshare install github.com/team/skills --track

# Create skill
skillshare new my-skill

# Sync
skillshare sync
skillshare sync --dry-run

# Git checkpoints / cross-machine
skillshare commit -m "Update skill"
skillshare push -m "Add skill"
skillshare pull

# Status
skillshare status
skillshare list
skillshare diff

# Enable/disable skills
skillshare disable draft-*
skillshare enable draft-*

# Maintenance
skillshare update --all
skillshare analyze
skillshare audit
skillshare log
skillshare doctor
skillshare backup

# TUI preferences
skillshare tui            # Show current status
skillshare tui off        # Disable interactive TUI
skillshare tui on         # Re-enable TUI

# Web UI
skillshare ui
skillshare ui -p          # Project mode

# Hub
skillshare hub list
skillshare hub add https://hub.example.com/index.json

# Check for updates
skillshare check

# Trash management
skillshare trash list
skillshare trash restore my-skill

# Shell completion
skillshare completion bash --install
skillshare completion zsh --install

# Version
skillshare version
```

---

## 関連項目

- [Quick Reference](/docs/getting-started/quick-reference) — コマンドチートシート
- [Workflows](/docs/how-to/daily-tasks) — よくある使用パターン
