---
sidebar_position: 2
---

# diff

显示 Source 与 Targets 之间的差异。

```bash
skillshare diff              # 所有 Target（交互式 TUI）
skillshare diff claude       # 指定 Target
skillshare diff agents       # 仅 Agent Target
skillshare diff --stat       # 文件级别的变更
skillshare diff --patch      # 完整的统一 diff
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

## 交互式 TUI

在 TTY 上，`diff` 会打开交互式界面：左侧是 targets，右侧是所选 target 的差异，可以一直看到文件级别的 diff。按键列在界面底部。使用 `--no-tui` 或将输出 pipe 出去，就会改为输出纯文本。

## 使用场景

- 在 sync 之前准确查看 Source 与某个 Target 之间的差异
- 查找仅存在于某个 Target 中的 Skill（仅本地存在，尚未 collect）
- 找出可以替换为 symlink 的本地副本
- 使用 `--stat` 查看文件级变更，或使用 `--patch` 查看完整文本 diff

## 输出示例

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

### 分组显示多个 Target 的结果

当多个 Target 的 diff 结果完全相同时，会被归并到同一个区块中以减少噪音：

```
agents, claude
  New       skill-1, skill-2

cursor
  New       skill-1

✓ codex, copilot  in sync
```

结果不同的 Target（例如由于 `include`/`exclude` filters）仍会分开显示。

## 标签说明

| Label | Meaning | Action |
|-------|---------|--------|
| New | 存在于 Source，但 Target 中缺失 | `sync` 会添加它 |
| Restore | 曾存在于 Target 中，已被删除 | `sync` 会恢复它 |
| Modified | 内容或 target naming 已更改（copy mode），或 merge sync 会替换为链接且未被编辑的 copy mode 副本 | `sync` 会更新它 |
| Renamed | 受管理的条目仍使用之前 `target_naming` 给出的名称 | `sync` 会重命名它 |
| Local only, skill kept under old name | 本地文件夹占用了当前 `target_naming` 给 skill 的名称，skill 保留在旧的受管理条目，显示为 `name (stays at old-name)` | 重命名或删除该文件夹后 `sync` |
| Local override | 本地副本而非 symlink（包含被编辑过的 copy mode 副本） | 使用 `sync --force` 替换 |
| Orphan | 存在于 manifest 中，但不在 Source 中 | `sync` 会清理它 |
| Local only | 仅存在于 Target 中，不在 Source 中 | 使用 `collect` 导入 |

## 文件级详情

### `--stat`

显示每个 Skill 内哪些文件存在差异：

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

显示已修改文件的完整统一文本 diff：

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

`--stat` 和 `--patch` 都隐含 `--no-tui`（纯文本输出）。

## diff 显示的内容

### Merge Mode 的 Target

对于使用 merge mode（默认）的 Target：
- 列出 Source 中尚未 symlink 到 Target 的 Skill
- 显示以本地副本形式存在（而非 symlink）的 Skill
- 识别 Target 中仅本地存在的 Skill（不在 Source 中 — sync 会保留它们）

### Copy Mode 的 Target

对于使用 copy mode 的 Target：
- 列出 Source 中尚未被管理的 Skill（在 manifest 中缺失）
- 通过 checksum 比对显示内容变更
- 显示不再存在于 Source 中的孤立托管副本（sync 时会被清理）
- 识别仅本地存在的 Skill（既不在 Source 中，也未被管理）

### Symlink Mode 的 Target

对于使用 symlink mode 的 Target：
- 仅检查 symlink 是否指向正确的 Source
- 显示 "in sync"，或对错误的 symlink 发出警告

## 使用案例

### Sync 之前

查看将会发生哪些变更：

```bash
skillshare diff
# 查看 sync 将会执行的操作，然后：
skillshare sync
```

### 查找本地 Skill

发现你直接在某个 Target 中创建的 Skill：

```bash
skillshare diff claude
# 显示：Local only  my-local-skill

skillshare collect claude  # 导入到 Source
```

### 检查变更内容

在 sync 之前准确查看某个 Skill 发生了哪些变更：

```bash
skillshare diff --patch claude   # 完整文本 diff
skillshare diff --stat claude    # 文件级摘要
```

### 故障排查

当 sync 状态显示存在问题时：

```bash
skillshare status          # 显示 "needs sync"
skillshare diff claude     # 准确查看有哪些差异
skillshare sync            # 修复问题
```

## Agent Diff {#agent-diff}

使用 `agents` 关键字仅对 Agent Target 执行 diff：

```bash
skillshare diff agents             # 所有支持 Agent 的 Target
skillshare diff agents claude      # 指定 Target
skillshare diff agents --json      # JSON 输出
```

Agent diff 会显示缺失的 Agent（需要 sync）、孤立的 symlink（需要 prune），以及仅本地存在的 Agent 文件。只有配置了 `agents` 路径的 Target 才会被包含在内。完整列表参见 [Agents — Supported Targets](/docs/understand/agents#supported-targets)。

---

## 选项

| Flag | Description |
|------|-------------|
| `--project, -p` | 使用 Project mode |
| `--global, -g` | 使用 Global mode |
| `--stat` | 显示文件级变更（隐含 `--no-tui`） |
| `--patch` | 显示完整的统一 diff（隐含 `--no-tui`） |
| `--no-tui` | 纯文本输出（跳过交互式 TUI） |
| `--json` | 输出为 JSON（隐含 `--no-tui`） |

## JSON 输出

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
        {"action": "add", "name": "missing-skill", "kind": "skill", "reason": "source only", "is_sync": true},
        {"action": "modify", "name": "local-copy", "kind": "skill", "reason": "local copy (sync --force to replace)", "is_sync": true},
        {"action": "remove", "name": "my-own-skill", "kind": "skill", "reason": "local only", "is_sync": false},
        {"action": "kept", "name": "prototype", "kind": "skill", "reason": "local folder; the skill stays at _emil-design__skills__prototype", "is_sync": false}
      ],
      "include": [],
      "exclude": []
    }
  ],
  "duration": "0.045s"
}
```

当 `sync` 对该 target 无事可做时，`synced` 为 `true`。`"is_sync": false` 的条目（例如只存在于 target 的文件夹）仍会列出，但不会让它变成 `false`。文本输出同样把只有 local-only 文件夹的 target 视为已同步。

`action` 为 `add`、`modify`、`remove` 或 `kept`。`kept` 表示本地文件夹占用了当前 `target_naming` 给 skill 的名称，skill 保留在旧名称下，sync 不会改动该文件夹。

## 另请参阅

- [sync](/docs/reference/commands/sync) — 同步到 Target
- [collect](/docs/reference/commands/collect) — 导入本地 Skill
- [status](/docs/reference/commands/status) — 快速概览
