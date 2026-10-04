---
sidebar_position: 7
---

# unfollow

停止跟随 skills source 中的某个第一层链接。链接的目标永远不会被改动。

```bash
skillshare unfollow _team-skills               # 移除声明和链接
skillshare unfollow _team-skills --keep-link   # 只移除声明
skillshare unfollow _team-skills --local       # 只从 .skillfollow.local 移除
skillshare unfollow _team-skills -p            # 项目模式
```

## 执行内容

1. 从每个包含 `<name>` 的声明文件（`.skillfollow` 和 `.skillfollow.local`，两者是并集）中移除它，并列出编辑过的每个文件。注释和其他 entry 都会保留。
2. 移除链接 `<source>/<name>` 本身。真实目录（`not-link`）永远不会被移除。
3. 只有在链接被移除时，才会从 source 的 `.gitignore` managed 区块中移除该链接的 ignore 行。否则保留这一行并加以说明。

指定 `--local` 时只编辑 `.skillfollow.local`。如果 `.skillfollow` 仍然声明该名称，`unfollow` 会说明该 entry 仍被跟随，并保留链接。

如果写入失败，`unfollow` 会报告失败，并列出已经编辑过的文件。它永远不会把只完成一部分的 unfollow 报告为成功。

unfollow 之后请运行 `skillshare sync`：该 entry 的 skill 不再被发现，因此 sync 会清理它们的 managed 链接。直接指向外部路径的链接如何处理，参见 [清理安全](../skillfollow.md#cleanup)。

## 选项

| 标志 | 描述 |
|------|-------------|
| `<name>` | 已声明的第一层 entry |
| `--local` | 只从 `.skillfollow.local` 移除名称 |
| `--keep-link` | 保留链接及其 ignore 行 |
| `--project, -p` | 使用项目的 skills source（`.skillshare/skills/`） |
| `--global, -g` | 使用全局的 skills source |
| `--json` | 以 JSON 输出 |
| `--help, -h` | 显示帮助 |

## 示例

```bash
$ skillshare unfollow _team-skills
✓ _team-skills  removed from .skillfollow
✓ _team-skills  link removed; its target was not touched
✓ .gitignore    removed /_team-skills

Next
  skillshare sync  prune the entry's managed links

# 保留链接
$ skillshare unfollow _team-skills --keep-link
✓ _team-skills  removed from .skillfollow
  _team-skills  link kept: --keep-link
  .gitignore    kept /_team-skills

Next
  skillshare sync  prune the entry's managed links

# 已提交的文件仍然声明它
$ skillshare unfollow _team-skills --local
✓ _team-skills  removed from .skillfollow.local
! _team-skills  still declared in .skillfollow; it remains followed

# 真实目录会被保留
$ skillshare unfollow realdir
✓ realdir  removed from .skillfollow
  realdir  link kept: not a link; a real directory is never removed
```

## JSON 输出

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

链接被保留时，`link_kept` 给出原因。失败时打印 `{"error": "..."}` 并以状态码 1 退出。参数错误（如未知 flag 或缺少名称）即使使用 `--json` 也打印纯文本。

## 另请参阅

- [follow](./follow.md) — 声明一个 entry
- [.skillfollow](../skillfollow.md) — 文件格式、状态和安全规则
- [sync](./sync.md) — 清理该 entry 的 managed 链接
