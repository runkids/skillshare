---
sidebar_position: 1
---

# doctor

检查环境并诊断你的 skillshare 设置中的问题。

```bash
skillshare doctor
skillshare doctor -p        # Project mode (.skillshare/config.yaml)
skillshare doctor -g        # Force global mode
skillshare doctor --json    # Structured JSON output for CI
```

```text
skillshare doctor

Environment
✓ Config       ~/.config/skillshare/config.yaml
  Config dir   ~/.config/skillshare
  Data         ~/.local/share/skillshare
  State        ~/.local/state/skillshare
✓ Source       ~/.config/skillshare/skills · 43 skills
✓ Agents       ~/.config/skillshare/agents · 2 agents
  Skillignore  not configured
✓ Links        supported
! Git          not initialized (recommended for backup)
✓ Integrity    27/27 skills verified

Targets
✓ claude    skills  merged · merge · 43 shared
✓           agents  synced · merge · 2/2 linked
✓ cursor    skills  merged · merge · 43 shared, 1 local
✓           agents  synced · merge · 2/2 linked
✓ gemini    skills  merged · merge · 43 shared
…
! gemini will see content from: universal
  ~/.agents/skills ← universal
  suggestion: …
…
✗ claude: 1 broken symlink: frontend__css-review
…

Extras
✓ rules     2 files · 2/2 targets OK
✓ commands  1 file · 1/1 targets OK
✓ team      1 file · 4/4 targets OK

Storage
  Backups      last 2026-09-28_12-41-50 · 10m ago
  Trash        1 item, 247 B · oldest under a day

✗ 6 errors, 4 warnings · 1.2s

Next
  skillshare sync  bring the targets up to date
```

## 何时使用

- 某些功能无法工作，但你不知道原因
- 升级 skillshare 或操作系统之后
- 验证所有 targets、git 和 symlinks 是否健康
- 提交 bug report 之前的第一步诊断

## 检查内容

```text
skillshare doctor

Environment
✓ Config       ~/.config/skillshare/config.yaml
  Config dir   ~/.config/skillshare
  Data         ~/.local/share/skillshare
  State        ~/.local/state/skillshare
✓ Source       ~/.config/skillshare/skills · 12 skills
✓ Agents       ~/.config/skillshare/agents · 8 agents
✓ Skillignore  2 patterns, 1 skill ignored
✓ Links        supported
✓ Git          initialized with remote
✓ Integrity    12/12 skills verified

Targets
✓ claude    skills  merged · merge · 8 shared, 2 local
✓           agents  synced · merge · 8/8 linked
✓ codex     skills  merged · merge · 8 shared
✓ cursor    skills  copied · copy · 8 managed
✓           agents  synced · merge · 8/8 linked

Extras
✓ commands  3 files · 1/1 targets OK
✓ rules     4 files · 1/1 targets OK

MCP, hooks and plugins
✓ MCP          all 2 servers OK
✓ Hooks        all 1 hook in sync
  Plugins      none configured

Storage
  Backups      last 2026-01-18_09-00-00 · 3d ago
  Trash        empty

Version
✓ CLI          0.23.5
✓ Skill        0.23.5

✓ All checks passed · 0.4s
```

## 执行的检查

### Environment

| Check | What It Verifies |
|-------|-----------------|
| Config | Config file exists and is valid |
| Source | Source directory exists and is readable |
| Agents | Agents source directory exists (if configured) |
| Skillignore | `.skillignore` (and `.skillignore.local`) active patterns and ignored skill count |
| Source link | skills source 第一层的每个 symlink 或 Windows junction 各一行。[`follow_source_links`](../targets/configuration.md#follow_source_links) 关闭时（默认）：info，`not followed by discovery; its contents are invisible to skillshare. Set follow_source_links: true to follow it`。开启时：info `followed as a directory (follow_source_links)`，或警告 `not followed: <reason>`（目标缺失、指向 source 根目录或其上级目录、与 sync target 重叠） |
| Links | System can create symlinks |
| Git | Repository status and remote configuration |

Source link 检查在 global 与 project mode 都会执行。source 根目录按 discovery 的方式解析，只检查第一层项目。没有这类链接时不会增加输出。每个链接在 `doctor --json` 中也会以 `undeclared_source_links` 检查出现，status 为 `info`；被策略跳过的链接则为 `warning`。

### Targets

每个 target 都会显示 **skills** 和 **agents** 的子项（当配置了 agents 时）：
- Skills：路径、同步模式、同步状态、共享/本地计数
  - “N skills not synced” 只统计 `sync` 会放置的 skill；`sync` 有意跳过的（`standard` 或 `prefixed` 命名下名称无效，或名称冲突）不计入
- Agents：同步模式、已链接数量、drift 检测。在未开启 Developer Mode 的 Windows 上，`merge` 会显示为 `copy`；最新的受管理副本会计为 linked。skillshare 不拥有、但内容相同的本地文件会被保留。在 copy fallback 中，agent 计数会以 `local preserved` 单独显示，例如 `0/1 linked, 1 local preserved`。
- 没有损坏的 symlinks
- 针对意外本地冲突的重复 skill 检查：
  - `merge` 模式：跳过（本地 skills 是预期存在的）
  - `copy` 模式：忽略由 manifest 管理的副本；只警告本地冲突的副本
- 有效的 include/exclude glob 模式
- Naming 与 mode：在 copy 以外的模式下解析为 `prefixed` 的 target 会被标为错误，因为 sync 会跳过它
- 适用时给出的 info 级别的每个 target 兼容性提示（示例 target 优先级：`cursor` → `antigravity` → `copilot` → `opencode`；这些 targets 都不存在时不给提示）

### Path Overlap

Doctor 会在两类重复 skill 风险到达运行时选择器之前将其标记出来：

**`shared_target_paths`** —— 当两个或更多已启用的 targets 解析到同一个主路径时触发。常见原因：
同时启用了 `universal` 和写入 `~/.agents/skills` 的工具（如 `warp`、`witsy`）。

```text
! Shared path ~/.agents/skills ← universal, warp
```

解决方法：禁用其中一个重叠的 target，或用 `skillshare target <name> --path <dir>` 设置一个独立的路径。

如果共用路径的 targets 的 `include` 或 `exclude` 筛选、`mode` 或 `target_naming` 不同（两个 target 都用 `symlink` 模式时，会直接链接整个文件夹，筛选与命名都不影响），每次同步都会按其中一个 target 的设置重写文件夹、抵消另一个 target 的结果，文件夹永远不会稳定，`sync` 也会一直显示同样的待同步变更。Doctor 会标出这种情况，并建议只保留一个 target（如果其中有 `universal` 就保留它）、其余的关闭 skills 同步，而不是移除 target：

```text
! Shared path ~/.agents/skills ← codex, universal (different settings, so they undo each other on every sync)
  suggestion: Keep universal syncing skills to ~/.agents/skills and stop the rest with `skillshare target codex --skills=false`.
```

`sync` 也会列出相同的 targets 和要执行的命令，dashboard 的同步页面则提供按钮，可直接停止同步该 target 的 skills。设置完全相同的共用路径 targets 仍适用上面的解决方法。

**`cross_target_discovery`** —— 当某个已启用 target 的运行时文档说明它也会扫描
另一个已启用 target 写入的目录时触发。例如，沿用旧设置的配置仍让 `codex` 指向旧版的
`~/.codex/skills`，而 `universal` 写入 `~/.agents/skills` —— Codex 同样会读取这个目录。
两者都启用时，Codex 除了自己的内容之外还会看到 universal 的内容。

```text
! codex will see content from: universal
  ~/.agents/skills ← universal
```

解决方法：先移除负责扫描的 target（上例中的 `codex`）。它的运行时本来就会读取共享目录，而且不会影响其他工具。可用 `skillshare target remove codex --dry-run` 预览。如果改为移除写入方（`universal`），其他读取 `~/.agents/skills` 的工具也会看不到这些 skill。只有当扫描端 target 带有被写入方过滤掉的 skill 时才同时保留两者，并接受运行时选择器中出现重复列表。

OpenCode 在 `~/.claude/skills`、`~/.agents/skills` 和自己的文件夹之间，同名的 skill 只保留一个，所以从 source 同步到两边的 skill 只会加载一次。对 `opencode`，这项检查只在它从其他 target 的文件夹加载了 OpenCode 自己文件夹没有的 skill 时才警告，并列出这些 skill。skill 不在 OpenCode 文件夹里，可能是 sync 没放进去（`targets:`、include/exclude，或 `target_naming: standard` 跳过了它），也可能是你自己放在另一个文件夹里的：

```text
! opencode loads 1 skill missing from its own folder, from: claude
  ~/.claude/skills ← claude: claude-only
```

如果写入方（例如 `claude`）使用 symlink mode 而 `opencode` 没有，那个文件夹就是 source 本身，检查无法判断 OpenCode 会从中加载哪些 skill，因此显示一般的警告。

解决方法：把这些 skill 也同步给 `opencode`，或在运行 OpenCode 的环境设置 `OPENCODE_DISABLE_CLAUDE_CODE_SKILLS=1`（或 `OPENCODE_DISABLE_EXTERNAL_SKILLS=1`，连 `.agents/skills` 也跳过）。skillshare 只看得到自己的环境：运行 skillshare 的环境也设置了这个变量时，`doctor`、`sync` 和 dashboard 会把该文件夹当成已跳过，`doctor` 也会注明：

```text
opencode skips ~/.claude/skills: OPENCODE_DISABLE_CLAUDE_CODE_SKILLS is set in this environment
```

如果 OpenCode 是从其他环境启动（例如桌面快捷方式），那边也要设置这个变量。

`shared_target_paths` 只读取已配置的路径。`cross_target_discovery` 也会读内置的 `also_scans` 表，对 `opencode` 还会看每个 target 拿到哪些 skill。

### Version

- CLI 版本
- skillshare skill 版本；有较新的 skill 发布时会给出警告（`skillshare upgrade --skill`）
- 检查是否有可用更新

### Skill Integrity

对于带有文件哈希 metadata 的已安装 skills，doctor 会验证自安装以来是否有文件被篡改：

- 将当前的 SHA-256 哈希与存储的哈希进行比较
- 报告每个 skill 中被修改、缺失和新增的文件
- 本地 skills（不在 `.metadata.json` 中）会被静默跳过——这是预期行为
- 有 metadata 但缺少 `file_hashes` 的已安装 skills 会被标记出名称

```text
! Integrity    5/6 skills verified
!              _team-repo__api-helper: 1 modified, 1 missing
!              1 skill missing file hashes: _old-repo__legacy-skill
```

### Extras

当配置了 extras 时，会验证：
- 每个 extra 的配置有效（mode、`flatten`、`as`），且没有两个 extras 争用同一个文件
- 每个 extra 的 source 目录是否存在
- Target 目录是否可达
- 目录型 target 中的失效符号链接（error）
- 与 source 不一致的文件，判断方式同 `skillshare diff`（warning）。设置了 `flatten` 或 `extension` 的 target 不做比对。

```text
✗ rules     → ~/.claude/rules: broken symlink gone.md
!           → ~/.claude/rules: 1 file out of sync (a.md missing in target)
```

### MCP

执行 [`mcp check`](./mcp.md) 的静态检查：引用的环境变量已设置、`command` 能在 `PATH` 中找到、client 规则接受该 server，以及每个条目都已同步。Doctor 不解析 host，也不启动 server；需要时请运行 `skillshare mcp check` 或 `skillshare mcp check --live`。未配置任何 server 时显示 `info`。

```text
✗ MCP          docs: command no-such-mcp-binary was not found on PATH
!              docs → claude: not synced yet; run skillshare sync mcp
```

### Hooks

预览 `skillshare sync hooks`，不写入任何文件。预览失败是 error。Sync 仍会新增、更新或删除的条目、与原生 hooks 的冲突，以及提示性警告（例如 Agent 未记载的事件名称）是 warning。未配置 hook 时显示 `info`。

```text
! Hooks        bash-log → claude: not synced (add)
```

### Plugins

预览 `skillshare sync plugins`，不拉取任何来源。Doctor 只询问各绑定 Agent 的原生 CLI 当前安装了什么，并且只在存在 plugin package 时才这样做。被阻止的绑定（例如 Agent 的 CLI 未安装）和仍需 sync 的绑定是 warning。检查来源是否有新版本仍由 `skillshare plugin check` 负责。未配置 package 时显示 `info`。

### 其他

- 没有 `SKILL.md` 文件的 skills
- Skill 级别的 `targets:` 字段验证（对未知 target 名称发出警告）
- 最近一次备份的时间戳（global mode）
- Trash 状态（条目数量、总大小、最旧条目的存续时间）
- targets 中损坏的 symlinks。位于目标不可用（例如未挂载的硬盘）的 source link 背后的 target 链接会单独以警告形式报告，`N links behind an unavailable source link, kept until it is back`，且不会建议清理：`sync` 是有意保留它们的，并且在 `doctor --json` 中 `broken_symlinks` 检查为 `warning` 而不是 `error`。

:::note Project Mode
当某个项目存在 `.skillshare/config.yaml` 时，`skillshare doctor` 会自动以 project mode 运行。

在 project mode 下：
- Config/source 检查使用 `.skillshare/config.yaml` 和 `.skillshare/skills`
- Trash 状态使用 `.skillshare/trash`
- Backups 显示 `not used in project mode`
:::

## 常见问题

### "Needs sync"

Target 模式已更改但尚未生效：

```bash
skillshare sync
```

### "Not synced"

Target 已链接的 skills 数量少于 source（例如在安装新 skills 之后）：

```bash
skillshare sync
```

### "Has uncommitted changes"

Tracked repo 有本地更改：

```bash
cd ~/.config/skillshare/skills/_team-repo
git status
# Commit or discard changes
```

### "Broken symlink"

某个 skill 已从 source 移除，但 symlink 仍然存在：

```bash
skillshare sync  # Will prune orphaned symlinks
```

如果该行显示的是 `behind an unavailable source link, kept until it is back`，说明这个 skill 位于某个[被跟随的 source link](../targets/configuration.md#follow_source_links) 背后，而该链接的目标暂时不可用。无需清理：挂载硬盘或恢复 checkout 后运行 `skillshare sync` 即可。

### "Skills without SKILL.md"

Skill 文件夹缺少必需的文件：

```bash
# Add SKILL.md to each skill, or remove the folder
skillshare new my-skill  # Creates proper structure
```

### "Link not supported"

`doctor` 会在系统临时目录（Windows 上是 `%TEMP%`，其他系统是 `$TMPDIR` 或 `/tmp`）中链接一个测试文件夹。在 Windows 上这个链接是 NTFS junction，既不需要管理员权限，也不需要 Developer Mode，所以开启 Developer Mode 无法解决这个错误。消息中的 `junction error:` 一行会显示 Windows 拒绝的原因。请确认临时目录：

1. 位于本地 NTFS 磁盘，而不是 FAT32、exFAT 或网络共享（junction 只能在 NTFS 上使用）
2. 当前账户有写入权限，且没有被杀毒或安全软件拦截

这项检查不会测试文件链接。没有 Developer Mode 时，会链接单个文件的 agents 和 extras 会改为复制；请参阅 [Windows 故障排除](../../troubleshooting/windows.md#file-links-need-windows-developer-mode-copying-instead)。

## 带有问题的示例输出

```
Environment
✓ Config       ~/.config/skillshare/config.yaml
✓ Source       ~/.config/skillshare/skills · 12 skills
✓ Agents       ~/.config/skillshare/agents · 8 agents
✓ Links        supported
! Git          3 uncommitted changes
! Integrity    5/6 skills verified
!              _team-repo__api-helper: 1 modified
! Skills without SKILL.md: test-dir, temp

Targets
✓ claude    skills  merged · merge · 8 shared, 2 local
✓           agents  synced · merge · 8/8 linked
! codex     skills  linked · merge · needs sync
✓ cursor    skills  merged · merge · 6 shared
! claude    1 skill not synced · 2/3 linked
✗ cursor: 2 broken symlinks: old-skill, removed-skill

Storage
  Backups      last 2026-01-18_09-00-00 · 3d ago
  Trash        2 items, 45.2 KB · oldest 3 days

Version
✓ CLI          1.2.0
✓ Skill        0.16.0
  Update       v1.2.0 → v1.3.0 available

✗ 1 error, 5 warnings · 0.6s

Next
  skillshare sync          bring the targets up to date
  brew upgrade skillshare  update to v1.3.0
```

## JSON 输出

在 CI 流水线和自动化中使用 `--json` 获取机器可读的输出：

```bash
skillshare doctor --json
```

```json
{
  "checks": [
    { "name": "source", "status": "pass", "message": "Source: ~/.config/skillshare/skills (12 skills)" },
    { "name": "skillignore", "status": "pass", "message": ".skillignore: 3 patterns, 2 skills ignored", "details": ["test-*", "vendor/", "!important", "---", "test-draft", "vendor/lib"] },
    { "name": "sync_drift", "status": "warning", "message": "claude: 1 skill(s) not synced (7/8 linked)", "details": ["new-skill"] },
    { "name": "shared_target_paths", "status": "warning", "message": "1 shared target path(s) — enabled targets writing to the same directory may produce duplicate skills in runtime pickers", "details": ["~/.agents/skills ← universal, warp"], "suggestions": ["Choose one authoritative target for ~/.agents/skills; preview removing duplicate targets with `skillshare target remove <name> --global --dry-run` (currently: universal, warp)."] },
    { "name": "broken_symlinks", "status": "error", "message": "cursor: 1 broken symlink(s)", "details": ["old-skill"] }
  ],
  "summary": { "total": 14, "pass": 12, "warnings": 1, "errors": 1, "info": 0 },
  "version": { "current": "0.17.4", "latest": "0.18.0", "update_available": true }
}
```

检查状态：`pass`、`warning`、`error`、`info`。`info` 状态用于既非通过也非失败的信息性检查
（例如未找到 `.skillignore`）。Info 检查计入 `total`，但不计入 `pass`、`warnings` 或 `errors`。

某些警告级别的检查（如 `shared_target_paths`、`cross_target_discovery`）还会包含一个可选的
`suggestions` 数组，给出可执行的修复步骤。当没有可建议的内容时，该字段会被省略。

### 退出代码

| Condition | Exit Code |
|-----------|-----------|
| All checks pass (or warnings only) | `0` |
| Any check has `error` status | `1` |

### CI 示例

```bash
# Fail pipeline if doctor finds errors
skillshare doctor --json | jq -e '.summary.errors == 0'

# Extract warnings for notification
skillshare doctor --json | jq '[.checks[] | select(.status == "warning")]'
```

:::tip Web Dashboard
Web dashboard（`skillshare ui`）中的 **Health Check** 页面提供了 `doctor --json` 的可视化版本，带有过滤开关和可展开的详情。
:::

## 另请参阅

- [status](/docs/reference/commands/status) — 快速状态检查
- [sync](/docs/reference/commands/sync) — 修复同步问题
- [upgrade](/docs/reference/commands/upgrade) — 更新 CLI 和 skill
