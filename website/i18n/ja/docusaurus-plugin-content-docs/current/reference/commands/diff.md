---
sidebar_position: 2
---

# diff

Source と Target 間の差分を表示します。

```bash
skillshare diff              # すべての Target（インタラクティブ TUI）
skillshare diff claude       # 特定の Target
skillshare diff agents       # Agent Target のみ
skillshare diff --stat       # ファイル単位の変更
skillshare diff --patch      # 完全な unified diff
```

```text
skillshare diff --no-tui

claude, claude-work, gemini, opencode, universal
  New       remotion-captions

cursor
  Local only  cursor-shortcuts
  New         remotion-captions

Extras
✓ commands  ~/.claude/commands · in sync
✓ rules     ~/.claude/rules · in sync
✓ rules     ~/.cursor/rules · in sync
✓ team      ~/.codex · in sync
✓ team      ~/.claude · in sync
✓ team      ~/.gemini · in sync
✓ team      ~/notes · in sync

! 6 targets: 6 to sync

Next
  skillshare sync     apply the changes
  skillshare collect  copy local-only skills into source
```

## インタラクティブ TUI

TTY 上では、`diff` はインタラクティブな画面を開きます。左側にターゲット、右側に選択したターゲットの差分が表示され、ファイル単位の diff まで確認できます。キーは画面下部に表示されます。プレーンテキストにするには `--no-tui` を使うか、出力をパイプしてください。

## 使うタイミング

- sync する前に、Source と Target の間で正確に何が異なるかを確認する
- Target にのみ存在する Skill（まだ収集されていないローカル専用）を見つける
- symlink に置き換えられるローカルコピーを特定する
- `--stat` でファイル単位の変更、`--patch` で完全なテキスト diff を確認する

## 出力例

```
claude
  Local override  local-copy
  Local only      my-local-skill
  New             another-skill, missing-skill

✓ cursor    in sync

! 2 targets: 1 to sync, 1 in sync

Next
  skillshare sync          apply the changes
  skillshare sync --force  also replace local copies
  skillshare collect       copy local-only skills into source
```

### グループ化された複数 Target の出力

複数の Target が同一の diff 結果を持つ場合、ノイズを減らすために 1 つのブロックにまとめられます。

```
agents, claude
  New       skill-1, skill-2

cursor
  New       skill-1

✓ codex, copilot  in sync
```

（`include`/`exclude` フィルタなどの理由で）結果が異なる Target は個別に表示されます。

## ラベル

| ラベル | 意味 | 動作 |
|-------|---------|--------|
| New | Source にあり、Target にない | `sync` が追加する |
| Restore | Target にあったが削除された | `sync` が復元する |
| Modified | 内容が変更された（copy mode） | `sync` が更新する |
| Local override | symlink ではなくローカルコピー | `sync --force` で置き換え |
| Orphan | マニフェストにあるが Source にない | `sync` が刈り取る |
| Local only | Target のみに存在し、Source にない | `collect` でインポート |

## ファイル単位の詳細

### `--stat`

各 Skill 内でどのファイルが異なるかを表示します。

```bash
skillshare diff --stat
```

```
claude
  Modified  my-skill
            + new-file.md (120 bytes)
            ~ SKILL.md (840 → 912 bytes)
            - old-file.md (64 bytes)
```

### `--patch`

変更されたファイルの完全な unified テキスト diff を表示します。

```bash
skillshare diff --patch
```

```
claude
  Modified  my-skill
            ~ SKILL.md (840 → 912 bytes)
            --- SKILL.md
            - old line
            + new line
```

`--stat` と `--patch` はどちらも `--no-tui`（プレーンテキスト出力）を暗黙的に指定します。

## diff が示す内容

### merge mode の Target

merge mode（デフォルト）を使う Target の場合:
- まだ Target に symlink されていない、Source 内の Skill を一覧表示する
- symlink ではなくローカルコピーとして存在する Skill を表示する
- Target 内のローカル専用の Skill を特定する（Source にない — sync で保持される）

### copy mode の Target

copy mode を使う Target の場合:
- まだ管理下にない（マニフェストにない）、Source 内の Skill を一覧表示する
- チェックサム比較による内容の変更を表示する
- Source にもう存在しない、孤立した管理下のコピーを表示する（sync 時に刈り取られる）
- ローカル専用の Skill（Source になく、管理もされていない）を特定する

### symlink mode の Target

symlink mode を使う Target の場合:
- symlink が正しい Source を指しているかを単純に確認する
- 「in sync」を表示するか、誤った symlink について警告する

## ユースケース

### sync 前

何が変更されるかを確認します。

```bash
skillshare diff
# sync が何を行うかを確認してから:
skillshare sync
```

### ローカル Skill を見つける

Target 内に直接作成した Skill を発見します。

```bash
skillshare diff claude
# 表示: Local only  my-local-skill

skillshare collect claude  # Source にインポート
```

### 変更内容の確認

sync 前に、Skill 内で何が変更されたかを正確に確認します。

```bash
skillshare diff --patch claude   # 完全なテキスト diff
skillshare diff --stat claude    # ファイル単位のサマリー
```

### トラブルシューティング

sync ステータスに問題が表示された場合:

```bash
skillshare status          # 「needs sync」を表示
skillshare diff claude     # 何が異なるかを正確に確認
skillshare sync            # 修正する
```

## Agent Diff {#agent-diff}

`agents` キーワードを使うと、Agent Target のみを diff します。

```bash
skillshare diff agents             # すべての Agent 対応 Target
skillshare diff agents claude      # 特定の Target
skillshare diff agents --json      # JSON 出力
```

Agent diff は、不足している agent（sync が必要）、孤立した symlink（prune が必要）、ローカル専用の agent ファイルを表示します。`agents` パス設定を持つ Target のみが対象です。全リストは [Agents — 対応 Target](/docs/understand/agents#supported-targets) を参照してください。

---

## オプション

| フラグ | 説明 |
|------|-------------|
| `--project, -p` | project mode を使う |
| `--global, -g` | global mode を使う |
| `--stat` | ファイル単位の変更を表示（`--no-tui` を暗黙指定） |
| `--patch` | 完全な unified diff を表示（`--no-tui` を暗黙指定） |
| `--no-tui` | プレーンテキスト出力（インタラクティブ TUI をスキップ） |
| `--json` | JSON として出力（`--no-tui` を暗黙指定） |

## JSON 出力

```bash
skillshare diff --json
```

```json
{
  "targets": [
    {
      "name": "claude",
      "mode": "merge",
      "synced": false,
      "items": [
        {"action": "link", "name": "missing-skill", "reason": "not in target", "is_sync": true},
        {"action": "update", "name": "local-copy", "reason": "local override", "is_sync": true}
      ],
      "include": [],
      "exclude": []
    }
  ],
  "duration": "0.045s"
}
```

## 関連項目

- [sync](/docs/reference/commands/sync) — Target へ同期
- [collect](/docs/reference/commands/collect) — ローカル Skill をインポート
- [status](/docs/reference/commands/status) — 概要をすばやく確認
