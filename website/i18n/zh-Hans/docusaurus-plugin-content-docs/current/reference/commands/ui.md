---
sidebar_position: 1
---

# ui

启动 web dashboard 以可视化管理 skills。

```bash
skillshare ui                  # 在前台运行
skillshare ui start            # 启动（或复用）一个后台 server
skillshare ui stop             # 停止后台 server
```

在默认浏览器中打开 `http://127.0.0.1:19420`。

## 模式

| 模式 | 行为 |
|------|------|
| `skillshare ui`（默认） | 在前台运行 UI server；`Ctrl+C` 停止它 |
| `skillshare ui start` | 把 UI server 作为后台进程启动，并把控制权交回 shell。再次运行 `start` 会复用现有进程（如果它仍然健康） |
| `skillshare ui stop` | 停止由 `skillshare ui start` 启动的后台 UI server |

## 何时使用

- 通过可视化 web 界面管理 skills、targets 和 sync
- 无需记忆 CLI flag 即可浏览和安装 skills
- 用可视化的 findings 报告运行安全审计
- 与不熟悉 CLI 的团队成员分享 dashboard 视图

## Flags

| Flag | 默认值 | 说明 |
|------|---------|-------------|
| `-p`, `--project` | | 以 project mode 运行（使用 `.skillshare/`） |
| `-g`, `--global` | | 以 global mode 运行（使用 `~/.config/skillshare/`） |
| `--port <port>` | `19420` | HTTP server 端口 |
| `--host <host>` | `127.0.0.1` | 绑定地址（Docker 请使用 `0.0.0.0`） |
| `-b`, `--base-path <path>` | | 反向代理用的子路径（例如 `/skillshare`） |
| `--no-open` | `false` | 不自动打开浏览器 |
| `--app` | `false` | 在可用时以桌面风格的 Chromium app 窗口打开 dashboard（仅 `start` 模式） |
| `--clear-cache` | | 前台形式下：清除缓存的 UI 资源并退出。与 `start` 结合：先清除缓存，再在后台启动 |

:::tip 自动检测
如果当前目录存在 `.skillshare/config.yaml`，dashboard 会自动以 project mode 启动。使用 `-g` 强制 global mode。
:::

## 示例

```bash
# 默认：在 localhost:19420 打开浏览器（前台）
skillshare ui

# Project mode（管理 .skillshare/ 中的 skills）
skillshare ui -p

# 自定义端口
skillshare ui --port 8080

# Docker / 远程访问
skillshare ui --host 0.0.0.0 --no-open

# 在后台启动并返回 shell
skillshare ui start

# 以无边框的桌面风格 app 窗口启动
skillshare ui start --app

# 停止后台 server（使用记住的 host/port）
skillshare ui stop

# 清除缓存的 UI 资源，然后在后台重新启动
skillshare ui start --clear-cache
```

## Dashboard 页面

侧边栏按任务分组页面：同步、你管理的内容、内容去向何处，以及日常维护。名称下方的一行显示模式和其目录，例如 `Global · ~/.config/skillshare`。

部分页面在需要关注时会在侧边栏显示计数。这些计数在 dashboard 标签页打开时每 15 秒刷新一次：

- **Sync**：一次 sync 会应用的变更
- **Git Sync**：未提交的文件；如果工作树干净，则显示尚未推送的提交数
- **Audit**：最近一次扫描中被阻止的 skills 和 agents（运行一次扫描后才会显示）

| 页面 | 说明 |
|------|------|
| **Dashboard** | skills、agents、extras、MCP servers、hooks、plugins 和 targets 的计数，以及需要关注的项目 |
| **Sync** | 在写入之前，按 Target 预览每一处变更。选择要包含的部分（Skills、Agents、Extras、MCP）。在 Target 内部编辑过的文件会被保留，除非开启了 **Force**。只存在于某个 Target 中的项目可以从这里收集回 Source。每次 sync 会先备份 Target 目录。某个 Target 失败时，其他 Target 仍会 sync：页面会在其他警告上方列出每个失败的 Target 及其部分（Skills、Agents、Extras 或 Config）和错误，常见原因（symlink 指向别处、权限不足、只读文件系统、文件或文件夹不存在、Target 设置无效）还会附上一句易懂的说明；若 Skills 的 symlink 指向别处，会提供 **开启 Force**；变更列表中也会标出该 Target。即使所有 Target 都失败，也会以同样方式列出。**上次同步** 卡片会列出最近一次 sync 中失败的 Target |
| **Git Sync** | 提交并推送 source 仓库，推送尚未上 remote 的提交，并拉取。打开页面时会先从 remote fetch，所以 **Pull** 会显示 remote 有多少个新提交。Pull 会同步该仓库 scope 所涵盖的内容（`skills`、`agents`、`extras` 或 `root`），与 [`pull`](/docs/reference/commands/pull) 相同。**Sync both ways** 会提交本地更改、拉取并合并、同步 targets，然后推送，与 [`push --pull`](/docs/reference/commands/push#push-and-pull-together) 相同；遇到冲突时会在推送前停止。当 remote 因为有更新的提交而拒绝 push 时，错误提示会提供 **Pull**。当首次 pull 无法与 remote 合并时，它会提供一个强制 pull 选项，用 remote 分支替换本地文件 |
| **Hubs** | 从 Skill 页面进入。列出内置 hub、已保存的 hubs 与你自己的 Hub（**我的**）；选择一个即可过滤并安装其中的 skills。**添加或创建 Hub** 可添加已有的 hub、创建新的 Hub，或导入 `skillshare-hub.json`。自己的 Hub 通过 **编辑** 修改；**分享** 会下载索引并生成 `hub add` 命令。参见 [`hub`](/docs/reference/commands/hub) |
| **Skills** / **Agents** | 已安装的项目、**Updates** 标签页，以及 **Trash** 标签页。Skills 还有一个 **Analyze** 标签页，估算每个 skill 为某个 target 的上下文增加了多少 token。**Install** 可从 GitHub 搜索，或从 URL 或路径安装。**+ New Skill** 打开创建向导。列表和卡片视图可以按 **Folder**（tracked repo、`frontend/react` 这类文件夹，或 **Root**）过滤，也可以按 **Folder** 分组，Root 排在最前，其余文件夹按名称 A→Z 排序。**tree** 视图左侧显示 source 文件夹，右侧显示详情：点击文件夹或 skill 进行选择，Cmd/Ctrl 点击加入选择，Shift 点击选择范围，双击 skill 打开它。右侧可以用一个开关启用或停用所有选中的项目、设置它们的 targets（包括 tracked repo 及其子文件夹），并列出每个 skill 各自的开关；tracked repo 还提供 **Update repo** 和 **Uninstall repo**。带有 `disable-model-invocation: true` 的 skill 会在列表、卡片和详情页上带有 **manual only** 标签，这与 [`list`](/docs/reference/commands/list) 用 `m` 切换的状态相同。在 skill 编辑器中，**Add field** 会描述每个 frontmatter 字段的作用。**Sync skills** / **Sync agents** 会先预览，然后只将该类型 sync 到每个 target；在更新、卸载或 collect 之后，点击 **Sync Now** 也会打开同一个对话框 |
| **Extras** | **文件夹**：与 skills 一起同步的 rules、commands 和其他目录。**AGENTS.md**：在 global mode 下是共享 `AGENTS.md` 以及哪些 target 在使用它们；在项目中是项目的 `./AGENTS.md` 以及每个 target 是否会读取它。参见[让多个工具共用一份 AGENTS.md](../../how-to/daily-tasks/sharing-instructions.md)  **Memory**: [记忆共享教程](../../how-to/daily-tasks/sharing-memory.md)（英文截图）。搜索、预览、编辑、删除笔记、INDEX 链接、冲突时保留草稿、Backup Files 历史与恢复，以及 **Connect to agents** → **Review changes** → **Apply changes**；配置状态不代表已读取 |
| **MCP** | 每个 server 一行，并以 chip 显示它同步到的 Agents；行上的计数按钮会打开它们的开关。**添加服务器** 接受 URL、命令、粘贴的片段或文件；**从目标导入** 会读取某个已安装 Agent 现有的配置。每个 server 的菜单都有 **查看各 Agent 会写入的配置**，会列出来源和每个 Agent 的文件，并显示你选中的那一个，包括尚未保存的编辑。冲突会提供 **从该 Agent 导入** 或 **使用来源覆盖**。server 对话框的 **工具** 区块用于设置[工具策略](./mcp.md#tool-policy)：**加载工具**，它会用对话框当前的设置（无论是否已保存）启动 server 一次并列出工具供勾选；用于 `*` 通配符的 **排除规则** 行；该区块会列出每个未应用部分策略的所选 Agent，设置了策略时该行会显示一个标签。对于 Pi server，对话框还提供 [Pi 内置 MCP](./mcp.md#pi) 的设置：工具暴露方式和其他 Pi 设置，后者以 JSON 形式填写 [`piOptions`](./mcp.md#pi-options)，说明位于信息提示中。**默认值** 用于编辑 `mcp.targets`。Sync 框中的 **Sync MCP** 会列出待写入的更改，并只写入 MCP 配置文件，同时为每个文件保留一份备份；其下方的 [**检查**](./mcp.md#check-servers-before-an-agent-starts-them) 用于检查 server，**备份与还原** 用于预览并恢复这些备份。 |
| **Plugins** | 每个 plugin 一行，以及它的 Agents 开关。展开一行还会列出该来源支持的其他 Agents；勾选一个会预览安装效果。行菜单可以同步、更新、移除，或打开 **View files**，即 Skillshare 审阅过的本地副本的只读浏览器。参见 [Manage plugins across tools](/docs/how-to/daily-tasks/sharing-plugins) |
| **Targets** | 带状态的 Target 列表。**添加目标** 还提供 **另一个账号**：你已在使用的某个 Agent 的第二个配置文件夹，并会预览它的写入位置。每个 Target 的页面可编辑 include/exclude filter，并把本地专属的 skills 收集回 Source。列表也会显示每个 Agent 拿到的 MCP server 数量。有 MCP 配置文件的 Target 会多一个 **MCP** 标签页，每个 server 一行；点一下就会保存，**Sync all targets** 会写入所有 Target 的 MCP 配置文件。每个 Target 还有一个以其指示文件命名的标签页（**CLAUDE.md**、**GEMINI.md**、**AGENTS.md** 等），显示读取顺序、编辑该文件，并可将其转换为 `AGENTS.md` |
| **Projects** | 仅限 global mode。global 配置会 sync 进的项目文件夹，来自 [`projects`](/docs/reference/targets/configuration#projects) 和 [`mcp.projects`](./mcp.md#projects-in-the-dashboard)。**添加项目** 需要填写文件夹、它的 Target 和要 sync 的内容。每个项目都有带 filter 的 **Skills** 和 **Agents** 标签页，可以预览并查看会写入的文件夹，还有一个 **MCP** 标签页，用于在该文件夹中关闭 global server 或为它添加自己的 server，并提供 **检查** 来检查这些 server。**Sync project** 会先预览，然后只 sync 该项目的 skills、agents 和 MCP。已经指向某个项目文件夹的 Target 可以被转换 |
| **Audit** | 对 skills 和 agents 的安全扫描，按严重程度列出 findings。**Rules** 标签页按类别浏览每一条规则：可以关闭某一条、更改其严重程度、把某个严重程度应用到整个类别、选择扫描 profile（`default`、`strict`、`permissive`），或打开自定义 `audit-rules.yaml` 的编辑器 |
| **Settings** | 带标签页：**General**（source 路径、sync 模式、外观）、**Backup**（target 文件夹快照、`AGENTS.md` 等文件的早期版本，以及 MCP 配置备份；参见 [`backup`](./backup.md#dashboard)）、**Log**（操作历史）、**Health**（与 [`doctor`](/docs/reference/commands/doctor) 相同的检查）、**Extensions**（同步时的文件转换）、**Files**（`config.yaml`、`.skillignore` 和 `.agentignore` 的直接编辑器） |

更改列表旁的 **放弃更改** 会在确认后，将所选 Git 范围内所有已跟踪的文件和暂存区恢复至最后一次提交，并删除未跟踪的文件和文件夹。被 Git 忽略的文件、嵌套 Git 仓库，以及 `root` 范围的 `config.yaml` 都会保留。此操作不会更改提交记录，也不会推送到远程仓库，且无法撤销。**试运行** 只会预览，不会更改文件。仓库必须已有第一次提交，才能放弃更改。

在 **Updates** 标签页中，进度条会显示更新进度，正在更新的行也会标记出来。被阻止或失败的更新会显示在单独的区域。

旧链接如 `/collect`、`/install`、`/search`、`/trash`、`/analyze`、`/backup`、`/log` 和 `/doctor` 会重定向到新位置。

**Files** 标签页在编辑器旁边放了一个面板。对于 `config.yaml`，它会显示光标所在字段的作用、文件结构以及尚未保存的变更；对于 ignore 文件，它会列出当前这些 pattern 隐藏了什么。`Cmd+S` / `Ctrl+S` 保存。**Audit -> Rules -> Edit YAML** 下的规则编辑器有同样的面板，外加一个 **Test** 标签页，可以用你粘贴的行来测试某条规则的正则表达式。

### 主题系统

Dashboard 支持两种视觉风格和三种颜色模式，可通过侧边栏的 **Theme** 按钮切换：

| 设置 | 选项 | 默认值 |
|---------|---------|---------|
| **Style** | `Clean`（专业风格）、`Playful`（粗描边、硬阴影、手写标题） | Playful |
| **Mode** | `Light`、`Dark`、`System`（跟随操作系统偏好） | Light |

主题偏好会持久化在 localStorage 中，跨会话保留。

### Project Mode 差异

在 project mode（`-p`）下运行时，dashboard 会有以下调整：

- **Sidebar** 在名称下方显示 `Project · <project path>`
- **Git Sync 页面** 被隐藏（project skills 使用项目自己的 git）
- **Sync** 只备份 agent target 目录，与 `skillshare sync -p` 相同
- **Backup 标签页** 在 Settings 中被隐藏（改用版本控制）
- **Tracked Repos 区块** 从 Dashboard 中隐藏（不适用）
- **Settings -> Files** 显示的是 `.skillshare/config.yaml` 和项目级的 `.skillignore`，而非全局版本
- **Available targets** 列出的是 project 级 target（例如相对于项目根目录的 `.claude/skills/`）
- **Targets** 统计并切换 project 的 MCP server，写入 project 自己的文件（例如 `.mcp.json`）。Claude Code、OpenCode、Kilo Code 和 Pi 的 **MCP** 标签页还会列出在这个 project 关闭全局 server 的开关
- **Install** 会自动协调 project config 中的 `skills:` 条目
- **Extras -> AGENTS.md** 编辑的是项目的 `./AGENTS.md` 而不是共享文件，并为只读取自己文件的 target 提供一个小修正

## UI 预览

<div style={{display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))', gap: '1rem'}}>
  <img src="/img/web-install-demo.png" alt="安装流程" />
  <img src="/img/web-dashboard-demo.png" alt="仪表板概览" />
  <img src="/img/web-skills-demo.png" alt="技能浏览页面" />
  <img src="/img/web-skill-detail-demo.png" alt="技能详细信息页面" />
  <img src="/img/web-sync-demo.png" alt="同步控件" />
  <img src="/img/web-search-skills-demo.png" alt="GitHub 搜索界面" />
  <img src="/img/web-projects-demo.png" alt="列出项目文件夹的 Projects 页面" />
</div>

## REST API

web dashboard 在 `/api/` 下暴露一个 REST API。所有端点都返回 JSON。

| Method | Path | 说明 |
|--------|------|-------------|
| GET | `/api/overview` | skill/target 计数、模式、版本、config 目录（`configDir`） |
| GET | `/api/skills` | 列出所有 skills 及其元数据 |
| GET | `/api/skills/{name}` | skill 详情 + SKILL.md 内容 |
| GET | `/api/skills/templates` | 获取用于创建 skill 的可用 pattern 和分类 |
| POST | `/api/skills` | 创建一个新 skill（name、pattern、category、scaffoldDirs） |
| DELETE | `/api/skills/{name}` | 卸载一个 skill |
| GET | `/api/targets` | 列出 targets 及其状态、include/exclude filter，以及每个 target 的预期计数 |
| POST | `/api/targets` | 添加一个 target |
| DELETE | `/api/targets/{name}` | 移除一个 target |
| POST | `/api/sync` | 运行 sync（支持 `dryRun`、`force`、`kind`，以及 `project`：一个已声明的项目根目录，用于将 sync 限定在该项目的 targets）。除非设置了 `dryRun`，否则会先备份 targets |
| POST | `/api/git/commit` | 从 source 仓库创建一个本地 git commit，但不推送 |
| POST | `/api/git/discard` | 放弃配置的 Git 范围内尚未提交的更改（仅限全局模式，仓库必须已有第一次提交）。支持 `dryRun`；保留被 Git 忽略的文件、嵌套 Git 仓库，以及 `root` 范围的 `config.yaml` |
| GET | `/api/git/status` | source 仓库状态，包括尚未推送的提交（`ahead`），以及截至上次 fetch 尚未拉取的 upstream 提交（`behind`）。不会执行 fetch |
| POST | `/api/push` | 提交任何变更，然后推送。首次推送时会设置 upstream。当 remote 有本仓库没有的提交时，会以 `409` 和错误码 `push_rejected` 失败；先 pull，再重新 push |
| POST | `/api/pull` | 拉取，然后同步该仓库 scope 所涵盖的内容。分歧的历史会被合并；`.metadata.json` 的冲突会自动解决，其他冲突会失败并撤销合并。当首次 pull 无法合并时，会以错误码 `merge_failed` 失败；用 `force: true` 重试可用 remote 分支替换本地文件。`alwaysSync: true` 时即使没有拉取到新内容也会同步 targets。remote 没有任何分支时返回 `400 remote_empty` |
| GET | `/api/diff` | source 与 targets 之间的差异 |
| GET | `/api/search?q=` | 在 GitHub 上搜索 skills |
| POST | `/api/install` | 从来源安装一个 skill |
| GET | `/api/audit` | 扫描所有 skills 的安全威胁 |
| GET | `/api/audit/rules` | 获取自定义 audit rules YAML |
| PUT | `/api/audit/rules` | 保存自定义 audit rules（会校验正则表达式） |
| POST | `/api/audit/rules` | 创建初始的 audit-rules.yaml |
| GET | `/api/audit/rules/compiled` | 合并内置规则和自定义规则后的每一条规则，以及当前激活的 profile |
| POST | `/api/audit/rules/toggle` | 启用、禁用或重新评级某条规则或整个 pattern |
| POST | `/api/audit/rules/reset` | 删除自定义规则并恢复内置默认值 |
| PATCH | `/api/audit/policy` | 设置 `blockThreshold`、`profile`，或两者一起 |
| GET | `/api/log` | 列出日志条目，支持可选的过滤条件 |
| GET | `/api/config` | 以 YAML 形式获取 config |
| PUT | `/api/config` | 更新 config YAML |
| GET | `/api/skillignore` | 获取 `.skillignore` 内容 + 忽略统计 |
| PUT | `/api/skillignore` | 更新 `.skillignore` 内容 |
| GET | `/api/doctor` | 运行所有健康检查（JSON） |
| GET | `/api/health` | 存活探针；server 就绪后返回 `200` |
| GET | `/api/version` | 当前/最新版本，以及是否有可用升级 |
| POST | `/api/upgrade` | 就地运行 `skillshare upgrade`（当二进制文件为开发构建时返回 `devMode: true`） |
| POST | `/api/restart` | 重启本地 UI server；可选的 `{ "clearCache": true }` body 会先清除缓存的 UI 资源 |

## 就地升级

当 dashboard 检测到有新的 CLI 发布版本可用时，**Update** 对话框和 **Doctor** 页面的 *Version* 卡片都会出现 **Update now** 按钮：

1. UI 调用 `POST /api/upgrade`，在宿主机上运行 `skillshare upgrade`。
2. 新二进制文件就位后，UI 调用 `POST /api/restart` 重启本地 server。
3. 浏览器轮询 `GET /api/health`，并在新 server 就绪后自动重新加载。

如果运行中的二进制文件是开发构建（`version == "dev"`），upgrade 端点会返回
`devMode: true`，UI 会模拟一次重启而不修改磁盘上的任何内容。

如果自动重新加载没有完成，对话框会提示你运行 `skillshare ui start` 来重新拉起后台 server。

## 反向代理 {#reverse-proxy}

如果你在共享服务器（例如家庭实验室、内部工具平台）上运行 dashboard，并处于反向代理之后，可以用 `--base-path` 把它挂载在子路径下，与其他服务共存：

```bash
skillshare ui --base-path /skillshare --host 0.0.0.0 --no-open
```

或者通过环境变量：

```bash
SKILLSHARE_UI_BASE_PATH=/skillshare skillshare ui --host 0.0.0.0 --no-open
```

### Nginx

```nginx
location /skillshare/ {
    proxy_pass http://127.0.0.1:19420;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
}
```

### Caddy

```
handle_path /skillshare/* {
    reverse_proxy 127.0.0.1:19420
}
```

:::tip
不使用 `--base-path` 时，dashboard 的行为与以前完全一致——直接访问 `localhost:19420` 不需要任何配置。
:::

:::note MCP 设置
MCP 页面只有在浏览器通过 `localhost` 或 IP 地址打开 dashboard 时才能工作，例如 `http://192.168.1.20:19420`。通过域名访问时（包括通过反向代理），MCP 请求会返回 403：DNS rebinding 攻击总是使用域名。要在远程机器上管理 MCP 设置，用 `ssh -L 19420:127.0.0.1:19420 HOST` 转发端口，然后打开 `http://localhost:19420`。
:::

## Docker 使用方式

要在 Docker 内使用 web UI（首次下载 UI 需要网络访问）：

```bash
make playground

# 在容器内：
skillshare ui --host 0.0.0.0 --no-open
```

然后在宿主机上打开 `http://localhost:19420`（19420 端口已自动映射）。

## Project Mode

web dashboard 完整支持 project 级 skills：

```bash
cd my-project
skillshare ui -p
```

或者，如果 `.skillshare/config.yaml` 存在（自动检测），直接运行 `skillshare ui` 即可。

dashboard 会读写 `.skillshare/config.yaml`，同步到 project 本地的 targets，并在安装后协调 remote skill 条目——就像 CLI 一样。

## 运行时 UI 下载

`skillshare ui` 会在首次启动时自动从匹配的 GitHub Release 下载预构建的 UI 资源。这些资源会缓存在 `~/.cache/skillshare/ui/<version>/`（遵循 `XDG_CACHE_HOME`），因此后续启动是即时且离线的。

- **首次运行** 需要网络连接来下载 UI 资源（约 1 MB）
- **后续运行** 使用缓存的资源——无需网络
- **升级时**，旧的缓存版本会自动清理；新 UI 会在 `skillshare upgrade` 期间被预先下载
- **手动清除缓存**，运行 `skillshare ui --clear-cache`

## Homebrew 说明

所有安装方式（Homebrew、安装脚本、手动二进制文件）都使用运行时 UI 下载。当你运行 `skillshare ui` 时，它会在首次启动时自动从 GitHub 下载 UI 资源。之后会使用缓存的资源离线运行。

清除已下载的 UI 缓存：

```bash
skillshare ui --clear-cache
```

## 架构

web UI 是一个单页 React 应用，在运行时从匹配的 GitHub Release 下载，并从磁盘缓存中提供服务（`~/.cache/skillshare/ui/<version>/`）。

```
skillshare ui
  ├── Go HTTP server (net/http)
  │   ├── /api/*    → REST API handlers
  │   └── /*        → Cached React SPA (runtime download)
  └── Browser opens http://127.0.0.1:19420
```

## 另请参阅

- [status](/docs/reference/commands/status) —— CLI 状态检查
- [sync](/docs/reference/commands/sync) —— CLI 同步命令
- [Project Setup](/docs/how-to/sharing/project-setup) —— Project mode 设置指南
- [Docker Sandbox](/docs/how-to/advanced/docker-sandbox) —— 在 Docker 中运行 UI
