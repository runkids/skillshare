---
sidebar_position: 5
---

# tui

インタラクティブ TUI モードをグローバルに切り替えます。

## こんなときに使う

- すべてのコマンドで、インタラクティブ TUI よりプレーンテキスト出力を好む場合
- CI/CD パイプラインや非インタラクティブな環境で skillshare を実行している場合
- 無効化した TUI を再度有効にしたい場合

## 構文

```bash
skillshare tui          # 現在の状態を表示
skillshare tui on       # すべてのコマンドで TUI を有効化
skillshare tui off      # すべてのコマンドで TUI を無効化（プレーンテキスト出力）
```

## 動作

TUI が無効化されている場合、通常はインタラクティブなインターフェースを起動するコマンド（`list`、`log`、`search`、`audit rules`、`trash`、`restore`、`diff`、`target list`、`mcp`）は、プレーンテキスト出力にフォールバックします — これはすべてのコマンドに `--no-tui` を渡すのと同等です。

| 状態 | 意味 |
|-------|---------|
| `on (default)` | config に TUI キーが存在しない — TUI 有効 |
| `on` | 明示的に有効化 |
| `off` | 明示的に無効化 |

この設定は `config.yaml` に `tui: false` として保存されます。キーを削除するとデフォルト（有効）に戻ります。

## 優先順位

個々のコマンドに指定した `--no-tui` フラグは、常にグローバル設定より優先されます。例えば、`tui on` が設定されていても `skillshare list --no-tui` は TUI を無効化します。

## 例

```
$ skillshare tui
  TUI  on · default

$ skillshare tui off
✓ TUI disabled

$ skillshare tui
  TUI  off

$ skillshare tui on
✓ TUI enabled
```

## 関連項目

- [list](./list.md) — Skill を一覧表示（有効時は TUI を使用）
- [log](./log.md) — 操作ログを表示（有効時は TUI を使用）
- [target](./target.md) — ターゲット一覧（有効時は TUI を使用）
- [audit rules](./audit-rules.md) — 監査ルールブラウザ（有効時は TUI を使用）
