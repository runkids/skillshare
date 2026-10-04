---
sidebar_position: 6
---

# follow

在 skills source 中声明一个第一层链接，让 discovery 跟随它。这是设置 [`.skillfollow`](../skillfollow.md) 的一步到位方式，无需手动编辑文件。

```bash
skillshare follow _team-skills --to ~/work/team-skills   # 创建链接并声明
skillshare follow _team-skills                           # 声明已存在的链接
skillshare follow _team-skills --local                   # 只在本机声明
skillshare follow _team-skills -p                        # 项目模式
```

## 何时使用

- 让工作中的仓库留在你平时编辑的位置，同时让 skillshare 发现其中的 skill
- 把 `doctor` 报告为 `undeclared-link` 的链接变成被跟随的 entry
- 添加一个不与队友共享、只属于本机的链接（`--local`）

## 执行内容

1. 指定 `--to <dir>` 时，创建指向 `<dir>` 的链接 `<source>/<name>`：macOS/Linux 上是 symlink，Windows 上是 junction。`<dir>` 必须是 skills source 之外已存在的目录。如果 `<source>/<name>` 已存在，它必须已经是指向同一目录的链接。
2. 不指定 `--to` 时，`<source>/<name>` 必须已经存在，可以是链接或真实目录。真实目录会被接受并报告为 `not-link`；它本来就会被发现。
3. 把 `<name>` 添加到 `.skillfollow`，指定 `--local` 时添加到 `.skillfollow.local`。注释、空行以及现有行的顺序都会保留。已声明的名称不会重复添加。
4. 当 source 位于 Git 工作区内时，把锚定的 ignore 行（例如 `/_team-skills`）添加到 source 的 `.gitignore`，与 `doctor` 要求的那一行相同。指定 `--local` 时还会添加 `/.skillfollow.local`。Git 已经忽略的路径不会再次添加。
5. 打印该 entry 最终的 [state](../skillfollow.md#states) 和原因，让无法跟随的声明立即可见。

`follow` 不会执行 sync。之后请运行 `skillshare sync`。

以 `_` 开头且包含 `.git` 的名称会作为 tracked repository 跟随；其他名称作为 group 跟随。`<name>` 必须是直接子项的名称：不能包含 `/` 或 `\`，不能使用 glob 或否定字符，不能是绝对路径或盘符。

如果链接已被 Git 跟踪，`follow` 不会取消跟踪。它会打印需要你自己运行的 `git -C <source> rm --cached` 命令，可在任意目录运行。`commit` 和 `doctor` 打印的是不带 `-C` 的同一命令，需在 source 中运行。

## 选项

| 标志 | 描述 |
|------|-------------|
| `<name>` | skills source 中的第一层 entry |
| `--to <dir>` | 先创建指向 `<dir>` 的链接（Windows 上是 junction） |
| `--local` | 写入 `.skillfollow.local` 而不是 `.skillfollow`，并在 Git 中忽略它 |
| `--project, -p` | 使用项目的 skills source（`.skillshare/skills/`） |
| `--global, -g` | 使用全局的 skills source |
| `--json` | 以 JSON 输出 |
| `--help, -h` | 显示帮助 |

当既未指定 `-p` 也未指定 `-g` 时会自动检测模式（与其他命令相同）。

## 示例

```bash
# 在 Git source 中创建链接并声明
$ skillshare follow _team-skills --to ~/work/team-skills
✓ _team-skills  linked to /home/me/work/team-skills
✓ _team-skills  added to .skillfollow
✓ .gitignore    added /_team-skills
✓ _team-skills  followed — following directory

Next
  skillshare sync  apply the change

# 已经声明过
$ skillshare follow _team-skills
! _team-skills  already in .skillfollow
✓ _team-skills  followed — following directory

# 无法跟随的声明会被报告，而不是被隐藏
$ skillshare follow out --to ~/.claude
✓ out         linked to /home/me/.claude
✓ out         added to .skillfollow
✓ .gitignore  added /out
! out         target-overlap — target overlaps active skills target /home/me/.claude/skills

Next
  skillshare sync  apply the change

# 链接已被 Git 跟踪
$ skillshare follow _dev
✓ _dev        added to .skillfollow
✓ .gitignore  added /_dev
! _dev        indexed in Git; run git -C '/home/me/.config/skillshare/skills' rm --cached -- '_dev'
✓ _dev        followed — following directory

Next
  skillshare sync  apply the change
```

被拒绝的 entry 仍保持声明状态，并会暂停 target 清理，直到你修复它或运行 [`unfollow`](./unfollow.md)。参见 [状态与恢复](../skillfollow.md#states)。

## JSON 输出

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

指定 `--to` 时会出现 `link_target`，链接已被 Git 跟踪时会出现 `untrack_command`。失败时打印 `{"error": "..."}` 并以状态码 1 退出。参数错误（如未知 flag 或缺少名称）即使使用 `--json` 也打印纯文本。

## 另请参阅

- [unfollow](./unfollow.md) — 停止跟随某个 entry
- [.skillfollow](../skillfollow.md) — 文件格式、状态和安全规则
- [doctor](./doctor.md) — 报告已声明的状态和缺失的 ignore 行
- [sync](./sync.md) — 把变更应用到 target
