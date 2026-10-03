---
sidebar_position: 2
---

# 第一次 Sync

一次完整的首次配置流程，按顺序进行。从安装到跑通 Sync 大约五分钟。另有两种变体 —— 在另一台机器上恢复，以及在无人值守的无头机器上运行 —— 记录在本页末尾。

## 前置条件

- macOS、Linux 或 Windows
- 至少安装了一个 AI CLI（Claude Code、Pi、Codex 等）

## 1. 安装 CLI

**Homebrew（macOS / Linux）：**
```bash
brew install skillshare
```

:::note
Homebrew 上的版本可能会滞后几天。想用最新版，请使用安装脚本。
:::

**安装脚本（macOS / Linux）：**
```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

脚本默认安装到 `~/.local/bin`，正常安装与更新不需要 `sudo`。只有安装器显示 PATH 设置提示时，才需要按提示设置后再运行 `skillshare`。可把提示的命令加入 shell 配置文件（例如 `~/.zshrc` 或 `~/.bashrc`），让之后打开的终端也能使用。可用 `INSTALL_DIR` 指定其他安装位置。

现有安装不会被移动或删除。如果 PATH 优先选到旧可执行文件，安装器会显示它的位置；请把新安装目录放在 PATH 前面。自定义的受保护目录仍可能需要 `sudo`。

如需整台电脑共用的安装：

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | INSTALL_DIR=/usr/local/bin sh
```

**Windows（PowerShell）：**
```powershell
irm https://raw.githubusercontent.com/runkids/skillshare/main/install.ps1 | iex
```

:::tip 之后如何更新
`skillshare upgrade` 会自动识别你的安装方式（Homebrew、脚本、手动），并就地更新 CLI。
:::

## 2. 初始化

```bash
skillshare init
```

<p>
  <img src="/img/init-demo.png" alt="skillshare init：已回答的问题、摘要与首次同步" width="720" />
</p>

`init` 会问几个简短的问题、显示摘要，在你确认之前不会写入任何东西：

1. **如何开始** —— 在这台机器上新建设置，或[连接你已有的 skillshare repo](/docs/how-to/sharing/cross-machine-sync#second-machine-setup)。
2. **Targets** —— 检测到的 AI CLI 全部勾选。按空格键可以取消其中一个。
3. **导入** —— 这些工具里已有的 skills 会复制到 source。
4. **Git** —— 默认开启，之后就能把 skills 推送到 repo。
5. **Remote** —— 粘贴你的 skills repo URL（例如 `git@github.com:you/skills.git`），或按 Enter 跳过，之后再添加。

摘要中还会列出 source 目录（`~/.config/skillshare/skills/`）、sync mode，以及内置 skill（会为 AI CLI 添加 `/skillshare` 命令）。选择 **Change settings** 可以修改，按 <kbd>Esc</kbd> 则取消。确认后，`init` 会询问是否立即同步。

### 选择 Sync 模式

`init` 支持 `--mode <merge|copy|symlink>`，用来设定新添加 Target 的默认模式：

- `merge`（默认）—— 逐个 skill 的 symlink；Target 中已有的本地 skills 会被保留
- `symlink` —— 整个 Target 目录变成一个 symlink（最快，会替换掉该目录）
- `copy` —— 真实文件；改动在下一次 `sync` 时生效

之后可以通过 `skillshare target <name> --mode <mode>` 针对单个 Target 覆盖设置。

## 3. 安装一个 skill

```bash
skillshare install anthropics/skills/skills/pdf
```

每次安装都会执行一次安全审计。发现 critical 级别的问题会阻止安装；只有在你已审阅并愿意承担风险时，才使用 `--force`。

## 4. Sync

```bash
skillshare sync
```

现在每个已配置的 Target 都指向你的 Source 了。

## 5. 验证

```bash
skillshare status
```

```text
$ skillshare status

Source
  skills    ~/.config/skillshare/skills  43 skills
  agents    ~/.config/skillshare/agents  2 agents

Targets                     skills       agents
  claude  ~/.claude/skills  ✓ 43 linked  ✓ 2
  cursor  ~/.cursor/skills  ✓ 43 linked  ✓ 2
  gemini  ~/.gemini/skills  ✓ 43 linked  —
  all use merge
```

输出会显示 Source 路径和每个 Target。已同步的 Target 会显示 `✓` 和链接的 skill 数量，其中也包含你刚刚安装的 skill。

仪表板（`skillshare ui`）也能一眼看到同样的状态：

![首次同步后的 Dashboard：一个 source 连接到每个 target，全部已同步](/img/web-dashboard-demo.png)

---

## 刚才发生了什么

1. **`init`** 创建了 `~/.config/skillshare/config.yaml` 和 `~/.config/skillshare/skills/`，自动检测了你的 AI CLI，并且 —— 如果你提供了 remote —— 会从中 clone 已有的 skills。
2. **`install`** 把 skill clone 到 Source 目录并执行了一次安全审计。`.metadata.json` 记录上游 URL 与 commit，以便 `skillshare update` 之后能拉取更新。
3. **`sync`** 按每个 Target 配置的模式执行。例如在 `merge` 模式下：
   ```
   ~/.claude/skills/pdf → ~/.config/skillshare/skills/pdf  (symlink)
   ```

在 `merge` 和 `symlink` 模式下，对 Source 的编辑会立即出现在每个 Target 中。在 `copy` 模式下则在下一次 `sync` 时生效。`merge` 和 `copy` 会保留 Target 中已有的本地 skills；`skillshare backup` 会在破坏性操作前创建快照，`skillshare restore <target>` 可以回滚。

只想给某一个 Target 换个模式？按 Target 覆盖即可：

```bash
skillshare target <name> --mode copy
skillshare sync
```

完整的选择矩阵见 [Sync 模式](/docs/understand/sync-modes)。

---

## 变体：在另一台机器上恢复

你已经在别处用了 skillshare，并且在 GitHub 上有一个个人 skills repo。在新笔记本、devcontainer 或 VM 上，四条命令即可恢复全部内容 —— 没有交互提示，没有选项，重复执行也是幂等的：

```bash
# 1. Install the CLI (Homebrew or curl|sh — same as Step 1 above)
brew install skillshare

# 2. Clone your skills repo and add detected targets
skillshare init \
  --remote git@github.com:<you>/skills.git \
  --all-targets \
  --no-skill

# 3. Re-install tracked dependencies
#    (the _-prefixed dirs are gitignored, so they aren't in the cloned repo)
skillshare install https://github.com/<your-company>/skills --track --force

# 4. Sync
skillshare sync
```

`--no-skill` 会跳过内置 skill；如果这台机器上也想要它，之后用 `skillshare upgrade --skill` 添加即可。

---

## 变体：无头配置（无 TTY）

对于 CI 任务、devcontainer 的 post-create hook，或云 VM 的 provisioner，每一个交互提示都有对应的非交互 flag：

```bash
skillshare init \
  --source ~/.config/skillshare/skills \
  --remote https://github.com/<you>/skills \
  --targets codex \
  --mode merge \
  --no-copy \
  --no-skill

skillshare install https://github.com/<your-company>/skills --track --force
skillshare sync
```

| Flag | 作用 |
|---|---|
| `--source <path>` | 跳过 Source 路径提示 |
| `--remote <url>` | 跳过 remote 提示；若 remote 有内容则 clone |
| `--targets <name>` | 只添加列出的 Targets（用 `--all-targets` 添加所有检测到的） |
| `--mode merge` | 新 Target 的默认 Sync 模式 |
| `--no-copy` | 跳过「是否复制 Target 中已有 skills？」提示；以空目录开始 |
| `--no-skill` | 跳过内置 skill 提示 |

`--targets`、`--all-targets` 和 `--no-targets` 互斥 —— 只能选一个。

---

## 接下来

- [创建你自己的 skill](/docs/how-to/daily-tasks/creating-skills)
- [跨机器同步](/docs/how-to/sharing/cross-machine-sync)
- [组织级 skills](/docs/how-to/sharing/organization-sharing)
- [Agents](/docs/understand/agents) —— 与 skills 一同管理单文件 `.md` agents
- [Sync 模式](/docs/understand/sync-modes) —— 选择矩阵与取舍
