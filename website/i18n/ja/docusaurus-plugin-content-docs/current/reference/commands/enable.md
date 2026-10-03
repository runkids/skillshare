---
sidebar_position: 5
---

# enable / disable

Skill を削除せずに一時的に有効化・無効化します。

```bash
skillshare disable draft-*          # パターンで無効化
skillshare enable draft-*           # 再有効化
skillshare disable "frontend/**"    # フォルダ内のすべての Skill を無効化
skillshare disable my-skill -p      # Project mode
```

## 使うタイミング

- アンインストールせずに、Skill を sync から一時的に隠したい
- ドラフトや実験的な Skill をすべての Target でミュートしたい
- `list` の TUI から `t` キーで Skill の有効/無効を切り替えたい

## 仕組み

`disable` は `.skillignore` にパターンを追加し、`enable` はそれを削除します。無効化された Skill は Source ディレクトリには残りますが、`sync` と `collect` の対象からは除外されます。

```mermaid
flowchart LR
    DIS["skillshare disable my-skill"]
    IGN[".skillignore += my-skill"]
    SYNC["sync skips my-skill"]
    DIS --> IGN --> SYNC
```

```mermaid
flowchart LR
    EN["skillshare enable my-skill"]
    IGN[".skillignore -= my-skill"]
    SYNC["sync includes my-skill"]
    EN --> IGN --> SYNC
```

:::tip
有効化・無効化の後は、`skillshare sync` を実行して Target に変更を適用してください。
:::

## オプション

| フラグ | 説明 |
|------|-------------|
| `<name\|pattern>` | 1 つ以上の Skill 名または glob パターン（例: `draft-*`、`frontend/**`） |
| `--project, -p` | Project の `.skillignore`（`.skillshare/skills/.skillignore`）を使用 |
| `--global, -g` | グローバルの `.skillignore`（`~/.config/skillshare/.skillignore`）を使用 |
| `--dry-run, -n` | 書き込まずにプレビュー |
| `--help, -h` | ヘルプを表示 |

`-p` も `-g` も指定しない場合、他のコマンドと同様にモードが自動検出されます。

## 例

```bash
# 単一の Skill を無効化
$ skillshare disable my-draft
✓ my-draft  added to .skillignore

Next
  skillshare sync  apply the change

# glob パターンで無効化
$ skillshare disable "experimental-*"
✓ experimental-*  added to .skillignore

Next
  skillshare sync  apply the change

# 再有効化
$ skillshare enable my-draft
✓ my-draft  removed from .skillignore

Next
  skillshare sync  apply the change

# 書き込まずにプレビュー
$ skillshare disable my-skill --dry-run
  my-skill  would be added to ~/.config/skillshare/skills/.skillignore

Dry run — nothing was written

# すでに無効化されている場合
$ skillshare disable my-draft
! my-draft  already disabled
```

## フォルダ全体を無効化する

`disable`/`enable` は `.skillignore` と同じ glob 構文を受け付けるため、別途「グループ」フラグは存在しません。パターンをフォルダに向けるだけで、その中のすべての Skill が一括で切り替わります。

```bash
# frontend/ 配下のすべての Skill を無効化（深さを問わず）
$ skillshare disable "frontend/**"
✓ frontend/**  added to .skillignore

Next
  skillshare sync  apply the change

# フォルダ全体を再有効化
$ skillshare enable "frontend/**"
✓ frontend/**  removed from .skillignore

Next
  skillshare sync  apply the change
```

:::tip パターンをクォートする
シェルが `*` を skillshare に渡す前に展開してしまわないよう、フォルダのパターンは必ずクォート（`"frontend/**"`）で囲んでください。
:::

`frontend/**` は `.skillignore` に 1 行だけ書き込み、後でそのフォルダに追加したものもすべて引き続きカバーします。**同じ**パターンで `enable` を実行すると、その行が削除されます。個々の Skill を無効化したい場合は、名前で列挙してください（`skillshare disable a b c`）。glob の完全なリファレンス（`*`、`**`、`?`、`[abc]`、`!negation`、アンカー付き `/`、ディレクトリ限定の `pattern/`）については [.skillignore pattern syntax](/docs/reference/filtering#skillignore) を参照してください。

## TUI での切り替え

`skillshare list` の TUI でも Skill の有効/無効を切り替えられます。変更は即座に `.skillignore` に書き込まれます。

## .skillignore はどこにある?

| モード | パス |
|------|------|
| グローバル | `~/.config/skillshare/skills/.skillignore` |
| Project | `.skillshare/skills/.skillignore` |

このファイルは最初の `disable` 実行時に自動的に作成されます。

## Agent 対応

`--kind agent` を使うと Agent の有効化・無効化ができます。この場合、`.skillignore` の代わりに `.agentignore` に書き込まれます。

```bash
skillshare disable --kind agent draft-reviewer     # Agent を無効化
skillshare enable --kind agent draft-reviewer      # 再有効化
skillshare disable --kind agent "experimental-*"   # パターンで無効化
```

| モード | `.agentignore` のパス |
|------|---------------------|
| グローバル | `~/.config/skillshare/agents/.agentignore` |
| Project | `.skillshare/agents/.agentignore` |

Agent 管理の背景については [Agents](/docs/understand/agents) を参照してください。

## 関連項目

- [list](./list.md) — 無効化された Skill の表示と `t` キーでの切り替え
- [Filtering Skills](/docs/how-to/daily-tasks/filtering-skills) — すべてのフィルタリング層
- [.skillignore](/docs/reference/filtering#skillignore) — パターン構文
- [sync](./sync.md) — 有効化・無効化後に変更を適用
- [Agents](/docs/understand/agents) — Agent の概念
