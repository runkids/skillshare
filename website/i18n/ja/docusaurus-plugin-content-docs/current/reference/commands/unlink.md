---
sidebar_position: 3
---

# unlink

[`skillshare link`](./link.md) で作成したリンク、または Skill の source にある第1階層のリンクを削除します。trash に移動するのはリンクだけで、リンク先のフォルダは残ります。

```bash
skillshare unlink _dev-skills              # リンクを削除
skillshare unlink _team -p                 # project の source のリンクを削除
skillshare unlink -- -local                # - で始まる名前の前でオプション解析を終了
```

## 使うタイミング

- リンクしたチェックアウトを、そのファイルに触れずに使うのをやめる
- 別の名前でリンクし直す前にリンクを削除する

```text
$ skillshare unlink _dev-skills
✓ Unlinked _dev-skills → trash, kept 7 days; its target is untouched

Next
  skillshare sync                       remove its skills from your targets
  skillshare trash restore _dev-skills  undo
```

## 実行内容

`unlink` はリンクのエントリのみを trash に移動します。リンク先フォルダーの直下に `SKILL.md` がある場合も同じです。ダッシュボードの **Unlink** も同じ動作をします。trash にはそれが `link → <target>` として一覧表示され、`skillshare trash restore <name>` はリンク（またはジャンクション）を再作成し、リンク先のコピーにはしません。その後 `skillshare sync` を実行すると、その Skill が target から削除されます。 Global mode では、`unlink` はそのリンク名に対応する tracked リポジトリの記録と監査の承認記録も削除します。リンク先の内容は変更しません。

第1階層のリンクではない名前は拒否されるため、通常の Skill やフォルダを削除することはありません。

```text
$ skillshare unlink my-skill
✗ cannot unlink my-skill: my-skill is not a link
```

| 理由 | 原因 |
|--------|-------|
| `<name> not found in source` | その名前のものが source の直下にない |
| `<name> is not a link` | エントリが通常のディレクトリまたはファイルである。[`uninstall`](./uninstall.md) を使用してください |
| `"<name>" is not a first-level name` | 名前にパス区切り文字が含まれている、または `.` / `..` である |

## オプション

| フラグ | 説明 |
|------|-------------|
| `--project, -p` | カレントディレクトリの project レベルの config を使用 |
| `--global, -g` | global config を使用（`~/.config/skillshare`） |
| `--` | オプションの終わり。`-` で始まる名前を指定できる |
| `--help, -h` | ヘルプを表示 |

## 関連項目

- [link](./link.md) — リンクを作成
- [trash](./trash.md) — 削除したリンクを復元
- [Configuration — `follow_source_links`](../targets/configuration.md#follow_source_links) — discovery の動作とリンク経由の書き込み
