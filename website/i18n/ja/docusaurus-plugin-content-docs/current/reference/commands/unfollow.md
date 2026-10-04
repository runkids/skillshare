---
sidebar_position: 7
---

# unfollow

skills source の第 1 階層にあるリンクをたどるのをやめます。リンク先には一切触れません。

```bash
skillshare unfollow _team-skills               # 宣言とリンクを削除
skillshare unfollow _team-skills --keep-link   # 宣言だけを削除
skillshare unfollow _team-skills --local       # .skillfollow.local からだけ削除
skillshare unfollow _team-skills -p            # プロジェクトモード
```

## 動作

1. `<name>` を含むすべての宣言ファイル（`.skillfollow` と `.skillfollow.local`。両者は和集合です）から削除し、編集した各ファイルを表示します。コメントと他の entry は保持されます。
2. リンク `<source>/<name>` そのものを削除します。実ディレクトリ（`not-link`）は決して削除しません。
3. リンクを削除した場合に限り、source の `.gitignore` の managed ブロックからそのリンクの ignore 行を削除します。それ以外の場合は行を残し、その旨を表示します。

`--local` を指定すると `.skillfollow.local` だけを編集します。`.skillfollow` がまだその名前を宣言している場合、`unfollow` は entry が引き続きたどられることを伝え、リンクを残します。

書き込みに失敗した場合、`unfollow` は失敗を報告し、すでに編集したファイルを示します。一部だけ完了した unfollow を成功として報告することはありません。

unfollow の後は `skillshare sync` を実行してください。entry の Skill が見つからなくなるので、sync がそれらの managed リンクを削除します。外部パスを直接指すリンクの扱いは [クリーンアップの安全性](../skillfollow.md#cleanup) を参照してください。

## オプション

| フラグ | 説明 |
|------|-------------|
| `<name>` | 宣言済みの第 1 階層の entry |
| `--local` | `.skillfollow.local` からだけ名前を削除 |
| `--keep-link` | リンクとその ignore 行を残す |
| `--project, -p` | プロジェクトの skills source（`.skillshare/skills/`）を使用 |
| `--global, -g` | グローバルの skills source を使用 |
| `--json` | JSON で出力 |
| `--help, -h` | ヘルプを表示 |

## 例

```bash
$ skillshare unfollow _team-skills
✓ _team-skills  removed from .skillfollow
✓ _team-skills  link removed; its target was not touched
✓ .gitignore    removed /_team-skills

Next
  skillshare sync  prune the entry's managed links

# リンクを残す
$ skillshare unfollow _team-skills --keep-link
✓ _team-skills  removed from .skillfollow
  _team-skills  link kept: --keep-link
  .gitignore    kept /_team-skills

Next
  skillshare sync  prune the entry's managed links

# コミット済みのファイルがまだ宣言している
$ skillshare unfollow _team-skills --local
✓ _team-skills  removed from .skillfollow.local
! _team-skills  still declared in .skillfollow; it remains followed

# 実ディレクトリは残る
$ skillshare unfollow realdir
✓ realdir  removed from .skillfollow
  realdir  link kept: not a link; a real directory is never removed
```

## JSON 出力

```bash
skillshare unfollow _team-skills --json
```

```json
{
  "name": "_team-skills",
  "source": "/home/me/.config/skillshare/skills",
  "files_edited": [".skillfollow"],
  "still_declared_in": [],
  "not_declared": false,
  "link": "/home/me/.config/skillshare/skills/_team-skills",
  "link_removed": true,
  "ignore_line": "/_team-skills",
  "ignore_line_removed": true,
  "ignore_line_kept": false
}
```

リンクを残した場合、`link_kept` に理由が入ります。失敗すると `{"error": "..."}` を表示し、ステータス 1 で終了します。不明なフラグや名前の欠落などの引数エラーは、`--json` を付けてもプレーンテキストで表示されます。

## 関連項目

- [follow](./follow.md) — entry を宣言する
- [.skillfollow](../skillfollow.md) — ファイル形式、state、安全規則
- [sync](./sync.md) — entry の managed リンクを削除
