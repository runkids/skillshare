---
sidebar_position: 6
---

# follow

skills source の第 1 階層にあるリンクを宣言し、discovery がそれをたどるようにします。ファイルを手で編集せずに [`.skillfollow`](../skillfollow.md) を設定するための、1 ステップの方法です。

```bash
skillshare follow _team-skills --to ~/work/team-skills   # リンクを作成して宣言
skillshare follow _team-skills                           # 既存のリンクを宣言
skillshare follow _team-skills --local                   # このマシンだけで宣言
skillshare follow _team-skills -p                        # プロジェクトモード
```

## 使うタイミング

- 作業中のリポジトリを普段編集している場所に置いたまま、skillshare にその Skill を見つけさせたいとき
- `doctor` が `undeclared-link` と報告したリンクを、たどる対象の entry にしたいとき
- チームと共有しない、このマシンだけのリンクを追加したいとき（`--local`）

## 動作

1. `--to <dir>` を指定すると、`<dir>` を指すリンク `<source>/<name>` を作成します。macOS/Linux では symlink、Windows では junction です。`<dir>` は skills source の外にある既存のディレクトリでなければなりません。`<source>/<name>` がすでに存在する場合は、同じディレクトリを指すリンクである必要があります。
2. `--to` を指定しない場合、`<source>/<name>` はリンクまたは実ディレクトリとしてすでに存在している必要があります。実ディレクトリは受け付けられ、`not-link` と報告されます。いずれにしても見つかります。
3. `<name>` を `.skillfollow` に、`--local` を指定した場合は `.skillfollow.local` に追加します。コメント、空行、既存行の順序は保持されます。すでに宣言されている名前はそのままです。
4. source が Git の作業ツリー内にある場合、アンカー付きの ignore 行（例: `/_team-skills`）を source の `.gitignore` に追加します。`doctor` が求める行と同じです。`--local` を指定すると `/.skillfollow.local` も追加します。Git がすでに無視しているパスの行は追加しません。
5. entry の最終的な [state](../skillfollow.md#states) と理由を表示するので、たどれない宣言にすぐ気づけます。

`follow` は sync を実行しません。その後 `skillshare sync` を実行してください。

`_` で始まり `.git` を含む名前は tracked repository として、それ以外の名前は group としてたどられます。`<name>` は直下の子の名前でなければなりません。`/` や `\`、glob や否定の文字、絶対パスやドライブ名は使えません。

リンクがすでに Git に追跡されている場合、`follow` は追跡を解除しません。自分で実行する `git -C <source> rm --cached` コマンドを表示します。どのディレクトリからでも実行できます。`commit` と `doctor` は `-C` なしの同じコマンドを表示するので、source で実行します。

## オプション

| フラグ | 説明 |
|------|-------------|
| `<name>` | skills source の第 1 階層の entry |
| `--to <dir>` | 先に `<dir>` へのリンクを作成（Windows では junction） |
| `--local` | `.skillfollow` ではなく `.skillfollow.local` に書き込み、Git で無視する |
| `--project, -p` | プロジェクトの skills source（`.skillshare/skills/`）を使用 |
| `--global, -g` | グローバルの skills source を使用 |
| `--json` | JSON で出力 |
| `--help, -h` | ヘルプを表示 |

`-p` も `-g` も指定しない場合、他のコマンドと同様にモードが自動検出されます。

## 例

```bash
# Git の source でリンクを作成して宣言
$ skillshare follow _team-skills --to ~/work/team-skills
✓ _team-skills  linked to /home/me/work/team-skills
✓ _team-skills  added to .skillfollow
✓ .gitignore    added /_team-skills
✓ _team-skills  followed — following directory

Next
  skillshare sync  apply the change

# すでに宣言済み
$ skillshare follow _team-skills
! _team-skills  already in .skillfollow
✓ _team-skills  followed — following directory

# たどれない宣言は隠さずに報告される
$ skillshare follow out --to ~/.claude
✓ out         linked to /home/me/.claude
✓ out         added to .skillfollow
✓ .gitignore  added /out
! out         target-overlap — target overlaps active skills target /home/me/.claude/skills

# リンクがすでに Git に追跡されている
$ skillshare follow _dev
✓ _dev        added to .skillfollow
✓ .gitignore  added /_dev
! _dev        indexed in Git; run git -C '/home/me/.config/skillshare/skills' rm --cached -- '_dev'
✓ _dev        followed — following directory
```

拒否された entry は宣言されたまま残り、修正するか [`unfollow`](./unfollow.md) を実行するまで target のクリーンアップを一時停止します。[状態と復旧](../skillfollow.md#states) を参照してください。

## JSON 出力

```bash
skillshare follow _team-skills --json
```

```json
{
  "name": "_team-skills",
  "source": "/home/me/.config/skillshare/skills",
  "file": ".skillfollow",
  "added": true,
  "link": "/home/me/.config/skillshare/skills/_team-skills",
  "link_created": false,
  "ignore_file": "/home/me/.config/skillshare/skills/.gitignore",
  "ignore_lines_added": ["/_team-skills"],
  "state": "followed",
  "reason": "following directory",
  "resolved_target": "/home/me/work/team-skills"
}
```

`--to` を指定した場合は `link_target` が、リンクが Git に追跡されている場合は `untrack_command` が含まれます。失敗すると `{"error": "..."}` を表示し、ステータス 1 で終了します。

## 関連項目

- [unfollow](./unfollow.md) — entry をたどるのをやめる
- [.skillfollow](../skillfollow.md) — ファイル形式、state、安全規則
- [doctor](./doctor.md) — 宣言された state と不足している ignore 行を報告
- [sync](./sync.md) — 変更を target に反映
