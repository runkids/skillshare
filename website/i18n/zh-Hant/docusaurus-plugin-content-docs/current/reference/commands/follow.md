---
sidebar_position: 6
---

# follow

在 skills source 中宣告一個第一層連結，讓 discovery 跟隨它。這是設定 [`.skillfollow`](../skillfollow.md) 的一步到位做法，不必手動編輯檔案。

```bash
skillshare follow _team-skills --to ~/work/team-skills   # 建立連結並宣告
skillshare follow _team-skills                           # 宣告已存在的連結
skillshare follow _team-skills --local                   # 只在這台機器上宣告
skillshare follow _team-skills -p                        # Project mode
```

## When to Use

- 讓工作中的 repository 留在你平常編輯的位置，同時讓 skillshare 找到其中的 skill
- 把 `doctor` 回報為 `undeclared-link` 的連結變成被跟隨的 entry
- 加入一個不與隊友共用、只屬於這台機器的連結（`--local`）

## What It Does

1. 加上 `--to <dir>` 時，建立指向 `<dir>` 的連結 `<source>/<name>`：macOS/Linux 上是 symlink，Windows 上是 junction。`<dir>` 必須是 skills source 之外、已存在的目錄。若 `<source>/<name>` 已經存在，它必須已經是指向同一個目錄的連結。
2. 不加 `--to` 時，`<source>/<name>` 必須已經存在，可以是連結或真實目錄。真實目錄會被接受並回報為 `not-link`；它本來就會被找到。
3. 把 `<name>` 加進 `.skillfollow`，或加上 `--local` 時加進 `.skillfollow.local`。註解、空行以及既有行的順序都會保留。已宣告的名稱不會被重複加入。
4. 當 source 位於 Git 工作目錄內時，把錨定的 ignore 行（例如 `/_team-skills`）加進 source 的 `.gitignore`，與 `doctor` 要求的那一行相同。加上 `--local` 時也會加入 `/.skillfollow.local`。Git 已經忽略的路徑不會再加一次。
5. 印出該 entry 最後的 [state](../skillfollow.md#states) 與原因，讓無法跟隨的宣告立刻被看見。

`follow` 不會執行 sync。之後請執行 `skillshare sync`。

以 `_` 開頭且內含 `.git` 的名稱會被當成 tracked repository 跟隨；其他名稱則當成 group 跟隨。`<name>` 必須是直接子項目的名稱：不可包含 `/` 或 `\`、不可使用 glob 或否定字元、不可是絕對路徑或磁碟代號。

如果連結已經被 Git 追蹤，`follow` 不會取消追蹤。它會印出讓你自己執行的 `git rm --cached` 指令，與 `commit` 和 `doctor` 印出的相同。

## Options

| Flag | Description |
|------|-------------|
| `<name>` | skills source 中的第一層 entry |
| `--to <dir>` | 先建立指向 `<dir>` 的連結（Windows 上是 junction） |
| `--local` | 寫入 `.skillfollow.local` 而非 `.skillfollow`，並在 Git 中忽略它 |
| `--project, -p` | 使用 project 的 skills source（`.skillshare/skills/`） |
| `--global, -g` | 使用 global 的 skills source |
| `--json` | 以 JSON 輸出 |
| `--help, -h` | 顯示說明 |

當未指定 `-p` 或 `-g` 時，模式會自動偵測（與其他指令相同）。

## Examples

```bash
# 在 Git source 中建立連結並宣告
$ skillshare follow _team-skills --to ~/work/team-skills
✓ _team-skills  linked to /home/me/work/team-skills
✓ _team-skills  added to .skillfollow
✓ .gitignore    added /_team-skills
✓ _team-skills  followed — following directory

Next
  skillshare sync  apply the change

# 已經宣告過
$ skillshare follow _team-skills
! _team-skills  already in .skillfollow
✓ _team-skills  followed — following directory

# 無法跟隨的宣告會被回報，而不是被隱藏
$ skillshare follow out --to ~/.claude
✓ out         linked to /home/me/.claude
✓ out         added to .skillfollow
✓ .gitignore  added /out
! out         target-overlap — target overlaps active skills target /home/me/.claude/skills

# 連結已經被 Git 追蹤
$ skillshare follow _dev
✓ _dev        added to .skillfollow
✓ .gitignore  added /_dev
! _dev        indexed in Git; run git rm --cached -- '_dev'
✓ _dev        followed — following directory
```

被拒絕的 entry 仍保持宣告狀態，並會暫停 target 清理，直到你修正它或執行 [`unfollow`](./unfollow.md)。請參閱 [States and recovery](../skillfollow.md#states)。

## JSON Output

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

給了 `--to` 時會出現 `link_target`，連結已被 Git 追蹤時會出現 `untrack_command`。失敗時會印出 `{"error": "..."}` 並以狀態碼 1 結束。

## See Also

- [unfollow](./unfollow.md) — 停止跟隨某個 entry
- [.skillfollow](../skillfollow.md) — 檔案格式、state 與安全規則
- [doctor](./doctor.md) — 回報已宣告的 state 與缺少的 ignore 行
- [sync](./sync.md) — 把變更套用到 target
