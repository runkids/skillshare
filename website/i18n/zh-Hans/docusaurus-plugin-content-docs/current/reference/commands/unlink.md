---
sidebar_position: 3
---

# unlink

移除由 [`skillshare link`](./link.md) 创建的链接，或 skills Source 中任何第一层的链接。只有链接本身会移到 trash；它指向的文件夹会保留。

```bash
skillshare unlink _dev-skills              # 移除链接
skillshare unlink _team -p                 # 移除项目 Source 中的链接
skillshare unlink -- -local                # 结束选项解析，允许名称以 - 开头
```

## 何时使用

- 不再使用某个已链接的 checkout，但不动其中的文件
- 先移除链接，再以另一个名称重新链接

```text
$ skillshare unlink _dev-skills
✓ Unlinked _dev-skills → trash, kept 7 days; its target is untouched

Next
  skillshare sync                       remove its skills from your targets
  skillshare trash restore _dev-skills  undo
```

## 会发生什么

`unlink` 只把链接条目移到 trash，链接文件夹本身包含 `SKILL.md` 时也适用。dashboard 的 **Unlink** 执行相同操作。trash 会把它列为 `link → <target>`，`skillshare trash restore <name>` 会重新创建链接（或 junction），绝不会变成目标的副本。之后执行 `skillshare sync`，就会把它的 skills 从 targets 中移除。 在 global mode 中，`unlink` 也会移除为该链接名称保留的追踪仓库记录及其审计接受记录；链接目标不受影响。

不是第一层链接的名称会被拒绝，所以它绝不会移除普通的 skill 或文件夹：

```text
$ skillshare unlink my-skill
✗ cannot unlink my-skill: my-skill is not a link
```

| 原因 | 成因 |
|--------|-------|
| `<name> not found in source` | Source 下没有这个名称的条目 |
| `<name> is not a link` | 该条目是普通目录或文件；请改用 [`uninstall`](./uninstall.md) |
| `"<name>" is not a first-level name` | 名称含有路径分隔符，或是 `.` / `..` |

## 选项

| Flag | Description |
|------|-------------|
| `--project, -p` | Use project-level config in current directory |
| `--global, -g` | Use global config (`~/.config/skillshare`) |
| `--` | End options; allow a name starting with `-` |
| `--help, -h` | Show help |

## 另请参阅

- [link](./link.md) — 创建链接
- [trash](./trash.md) — 还原已移除的链接
- [Configuration — `follow_source_links`](../targets/configuration.md#follow_source_links) — 发现行为与经由链接的写入
