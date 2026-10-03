---
sidebar_position: 4
---

# trash

trash ディレクトリ内のアンインストール済み Skill と agent を管理します。

```bash
skillshare trash list                    # インタラクティブ TUI（TTY 内）
skillshare trash list --no-tui           # プレーンテキスト出力
skillshare trash restore my-skill        # trash から復元
skillshare trash restore my-skill -p     # project モードで復元
skillshare trash delete my-skill         # trash から完全に削除
skillshare trash empty                   # trash を空にする
skillshare trash agents list             # trash 内の agent を一覧表示
skillshare trash agents restore tutor    # trash から agent を復元
skillshare trash --all list              # trash 内の Skill と agent を一覧表示
```

## 使うタイミング

- 最近アンインストールした Skill や agent を復元する（7 日以内）
- trash 内の項目を完全に削除して容量を空ける
- 自動失効する前に trash の中身を確認する

## インタラクティブ TUI

TTY 上では、`trash list` はゴミ箱内の項目をインタラクティブなリスト（新しい順）で開きます。1 つ以上を選択して復元または完全削除でき、ゴミ箱全体を空にすることもできます。いずれも実行前に確認します。項目を開くとファイルが表示され、復元前に内容を確認できます。キーは画面下部に表示されます。`--all` を使う場合や種類を指定しない場合は、Skill と Agent がまとめて表示されます。

一部の項目が失敗しても（たとえば、復元する Skill と同じ名前がすでにソースにある場合）、残りの項目は処理され、結果に失敗した項目が表示されます。

`--no-tui` を使うと TUI をスキップし、代わりにプレーンテキストを出力します。

```bash
skillshare trash list --no-tui           # プレーンテキスト出力
skillshare trash list --no-tui | less    # 手動でページャーにパイプする
```

## 種別フィルタ

デフォルトでは、trash は **Skill** に対して動作します。agent を対象にするには `agents` 位置引数キーワードを、両方を含めるには `--all` を使用してください。

```bash
skillshare trash list                    # Skill のみ（デフォルト）
skillshare trash agents list             # agent のみ
skillshare trash --all list              # Skill と agent の両方
skillshare trash agents restore tutor    # trash 内の agent を復元
skillshare trash agents empty            # agent の trash のみを空にする
```

## サブコマンド

### list（エイリアス: `ls`）

trash 内の全項目を表示します。ターミナルではインタラクティブ TUI を起動し、`--no-tui` を指定した場合や非 TTY の場合はプレーンテキストを出力します。

```bash
skillshare trash list
skillshare trash agents list
skillshare trash --all list --no-tui
```

プレーンテキスト出力:

```
Trash
  my-skill      1.2 KB · 2d ago
  old-helper    800 B · 5d ago

2 items, 2.0 KB
  Each item is removed for good 7 days after it was trashed
```

### restore

trash 内の最新バージョンを source ディレクトリに復元します。

```bash
skillshare trash restore my-skill
skillshare trash agents restore tutor
```

```
✓ Restore   my-skill → ~/.config/skillshare/skills · trashed 2d ago

Next
  skillshare sync  link it into your targets again
```

agent の場合、復元時のヒントには代わりに `skillshare sync agents` が提案されます。

同じ名前の項目が既に source に存在する場合、復元は失敗します。先に既存の項目をアンインストールするか、別の名前を使用してください。

### delete（エイリアス: `rm`）

trash から単一の項目を完全に削除します。

```bash
skillshare trash delete my-skill
skillshare trash agents delete tutor
```

```
✓ Permanently deleted my-skill
```

### empty

trash 内のすべての項目を完全に削除します（確認プロンプトあり）。

```bash
skillshare trash empty
skillshare trash agents empty
```

```
! This will permanently delete 3 items from trash
? Continue? [y/N] y
✓ Emptied trash: 3 items permanently deleted · 0.1s
```

## Backup vs Trash

この 2 つの安全機構は、それぞれ異なるものを保護します。

| | backup | trash |
|---|---|---|
| **保護対象** | target ディレクトリ（sync のスナップショット） | source の Skill と agent（アンインストール） |
| **保存場所** | `~/.local/share/skillshare/backups/` | `~/.local/share/skillshare/trash/`（Skill）、`.../trash/agents/`（agent） |
| **トリガー** | `sync`, `target remove` | `uninstall` |
| **復元方法** | `skillshare restore <target>` | `skillshare trash restore <name>` |
| **自動クリーンアップ** | 手動（`backup --cleanup`） | 7 日 |

## オプション

| フラグ | 説明 |
|------|-------------|
| `agents` | 位置引数キーワード — Skill の代わりに agent を対象にする |
| `--all` | Skill と agent の両方を含める |
| `--no-tui` | インタラクティブ TUI を無効化し、プレーンテキスト出力を使用 |
| `--project, -p` | project レベルの trash を使用（`.skillshare/trash/`） |
| `--global, -g` | グローバルな trash を使用 |
| `--help, -h` | ヘルプを表示 |

## 自動クリーンアップ

期限切れの trash 項目（7 日以上経過したもの）は、`uninstall` または `sync` を実行すると自動的にクリーンアップされます。cron やスケジュールされたタスクは不要です。

## 関連項目

- [uninstall](/docs/reference/commands/uninstall) — Skill を削除（trash へ移動）
- [backup](/docs/reference/commands/backup) — target ディレクトリをバックアップ
- [restore](/docs/reference/commands/restore) — バックアップから target を復元
