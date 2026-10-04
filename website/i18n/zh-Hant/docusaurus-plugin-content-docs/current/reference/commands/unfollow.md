---
sidebar_position: 7
---

# unfollow

停止跟隨 skills source 中的某個第一層連結。連結的目標絕不會被動到。

```bash
skillshare unfollow _team-skills               # 移除宣告與連結
skillshare unfollow _team-skills --keep-link   # 只移除宣告
skillshare unfollow _team-skills --local       # 只從 .skillfollow.local 移除
skillshare unfollow _team-skills -p            # Project mode
```

## What It Does

1. 從每個含有 `<name>` 的宣告檔（`.skillfollow` 與 `.skillfollow.local`，兩者是聯集）移除它，並列出編輯過的每個檔案。註解與其他 entry 都會保留。
2. 移除連結 `<source>/<name>` 本身。真實目錄（`not-link`）絕不會被移除。
3. 只有在連結被移除時，才會從 source 的 `.gitignore` managed 區塊移除該連結的 ignore 行。否則會保留這一行並加以說明。

加上 `--local` 時只會編輯 `.skillfollow.local`。如果 `.skillfollow` 仍然宣告這個名稱，`unfollow` 會說明該 entry 仍被跟隨，並保留連結。

如果寫入失敗，`unfollow` 會回報失敗，並列出已經編輯過的檔案。它絕不會把只完成一部分的 unfollow 當成成功回報。

unfollow 之後請執行 `skillshare sync`：該 entry 的 skill 不再被找到，因此 sync 會清掉它們的 managed 連結。直接指向外部路徑的連結如何處理，請參閱 [Cleanup safety](../skillfollow.md#cleanup)。

## Options

| Flag | Description |
|------|-------------|
| `<name>` | 已宣告的第一層 entry |
| `--local` | 只從 `.skillfollow.local` 移除名稱 |
| `--keep-link` | 保留連結與它的 ignore 行 |
| `--project, -p` | 使用 project 的 skills source（`.skillshare/skills/`） |
| `--global, -g` | 使用 global 的 skills source |
| `--json` | 以 JSON 輸出 |
| `--help, -h` | 顯示說明 |

## Examples

```bash
$ skillshare unfollow _team-skills
✓ _team-skills  removed from .skillfollow
✓ _team-skills  link removed; its target was not touched
✓ .gitignore    removed /_team-skills

Next
  skillshare sync  prune the entry's managed links

# 保留連結
$ skillshare unfollow _team-skills --keep-link
✓ _team-skills  removed from .skillfollow
  _team-skills  link kept: --keep-link
  .gitignore    kept /_team-skills

Next
  skillshare sync  prune the entry's managed links

# 已 commit 的檔案仍然宣告它
$ skillshare unfollow _team-skills --local
✓ _team-skills  removed from .skillfollow.local
! _team-skills  still declared in .skillfollow; it remains followed

# 真實目錄會被保留
$ skillshare unfollow realdir
✓ realdir  removed from .skillfollow
  realdir  link kept: not a link; a real directory is never removed
```

## JSON Output

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

連結被保留時，`link_kept` 會說明原因。失敗時會印出 `{"error": "..."}` 並以狀態碼 1 結束。參數錯誤（例如未知的 flag 或缺少名稱）即使加上 `--json` 也會印出純文字。

## See Also

- [follow](./follow.md) — 宣告一個 entry
- [.skillfollow](../skillfollow.md) — 檔案格式、state 與安全規則
- [sync](./sync.md) — 清掉該 entry 的 managed 連結
