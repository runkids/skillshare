---
sidebar_position: 3
---

# Sync Modes

skillshare 如何将 source 链接到 targets。

:::tip 什么时候需要关心这个？
如果你想要按 skill 逐一建立 symlink，并保留 target 中的本地 skill，选择 merge 模式（默认）。如果你需要真实文件而不是 symlink（可移植性、CI，或个人偏好），选择 copy 模式。如果你想要整个目录被链接，且不需要 target 专属的本地 skill，选择 symlink 模式。
:::

## 概览

| 模式 | 行为 | 使用场景 |
|------|----------|----------|
| `merge` | 每个 skill 单独建立 symlink | **默认。** 保留本地 skill。 |
| `copy` | 每个 skill 以真实文件形式复制 | 可移植性、CI/沙盒环境，或你更倾向于使用真实文件而非 symlink。 |
| `symlink` | 整个目录是一个 symlink | 处处都是完全一致的副本。 |

## 决策矩阵（中立视角）

用这张表根据你的实际约束来选择，而不是根据 target 的品牌名称：

| 决策维度 | `merge` | `copy` | `symlink` |
|---|---|---|---|
| 跨不同 AI CLI 的兼容性 | 中 | 高 | 低–中 |
| 编辑一次即时反映 | 高 | 低（需要 `sync`） | 高 |
| 磁盘占用 | 低 | 高 | 低 |
| 防止从 target 意外删除的安全性 | 高 | 高 | 低 |
| 操作简单性 | 中 | 中 | 高 |
| 按 target 过滤（`include`/`exclude`） | 支持 | 支持 | 不支持 |

如果你不确定，先从 `merge` 开始，再按需为特定 target 切换到 `copy`。

---

## Merge 模式（默认）

每个 skill 单独建立 symlink。Target 中的本地 skill 会被保留。

```
Source                          Target (claude)
─────────────────────────────────────────────────────────────
skills/                         ~/.claude/skills/
├── my-skill/        ────────►  ├── my-skill/ → (symlink)
├── another/         ────────►  ├── another/  → (symlink)
└── ...                         ├── local-only/  (preserved)
                                └── .skillshare-manifest.json
```

**优点：**
- 保留 target 专属的 skill（不参与同步）
- 混合使用已安装和本地的 skill
- 精细控制
- 按 target 的 include/exclude 过滤
- 基于 manifest 的孤儿清理（uninstall 后能安全地移除非 symlink 残留）

:::info 项目模式下的相对 symlink
在项目模式（`-p`）下，symlink 会以**相对路径**形式创建（例如 `../../.skillshare/skills/my-skill`），而不是绝对路径。这使得项目可移植 — 移动或重命名目录后，symlink 仍然有效。在 global 模式下则使用绝对路径，因为 source 和 targets 位于不同位置。
:::

**何时使用：**
- 你希望某些 skill 只出现在特定的 AI CLI 中
- 你想在同步之前先试用本地 skill
- 你想要一个 source，但每个 target 拥有不同的 skill 子集

### Merge 模式下的过滤策略

`include` 和 `exclude` 会按以下顺序针对每个 target 进行评估：
1. `include` 保留匹配的名称
2. `exclude` 从保留下来的集合中移除

快速选择：
- 当 target 只应获得一小部分子集时，使用 `include`
- 当 target 应获得几乎全部内容时，使用 `exclude`
- 当你需要一个较宽的子集并带有明确排除项时，使用 `include + exclude`

规则变更时的行为：
- 之前已同步的、来自 source 的 symlink 条目，一旦被过滤掉，会在下次 `sync` 时被移除
- Target 中已有的本地非 symlink 文件夹会被保留

完整示例请参见 [Target Configuration](/docs/reference/targets/configuration#include--exclude-target-filters)。

---

## Copy 模式

每个 skill 以真实文件的形式复制到 target 目录。`.skillshare-manifest.json` 文件会记录哪些 skill 受管理及其校验和，因此本地 skill 会被保留。

```
Source                          Target (cursor)
─────────────────────────────────────────────────────────────
skills/                         ~/.cursor/skills/
├── my-skill/        ────copy►  ├── my-skill/    (real files)
├── another/         ────copy►  ├── another/     (real files)
└── ...                         ├── local-only/  (preserved)
                                └── .skillshare-manifest.json
```

### 为什么需要 copy 模式？

即使你的 AI CLI 能正确处理 symlink，copy 模式仍然有其价值：

- **防御性设计** — 并非每个 AI CLI 都保证支持 symlink，尤其是在 Windows 上，symlink 行为会因平台和权限级别而异
- **沙盒环境** — 严格的 CI 流水线、容器和气隙（air-gapped）环境可能不会跨文件系统边界跟随 symlink
- **用户偏好** — 有些用户和团队出于透明性和可移植性的考虑，单纯更喜欢真实文件而非 symlink

**优点：**
- 到处都能用 — 不需要 AI CLI 或操作系统支持 symlink
- 保留本地 skill（与 merge 模式相同）
- 按 target 的 include/exclude 过滤
- 基于校验和的跳过机制：未变更的 skill 不会被重新复制

**何时使用：**
- 你的 AI CLI 报告"skill not found"，或无法读取 symlink 形式的 skill
- 你想把 skill 打包进项目仓库 — 项目模式下的 copy 模式让团队可以把真实的 skill 文件提交到 git，队友无需安装 skillshare
- 你需要不依赖中心 source 也能独立工作的自包含 skill 目录（可移植环境、CI 流水线、气隙环境）
- 你想要与 merge 模式相同的过滤行为，但使用真实文件
- 常见的 `copy` 首选对象：`cursor`、`antigravity`、`copilot`、`opencode`

### 更新是如何工作的

每次运行 `skillshare sync` 时，都会将每个 source skill 的校验和与 manifest 中存储的值进行比较：

- **校验和相同** → 跳过该 skill（速度快）
- **校验和不同** → 用新版本覆盖该 skill
- **`--force`** → 无论校验和如何，都覆盖所有受管理的 skill

### Manifest 生命周期

Merge 和 copy 两种模式都会写入 `.skillshare-manifest.json` 来跟踪受管理的 skill：

- **Merge 模式**：以 `"symlink"` 值记录 skill 名称 — 用于在 uninstall 后安全地清理孤儿真实目录（例如 copy 模式留下的残留）
- **Copy 模式**：以 SHA-256 校验和记录 skill 名称 — 用于增量同步和孤儿检测
- 切换到 symlink 模式时会自动移除
- 如果被手动删除，下次 `sync` 会重新构建它

---

## Symlink 模式

整个 target 目录是指向 source 的单一 symlink。

```
Source                          Target (claude)
─────────────────────────────────────────────────────────────
skills/              ────────►  ~/.claude/skills → (symlink to source)
├── my-skill/
├── another/
└── ...
```

**优点：**
- 所有 target 完全一致
- 管理更简单
- 没有孤儿 symlink

**何时使用：**
- 你希望所有 AI CLI 拥有完全相同的 skill
- 你不需要 target 专属的 skill

**警告：** 在 symlink 模式下，通过 target 删除会连带删除 source！
```bash
rm -rf ~/.claude/skills/my-skill  # ❌ Deletes from SOURCE
skillshare target remove claude   # ✅ Safe way to unlink
```

---

## 更改模式

### 按 target

```bash
# Switch to copy mode (for AI CLIs that can't read symlinks)
skillshare target cursor --mode copy
skillshare sync

# Switch to symlink mode
skillshare target claude --mode symlink
skillshare sync

# Switch back to merge mode
skillshare target claude --mode merge
skillshare sync
```

### 按 target 覆盖（推荐）

你不需要为每个 target 都设置同一个全局模式。一种常见的模式是：

```yaml
mode: merge
targets:
  claude:
    path: ~/.claude/skills
    # inherits merge
  cursor:
    path: ~/.cursor/skills
    mode: copy
  codex:
    path: ~/.codex/skills
    mode: symlink
```

当某个 target 需要以兼容性优先（`copy`）而其他 target 保持即时反映（`merge`/`symlink`）时，使用按 target 的覆盖设置。

### 默认模式

在配置中为新 target 设置：

```yaml
# ~/.config/skillshare/config.yaml
mode: merge  # or symlink or copy

targets:
  claude:
    path: ~/.claude/skills
    # inherits default mode

  cursor:
    path: ~/.cursor/skills
    mode: copy  # real files for Cursor

  codex:
    path: ~/.codex/skills
    mode: symlink  # override default
```

---

## Target 命名 {#target-naming}

控制在使用 merge 或 copy 模式时，target 中的 skill 目录如何命名。

| 命名方式 | 行为 |
|--------|------|
| `flat`（默认） | 嵌套 skill 用 `__` 分隔符展平：`frontend/dev` → `frontend__dev` |
| `standard` | 使用 SKILL.md 的 `name` 字段：`frontend/dev` → `dev` |
| `prefixed` | 仅限 copy 模式。在 `standard` 基础上加上 tracked repo 名称作为前缀：`_mattpocock-skills/skills/prototype` → `mattpocock-skills-prototype` |

可全局设置，也可按 target 设置：

```yaml
target_naming: standard    # global default
targets:
  claude:
    skills:
      target_naming: flat  # per-target override
```

或通过 CLI：

```bash
skillshare target claude --target-naming standard
skillshare sync
```

**Standard 模式**遵循 [Agent Skills specification](https://agentskills.io/specification)，该规范要求 SKILL.md 的 `name` 字段与父目录名一致。有效名称最多 64 个字符，只能包含小写字母（任何文字系统）、数字和单个连字符，且不能以 `-` 开头或结尾；不允许使用下划线。名称无效或存在名称冲突的 skill 会收到警告并被跳过。

**Prefixed 模式**适用于包含同名 skill 的 tracked repo。在 `standard` 下，来自不同 repo 的两个 `prototype` skill 会冲突并都被跳过；在 `prefixed` 下，tracked repo 内的每个 skill 都会变成 `<repo>-<name>`，因此两者都能到达工具。`<repo>` 是 tracked repo 文件夹名去掉开头的 `_`、转为小写，并把字母和数字（任何文字系统）以外的字符替换为 `-` 的结果。带前缀的名称既用作文件夹名，也会写入复制出的 `SKILL.md` 的 `name:`；source 不会被修改。

- 名称已经以 repo 名开头时不会重复加前缀：`_bmad/skills/bmad-ux` 仍是 `bmad-ux`。
- 不在 tracked repo 内的 skill 保持原名。
- source skill 必须先通过 `standard` 检查。带前缀的名称超过 64 个字符时会收到警告并被跳过，仍然冲突的名称与 `standard` 一样被跳过。tracked skill 不能在 `SKILL.md` 中改名，因此 sync 会建议重命名另一个 skill，或用 `--name` 重新 track 该 repo。
- 想要更短的前缀，可以用较短的名称 track repo：`skillshare install <repo> --track --name mp` 会得到 `mp-prototype`。
- 指向同级 skill 的相对链接（`../other-skill/`）不会被改写，与 copy 模式下的 `flat` 相同。
- 该名称就是工具中显示的名称；在 Claude Code 中它是 slash command，例如 `/mattpocock-skills-prototype`。

`prefixed` 需要 copy 模式，因为 merge 链接指向 source，而 source 中的 `name:` 不能被修改。在 merge 或 symlink 模式下解析为 `prefixed` 的 target 会校验失败，并被 sync 跳过。新 target 若会在 copy 以外的模式下继承 `prefixed`（项目 target 默认为 merge），`target add` 会让它使用 copy 模式。`target add` 这样做时会提示；`status` 和 `doctor` 会在 sync 之前标出在 copy 以外的模式下解析为 `prefixed` 的 target，`target list` 对 `targets:` 中的 target 也会如此，修正方法是在该 target 上设置 `mode: copy`；若该 target 由 `projects:` 条目展开而来，则设置 `projects.<root>.skills.mode: copy`。

```yaml
targets:
  universal:
    skills:
      mode: copy
      target_naming: prefixed
```

**迁移**：在 `flat`、`standard` 和 `prefixed` 之间切换，会就地重命名现有受管理的条目。在 copy 模式下，manifest 会记录每个副本是由哪种命名方式生成的，因此即使 source 没有变化，被重命名的副本也会重新复制，使其 `name:` 与新的命名方式一致。如果某个本地 skill 已经占用了新名称，旧的受管理条目会被保留。

**Symlink 模式**：`flat` 和 `standard` 会被忽略 — 整个目录会按原样被链接。`prefixed` 则如上所述会校验失败。

---

## 模式比较

| 方面 | Merge | Copy | Symlink |
|--------|-------|------|---------|
| 保留本地 skill | ✅ 是 | ✅ 是 | ❌ 否 |
| 兼容 symlink | ✅ 是 | ❌ 真实文件 | ✅ 是 |
| 所有 target 完全一致 | ❌ 可以不同 | ❌ 可以不同 | ✅ 是 |
| 按 target 的 include/exclude | ✅ 是 | ✅ 是 | ❌ 被忽略 |
| 需要孤儿清理 | ✅ 是 | ✅ 是 | ❌ 否 |
| 删除安全性 | ✅ 安全 | ✅ 安全 | ⚠️ 需谨慎 |
| 磁盘占用 | 低（symlink） | 较高（副本） | 低（symlink） |

---

## 孤儿清理

在 merge 和 copy 两种模式下，`sync` 都会自动清理孤儿：

- **指向已删除 source skill 的 symlink** 总是会被移除
- **真实目录** 如果出现在 `.skillshare-manifest.json` 中（此前由 skillshare 管理），会被移除
- **不在 manifest 中的未知目录** 会被保留，并给出警告（假定为用户自行创建）

这意味着在 `uninstall` + `sync` 之后，即使是非 symlink 的残留（例如之前 `copy` 模式留下的目录）也会被安全清理。

```
$ skillshare sync
✓ claude    5 linked · 2 local · 1 pruned
✓ cursor    3 copied · 2 up to date · 1 pruned
```

:::info Agent 遵循相同的模式
所有三种模式（merge、copy、symlink）同样适用于 agent 同步。Agent 的孤儿清理、按 target 的 include/exclude 过滤，以及模式转换，其行为与 skill 完全相同 — 唯一的区别是 agent 是单一的 `.md` 文件而不是目录。支持 agent 的 target（Claude、Cursor、Augment、OpenCode）在其 `agents:` 子键上遵循相同的 `mode` 设置。详情请参见 [Agents](./agents.md)。
:::

---

## Extras 同步模式

Extras（非 skill 资源，如 rules、commands、prompts）同样使用 merge 和 copy 模式。每个 extras target 可以指定自己的模式：

```yaml
extras:
  - name: rules
    targets:
      - path: ~/.claude/rules          # merge (default): per-file symlinks
      - path: ~/.cursor/rules
        mode: copy                     # copy: real file copies
```

其行为与 skill 同步模式相同 — merge 建立按文件的 symlink，copy 建立真实文件副本。

:::note 未开启 Developer Mode 的 Windows
merge 模式链接的是单个文件，而 Windows 只有在开启 Developer Mode 时才允许这样做。没有开启时，merge 模式下的 agents 和 extras 会改为复制，这些副本会像链接一样被更新和清理。Skills 是文件夹，所以无论如何都会（用 junction）链接。参见 [Windows 疑难解答](/docs/troubleshooting/windows#file-links-need-windows-developer-mode-copying-instead)。

skillshare 不拥有、但内容相同的本地文件会被保留。在 copy fallback 中，agent 计数会以 `local preserved` 单独显示，例如 `0/1 linked, 1 local preserved`。
:::

---

## 另请参阅

- [sync](/docs/reference/commands/sync) — 运行 sync 以应用模式变更
- [target](/docs/reference/commands/target) — 更改某个 target 的同步模式
- [Source & Targets](./source-and-targets.md) — 核心架构
- [Configuration](/docs/reference/targets/configuration) — 按 target 的设置
