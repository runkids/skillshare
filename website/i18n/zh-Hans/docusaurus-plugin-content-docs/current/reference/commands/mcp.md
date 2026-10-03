---
sidebar_position: 3
---

# mcp

管理可移植的 MCP 连接定义，并同步各原生 Agent 的设置。
从 [Set up MCP once](/docs/how-to/daily-tasks/sharing-mcp) 开始了解。

## 命令

```bash
skillshare mcp
skillshare mcp add
skillshare mcp edit
skillshare mcp edit docs --url https://updated.example/mcp --no-tui
skillshare mcp add docs --url https://example.com/mcp --target claude --sync
skillshare mcp add local --target codex -- company-mcp --workspace /path/to/workspace
skillshare mcp import docs --from claude --target claude --target cursor --sync
skillshare mcp import docs --file ./provider.json --target claude
skillshare mcp list --json
skillshare mcp check
skillshare mcp check docs --json --no-dns
skillshare mcp check --live --timeout 30s
skillshare mcp remove docs --sync
skillshare mcp remove docs --keep-files
skillshare mcp restore BACKUP_ID --dry-run
skillshare sync mcp --dry-run --json
skillshare sync mcp
skillshare sync --all
```

| 选项 | 含义 |
|---|---|
| `--target CLIENT` | 接收方 client；重复此标志可选择多个 client。`--target none` 会把 server 保留在 Skillshare 中，而不写入任何 client。参见[下文](#keep-a-server-without-syncing-it) |
| `--url URL` | 用于 `add` 的 Streamable HTTP 端点 |
| `-- command args...` | 用于 `add` 的本地可执行文件及其字面参数 |
| `--disabled` | Project mode，配合 `add` 使用：关闭一个由 Agent 全局配置定义的 server。参见[下文](#turn-off-a-global-server-in-one-project) |
| `--tools-allow TOOLS` | 只保留这些工具，用逗号分隔；`*` 匹配任意字符；`""` 表示清除。参见[工具策略](#tool-policy) |
| `--tools-deny TOOLS` | 始终排除这些工具，用逗号分隔；优先于 allow；`""` 表示清除。参见[工具策略](#tool-policy) |
| `--pi-options JSON` | Pi 内置 MCP 的其他单个 server 字段，以 JSON 对象表示。参见 [Pi](#pi-options) |
| `--from CLIENT` | 要导入的现有 client，或 `--file` 的格式 |
| `--file PATH` | 原生 JSON/JSONC、TOML 或 Goose YAML；`.toml` 默认对应 Codex，其他格式会根据其 MCP 区块自动检测；如需明确指定方言请使用 `--from` |
| `--sync` | 保存并同步；非交互式的 add/import/remove 默认仅保存 |
| `--keep-files` | 配合 `remove` 使用：停止管理该 server，并让它的 Agent 条目保持原样。不能与 `--sync` 同时使用。参见[下文](#stop-managing-a-server) |
| `--replace` | 在 add/import 期间显式替换已存在的 source 定义；在 import 时，若导入的 client 条目不同，也会重写它 |
| `--dry-run`, `-n` | 预览，不保存也不写入原生配置 |
| `--json` | 结构化输出；sync/preview 报告只包含名称、路径和动作，不包含 server 值 |
| `--no-dns` | 与 `check` 一起使用：跳过远程 server 的主机名解析。参见[下文](#check-servers-before-an-agent-starts-them) |
| `--live` | 与 `check` 一起使用：还会启动每个本地 server，并调用每个远程 server。参见[下文](#probe-servers-live) |
| `--timeout DURATION` | 与 `check --live` 一起使用：每个 server 探测的时间限制，例如 `30s`；默认 `10s` |
| `--no-tui` | 禁用交互式菜单；`tui: false`、`--json` 或非终端输入/输出也会禁用它 |
| `--revision ID` | 要求 add/import/remove 或 `sync mcp` 匹配指定的 preview |
| `--global`, `-g` | 使用 global Skillshare 配置 |
| `--project`, `-p` | 使用 project Skillshare 配置 |

不带子命令时，`mcp` 会在交互式终端中打开可搜索的管理器，
或在非交互模式下打印状态。不带名称的非交互式 import 会列出
已解析的候选项以供选择，并不会保存。候选项包含可移植的
定义，可识别的密钥会被转换为引用。Agent 专属的
字段会作为警告列出并被省略；已禁用的 server 和不受支持的
传输方式会阻止该候选项。`restore` 在应用之前总会再次预览；
使用 `--dry-run` 可以在不应用的情况下查看预览。

`--pi-extension`、`--pi-options-prune` 和 `--direct-tools` 已在 0.23.0 中移除，
现在使用它们会失败，并提示应改用什么。参见
[从 0.22 升级 Pi](#pi-migration)。

`sync mcp` 接受作用域标志、`--dry-run`、`--json`、`--no-tui` 和 `--revision`。
`sync --all` 包含 skills、agents、extras 和 MCP + hooks；普通的 `sync` 保持其
现有的资源行为。在 `--all` 更改其他资源之前会先检查 MCP 冲突。
资源类型与原生文件是各自独立的操作，而非单一事务。

## 交互式管理

运行 `skillshare mcp` 或 `skillshare mcp list`。与 skills 列表一样，该管理器
支持 `/` 搜索和 `Enter` 查看详情。连接列表会隐藏参数、
header 和环境变量的值，并省略 URL 中的查询参数。

| 按键 | 操作 |
|---|---|
| `a` | 添加一个连接 |
| `i` | 导入一个或多个连接 |
| `e` | 编辑所选的连接 |
| `x` | 移除所选的连接 |
| `s` | 预览并确认同步 |
| `b` | 按 client 浏览备份，按最新排序 |
| `r` | 刷新状态 |
| `q` | 退出 |

省略名称或备份 ID 时，`mcp edit`、`mcp remove` 和 `mcp restore` 会提供选择菜单。
编辑器涵盖 command/URL、参数、环境
变量、HTTP header、bearer-token 环境引用、接收方
target 以及[工具策略](#tool-policy)（**工具**）。参数接受每行一个字面参数，或一个 JSON 数组。切换
传输方式时会清除不适用于新连接类型的字段。

Add、edit、remove 和 import 会在 **Save and sync** 或 **Save
only** 之前显示预览。Remove 还提供 **Stop managing**，效果与 `--keep-files` 相同。Escape
会取消待处理的草稿。Restore 会预览并确认
对 Agent 条目所做的更改；它不会重写 source 定义。

不带 server 名称的 import 支持多选（`Space` 切换选中，
`a` 全选）。无效的候选项会被跳过；已存在的 source 名称会被跳过，
除非指定了 `--replace`。为整批操作选择一组兼容的接收方 client。
整批数据会先经过验证，然后 source 只被保存一次；
之后的原生文件 I/O 失败仍保留现有的恢复行为。

对于脚本，提供名称和标志。`mcp edit NAME --url URL`、
`mcp edit NAME --target CLIENT` 和 `mcp edit NAME -- command args...` 会更新
指定的字段，同时保留其他适用的设置。除非加上 `--sync`，
否则它们只会保存。使用 `--no-tui` 时，remove 需要名称，restore
需要备份 ID。`--dry-run` 绝不会保存或同步更改。

## Source 字段

选择内联的 `mcp.servers`，或使用 `sources.mcp` 指定的外部文件。
外部文件有一个顶层的 `servers` 映射。`mcp.targets` 和
[`mcp.projects`](#manage-several-projects-from-the-global-config) 保留在
Skillshare 配置中。schema 是仓库中的 `schemas/mcp.schema.json`。

| Server 字段 | 含义 |
|---|---|
| `command` | 本地可执行文件；与 `url` 互斥 |
| `args` | 本地可执行文件的字面参数列表 |
| `env` | 本地环境变量值：字符串或 `{fromEnv: VARIABLE}` |
| `url` | HTTP(S) MCP 端点；不能包含内嵌凭据或 fragment |
| `headers` | HTTP header：字符串或 `{fromEnv: VARIABLE}` |
| `bearerToken` | `{fromEnv: VARIABLE}`；不能与 Authorization header 共存 |
| `transport` | 可选的 `stdio` 或 `streamable-http`；省略时会自动推断 |
| `targets` | 可选的接收方 client；覆盖 `mcp.targets`。空列表会让该 server 只保留在 Skillshare 中。参见[下文](#keep-a-server-without-syncing-it) |
| `tools` | 哪些工具会提供给模型：`allow`、`deny`。只需写一次，会按各 Agent 转换。参见[工具策略](#tool-policy) |
| `piOptions` | Pi 内置 MCP 的其他单个 server 字段。参见 [Pi](#pi-options) |
| `disabled` | 仅限 `true`，不能有其他连接字段，且必须有 project 在作用范围内：project mode，或 `mcp.projects` 下的某个项目根目录。参见[下文](#turn-off-a-global-server-in-one-project) |

Client ID 有 `claude`、`codex`、`cursor`、`vscode`、`opencode`、`kilocode`、
`grok`、`antigravity`、`amp`、`claude-desktop`、`cline`、`copilot`、`factory`、`gemini`、
`goose`、`junie`、`kiro`、`lmstudio`、`warp`、`windsurf` 和 `pi`。
`grok` 指的是官方的 xAI Grok CLI。Server 名称使用字母、
数字、点、下划线和连字符。一个 server 在同步之前，必须
直接或通过 `mcp.targets` 选择至少一个 client，除非它自己的
`targets` 是空列表。

### 保留 server 但不同步 {#keep-a-server-without-syncing-it}

带有 `targets: []` 的 server 会保留在 Skillshare source 中，不会被写入任何 client。
可以用它把一个 server 从所有 client 中拿掉，同时保留它的定义以备日后使用。
如果它之前同步过，下一次同步会从那些 client 中移除它的条目。

```yaml
mcp:
  targets: [claude, codex]
  servers:
    docs:
      url: https://example.com/mcp
      targets: []
```

```bash
skillshare mcp add docs --url https://example.com/mcp --target none
skillshare mcp edit docs --target none
skillshare mcp edit docs --target claude   # 把它加回来
```

- **不写 `targets` 则是另一回事。** 此时该 server 会继承 `mcp.targets`，如果
  那个列表也是空的，它就会被拒绝。
- `none` 不能与 client 一起使用。
- 在终端的选择菜单中，不选择任何 client 直接确认。在仪表盘中，取消勾选
  所有 client；该 server 会被标记为 **尚未选择 Agent**。
- 对 project 的 server 以及 `mcp.projects` 下的 server，行为相同。
- `disabled` 条目仍然需要至少一个 client，因为它必须在某个地方把该 server
  关闭。
- `mcp list` 会把这样的 server 显示为 `kept no targets`。

对于 Grok，名称必须以字母或下划线开头，只能包含字母、
数字、连字符和单个下划线，且不能以下划线结尾。
诸如 `company-docs` 这样的名称在所有支持的 client 中都可用。

## 原生目标位置 {#native-destinations}

| Client | Global | Project | 区块 |
|---|---|---|---|
| Claude Code | `~/.claude.json` | `.mcp.json` | `mcpServers` |
| Codex | `~/.codex/config.toml` | `.codex/config.toml` | `mcp_servers` |
| Cursor | `~/.cursor/mcp.json` | `.cursor/mcp.json` | `mcpServers` |
| VS Code | 用户级 `mcp.json`（如下） | `.vscode/mcp.json` | `servers` |
| OpenCode | `~/.config/opencode/opencode.json` | `opencode.json` | `mcp` |
| Kilo Code | `~/.config/kilo/kilo.jsonc` | `kilo.jsonc` | `mcp` |
| Grok CLI | `~/.grok/config.toml` | `.grok/config.toml` | `mcp_servers` |
| Antigravity (AGY) | `~/.gemini/config/mcp_config.json` | `.agents/mcp_config.json` | `mcpServers` |
| [Amp](https://ampcode.com/docs/customize/mcp) | `~/.config/amp/settings.json` | `.amp/settings.json` | `amp.mcpServers`（字面 key） |
| [Claude Desktop](https://modelcontextprotocol.io/docs/develop/connect-local-servers) | Claude 应用数据目录下的 `claude_desktop_config.json` | 仅限 Global | `mcpServers` |
| [Cline](https://github.com/cline/cline/tree/main/apps/vscode/src/services/mcp) | `~/.cline/data/settings/cline_mcp_settings.json` | 仅限 Global | `mcpServers` |
| [Copilot CLI](https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/add-mcp-servers) | `~/.copilot/mcp-config.json` | `.github/mcp.json` | `mcpServers` |
| [Factory Droid](https://docs.factory.ai/harness/mcp) | `~/.factory/mcp.json` | `.factory/mcp.json` | `mcpServers` |
| [Gemini CLI](https://geminicli.com/docs/tools/mcp-server/) | `~/.gemini/settings.json` | `.gemini/settings.json` | `mcpServers` |
| [Goose](https://block.github.io/goose/docs/guides/config-files/) | `~/.config/goose/config.yaml` | 仅限 Global | `extensions`（YAML） |
| [Junie](https://junie.jetbrains.com/docs/junie-cli-mcp-configuration.html) | `~/.junie/mcp/mcp.json` | `.junie/mcp/mcp.json` | `mcpServers` |
| [Kiro](https://kiro.dev/docs/mcp/configuration/) | `~/.kiro/settings/mcp.json` | `.kiro/settings/mcp.json` | `mcpServers` |
| [LM Studio](https://lmstudio.ai/docs/app/mcp) | `~/.lmstudio/mcp.json` | 仅限 Global | `mcpServers` |
| [Warp](https://docs.warp.dev/agents/capabilities/mcp/) | `~/.warp/.mcp.json` | `.warp/.mcp.json` | `mcpServers` |
| [Windsurf (Cascade)](https://docs.devin.ai/desktop/cascade/mcp) | `~/.codeium/windsurf/mcp_config.json` | 仅限 Global | `mcpServers` |

仪表盘的 server 表单编辑 HTTP header 的方式与编辑环境变量相同，
包括 `fromEnv` 引用。**View what each Agent gets** 位于某个 server 的菜单中，
以及其表单中文件计数旁边，只读地显示 Sync 会为所选
client 写入的原生文本；在表单中它反映的是尚未保存的编辑。密钥仍以
引用形式呈现。

JSON 条目会按照文件自身的缩进方式，逐字段单独一行写入。如果 Skillshare
拥有的某个条目仍然写在一行上，会被报告为一次 `update` 并重新
按分行格式写入。它不拥有的条目，以及由人工手动格式化的条目，会保留原有的格式。

仪表盘只提供当前作用域和主机平台下可用的目标位置。每个 server 是
一行，名称下方以 chip 显示它会写入的 client；右侧的计数按钮会打开该 server 的完整 client 列表。仅限 Global 的 client
在 project mode 中无法被选中。右侧的 **Sync** 框列出了尚未写入的
更改：勾选某个 client 只会编辑 source。**Sync MCP** 会列出这些更改，并在你确认后
只写入 MCP 配置文件，同时为每个文件保留一份备份。同一个框中分隔线下方，
有 server 时会出现 **检查**，用于[检查这些 server](#check-servers-before-an-agent-starts-them)；**备份与还原**
用于浏览这些备份。在其下方，**Agents** 列出了此机器上检测到的 client。当某个
client 的 MCP 文件存在，或该 client 用于保存设置的文件夹存在时，就算作
已检测到，因此即使是全新安装、还没有 MCP 文件，也仍会出现在列表中。在 project mode 下，
当项目拥有自己的 MCP 文件，或该 client 在全局范围内被检测到时，就会列出该 client。

其他 client 细节：

- `codex` 这个目标位置是一个 `config.toml`，由 Codex CLI、Codex IDE
  扩展和 ChatGPT 桌面应用共享，因此同步到 `codex` 的 server 会出现在
  这三者中。ChatGPT 桌面应用在 **Settings → MCP servers** 下列出它们。
  Codex 只有在信任某个项目时才会读取 `.codex/config.toml`；在不受信任的
  项目中，同步的 server 不会被加载，也不会报错。`cwd`、
  `http_headers_helper`、审批模式、超时以及 `oauth` 表
  都没有可移植的形式：import 会将它们省略并给出警告，sync 则会将它们保留在
  现有条目中。`enabled_tools` 和 `disabled_tools` 来自
  [工具策略](#tool-policy)，导入时也会读回策略中。由 Codex 插件捆绑的 MCP server 配置在
  `plugins.<plugin>.mcp_servers` 下，不由此处管理。
- Claude Desktop 的文件同步**仅支持 stdio**，仅限 macOS 和 Windows。
  其目录在 macOS 上为 `~/Library/Application Support/Claude`，
  在 Windows 上为 `%APPDATA%/Claude`。远程连接器需要在应用内配置。
- Cline 面向默认的 VS Code Stable profile，而不是 Cline CLI 或其他 IDE。
- Copilot CLI 条目会得到 `tools`：即[工具策略](#tool-policy)允许的精确工具名称，
  否则为 `["*"]`。导入时会把 `tools` 读回策略中。如果存在 project 级的 `.mcp.json`，同步会中止，因为 Copilot 优先读取该
  文件而不是 `.github/mcp.json`；请先合并这些文件。
  在 project mode 下同时选择 Claude Code 和 Copilot CLI 也会在写入任一文件之前
  被阻止。请为其中一个 client 使用 global mode。
- Gemini 使用 `httpUrl` 表示 Streamable HTTP。它的 `url` 字段代表旧版 SSE，
  在导入时会被拒绝。Cline 使用 `type: streamableHttp`；Goose 使用
  `type: streamable_http` 和 `uri`。Skillshare 会自动转换这些格式。
- Goose 在 Windows 上使用 `%APPDATA%/Block/goose/config/config.yaml`。YAML 编辑
  会保留不相关的设置、注释和内置 extension，但可能会改变
  格式。别名（alias）、merge、重复的 key 以及多个文档会阻止编辑。
  内置 extension 和 keychain 中的 `env_keys` 无法作为可移植的 MCP
  连接被导入。
- Claude Code 会跳过名为 `workspace`、`claude-in-chrome` 或 `computer-use` 的 server，
  这些名称被它保留给内置 server。它也绝不会向远程 server 发送自己的
  凭据：`ANTHROPIC_API_KEY`、`ANTHROPIC_AUTH_TOKEN`、`AWS_BEARER_TOKEN_BEDROCK`、
  `HTTPS_PROXY` 和 `NPM_TOKEN` 在 `url` 和 `headers` 中会读取为空。Skillshare 对
  Claude 拒绝这两者。请把凭据复制到一个你自己命名的变量中。
- Claude Code 还有一个本地作用域：用 `claude mcp add`（不带
  `--scope`）添加的 server 会按项目存储在 `~/.claude.json` 中。本地 server 会
  整体覆盖 `.mcp.json` 或 user 作用域中同名的 server。在 project mode 下，
  Skillshare 会在其隐藏的条目旁边报告这样的 server，但不会阻止同步。可以在项目文件夹中
  运行 `claude mcp remove NAME -s local` 来移除它。
- Cline 的 VS Code 扩展、CLI 和 SDK 共享 `~/.cline/data/settings/`。该
  扩展会把它较旧的 VS Code `globalStorage` 文件迁移到那里一次，之后就不再
  读取旧文件，因此 Skillshare 只有在 `~/.cline/data` 尚不存在时才会写入旧文件。
  `CLINE_MCP_SETTINGS_PATH`、`CLINE_DATA_DIR` 和 `CLINE_DIR` 会按此顺序
  被遵循。
- Windsurf 的支持针对的是文档中所述的 Cascade 配置。Windsurf 较新的
  Devin Local agent 读取它自己的 `~/.config/devin/mcp_config.json`，Skillshare
  不管理这个文件。Warp 的 project 连接仍需要在每次会话中在 Warp 内部
  批准。
- Amp 只有在执行 `amp mcp approve <name>` 之后，才会从 project 的
  `.amp/settings.json` 运行某个 server。Global server 不需要批准。
- Kiro 只会为其 "Mcp Approved Env Vars" 设置中列出的名称展开 `${VARIABLE}`，
  并且只接受 localhost 的 `http://` URL。
- VS Code 为每个非默认 profile 在 `User/profiles/` 下保留单独的 `mcp.json`。
  Skillshare 管理的是默认 profile 的文件。

环境引用对 Amp、Copilot CLI、
Factory、Gemini CLI 和 Kiro 导出为 `${VARIABLE}`，对 Cline 和 Windsurf 导出为
`${env:VARIABLE}`。Claude Desktop、Goose、Junie、LM Studio 和 Warp 目前会拒绝
`fromEnv` 和 `bearerToken` 的导出，因为它们的原生插值方式尚未得到验证。
请使用不带自定义凭据的连接，或在受支持的情况下在接收方
client 中进行认证。Skillshare 绝不会将引用解析为明文。

Antigravity 使用当前的[官方 MCP 配置](https://antigravity.google/docs/mcp)，
包括用于远程连接的 `serverUrl`。Skillshare 会自动转换可移植的 `url`。
较旧的 `.gemini/antigravity/` 和 `.gemini/antigravity-cli/` 配置
位置不受管理。Antigravity 的 `fromEnv` 和 `bearerToken` 导出会被
阻止，因为它有据可查的配置并未指定环境
插值方式。请使用不需要自定义密钥 header 的连接，并在 Antigravity 内部完成
受支持的 OAuth 登录。Skillshare 绝不会将引用展开为明文凭据。

OpenCode 会为其 global 目录遵循 `XDG_CONFIG_HOME`。如果已存在
`opencode.jsonc`，会使用它而不是创建 `opencode.json`。在项目中，
OpenCode 还会从 `.opencode/` 读取这两个文件名，因此如果文件放在那里，Skillshare 就会
写入那个文件；新文件则会创建在项目根目录。如果存在多个
文件，请在同步之前先合并它们。自定义的 OpenCode 配置
路径、目录覆盖、内联配置以及继承的祖先文件不受管理。
它们可能会在 OpenCode 中覆盖所选的目标位置。

Kilo Code 使用与 OpenCode 相同的格式。它会从项目根目录和 `.kilo/`
读取 `kilo.jsonc` 和 `kilo.json` 并进行合并，因此 Skillshare 会写入
已存在的那个文件，只有在两者都不存在时才会创建 `kilo.jsonc`。如果
两者都存在，请在同步之前先合并它们。`KILO_CONFIG`、
`KILO_CONFIG_DIR` 以及较旧 VS Code 扩展的 `mcp_settings.json` 不受
管理。

Kilo Code 将 project 配置视为不受信任。它不允许在其中使用 `{env:VARIABLE}`
引用，并且一旦发现整个 project 文件就会忽略它。因此在 project
mode 下，Skillshare 会拒绝使用了 `fromEnv` 或 `bearerToken` 的 Kilo Code server。请在
允许使用引用的 global mode 中定义该 server。

OpenCode 和 Kilo Code 使用 `local`/`remote` 类型以及 `{env:VARIABLE}` 引用；Grok 使用
`${VARIABLE}` 引用。Skillshare 会自动转换这些格式。Claude 的
`"type": "streamable-http"` 会作为 HTTP 导入。已禁用的连接会阻止导入。
其他没有可移植等价形式的原生选项，例如 Codex 的
`startup_timeout_sec` 或 `envFile`，会在导入时被省略并给出警告；
sync 会将它们保留在 Agent 现有的条目中。Pi 使用它的内置 MCP；参见
[下文](#pi)。

VS Code Stable 的默认用户文件是：

- macOS：`~/Library/Application Support/Code/User/mcp.json`
- Linux：`${XDG_CONFIG_HOME:-~/.config}/Code/User/mcp.json`
- Windows：`%APPDATA%/Code/User/mcp.json`

Global 的 Claude、Codex、Grok 和 Copilot 路径会遵循 `CLAUDE_CONFIG_DIR`、`CODEX_HOME`、
`GROK_HOME` 和 `COPILOT_HOME`。`OPENCODE_CONFIG` 和 `OPENCODE_CONFIG_DIR` 不受管理。Amp 和 Goose 在使用其
`.config` 路径的平台上会遵循 `XDG_CONFIG_HOME`。
Project 目标位置是相对于所选项目根目录的。Project 信任、
server 批准和身份验证仍是接收方 Agent 自身的责任。

### 某个 Agent 的另一个账号 {#accounts}

声明为[某个 Agent 的另一个账号](/docs/reference/targets/configuration#agent-config-dir)的 target 同样是一个 MCP target，适用于 `claude`（`CLAUDE_CONFIG_DIR`）、`codex`（`CODEX_HOME`）和 `pi`（`PI_CODING_AGENT_DIR`）。它的 server 会以该 Agent 的格式，写入这个账号自己的文件：Claude 为 `<config_dir>/.claude.json`，Codex 为 `<config_dir>/config.toml`，Pi 为 `<config_dir>/mcp.json`。

```yaml
targets:
  claude-work:
    agent: claude
    config_dir: ~/.claude-work

mcp:
  targets: [claude, claude-work]      # 两个账号都会得到每一个 server
  servers:
    docs:
      url: https://example.com/mcp
    jira:
      command: jira-mcp
      targets: [claude-work]          # 仅工作账号
```

这里 `docs` 会写入 `~/.claude.json` 和 `~/.claude-work/.claude.json`，而 `jira` 只写入第二个文件。`--target claude-work` 可用于 `mcp add` 和 `mcp edit`，仪表盘也会把这个账号列在各个 Agent 旁边。

每个账号读取的是相同的 project 文件，因此在 `mcp.projects` 内以及 project mode 下，请使用 Agent 自身的名称。Claude Code 会把项目的关闭列表保存在每个账号各自的文件中：[在某个项目中关闭一个 server](#turn-off-a-global-server-in-one-project) 时，会把这个开关写入每一个拥有该 server 的账号。`mcp import --from claude-work` 以及仪表盘的「从 target 导入」读取的是这个账号自己的文件。`mcp import --file <path> --from claude-work` 读取的则是你自己导出的文件，采用这个账号所属 Agent 的格式。

## 在单个项目中关闭一个 global server {#turn-off-a-global-server-in-one-project}

一个 Agent 会同时读取它自己的 global MCP 文件和 project 的文件。因此
在 global 文件中定义的 server 会在每个项目中加载。要阻止它在
某个项目中加载，请添加一个**名称与该 Agent 的 global 文件所用名称相同**的条目，
并将其标记为 `disabled`。

这个方法仅适用于四个 client：

| Client | 是否支持 | Skillshare 写入的内容 |
|---|---|---|
| Claude Code | 是 | `~/.claude.json`：在此项目的 `disabledMcpServers` 列表中写入名称 |
| OpenCode | 是 | `opencode.json`：`"NAME": {"enabled": false}` |
| Kilo Code | 是 | `kilo.jsonc`：`"NAME": {"enabled": false}` |
| Pi | 是，从 `mcp.projects` | `.pi/mcp.json`: `"NAME": {"command": "...", "enabled": false}`，见下文 |
| Codex | 否 | 见下文 |
| 其他所有 client | 否 | 选择其中任何一个都会报错；不会写入任何内容 |

只会写入这个开关。Agent 仍会使用其 global 条目中的 command 或 URL。
其他 client 之所以被拒绝，是因为它们会用 project 条目整体替换 global 条目，
或者没有 project 文件，因此单独一个开关会破坏
该 server，而不是把它关闭。

Codex 因不同的原因被拒绝。它确实会将 `.codex/config.toml` 逐字段地
合并到 global 文件之上，因此如果某台机器的 global 配置定义了该 server，单独的
`enabled = false` 是有效的。但在没有定义该 server 的机器上，合并后的条目
没有 `command` 或 `url`，于是 Codex 会因 `invalid transport` 而无法加载它的
整个配置。`.codex/config.toml` 通常会被提交到仓库，因此一位队友的开关
可能会导致另一位队友的 Codex 无法启动。请改为在每台机器上单独
关闭该 server，在 `~/.codex/config.toml` 中设置 `enabled = false`。

Pi 会用项目中的同名条目整条替换 global 条目。没有 `command` 或 `url` 的条目，Pi 1.0.1
之前会跳过；1.0.1 起会关闭 global server，但在 global 配置没有该 server 的机器上，Pi 每次启动都会警告。
因此对 Pi，Skillshare 会写入 global server 的 `command`，或去掉 query 的 `url`，再加上
`enabled: false`。被关闭的 server 不会启动，所以 args、env 和 headers 都不会写进项目文件，
其他项目也照常使用该 server。每次同步都会根据 global server 重写这个条目。这需要 global
server，所以只适用于 global 配置中 `mcp.projects` 下的项目；项目自己的配置看不到 global
server，在那里的 `disabled` 条目中使用 `pi` 会报错。

### OpenCode 和 Kilo Code

```bash
cd my-project
skillshare mcp add company-docs --disabled --target opencode --target kilocode
skillshare sync mcp
```

```yaml
# .skillshare/config.yaml
mcp:
  servers:
    company-docs:
      disabled: true
      targets: [opencode, kilocode]
```

### Claude Code

Claude Code 会从某个作用域整体取用一个 server 条目，绝不会合并字段，因此
在 `.mcp.json` 中的开关会替换该 server，而不是将其关闭。它把
自己按项目区分的关闭列表保存在 `~/.claude.json` 中，也就是 `/mcp` 面板所编辑的那个列表。
Skillshare 会在该项目的绝对路径下把名称添加到那里，而不会向
`.mcp.json` 写入任何内容。

```bash
skillshare mcp add company-docs --disabled --target claude
skillshare sync mcp
```

- 该列表存在于你的机器上，而不是仓库中。每位队友都需要在自己的
  检出目录中运行一次 `skillshare sync mcp`。
- 你自己在 `/mcp` 中关闭的名称绝不会被认领或移除。
- 如果你在 `/mcp` 中重新打开该 server，下一次同步会报告冲突。
  请从 `.skillshare/config.yaml` 中移除该条目，或者用 replace 再次将其关闭。
- 该列表以项目路径为 key，因此移动项目后需要重新同步一次。

### 规则

- **必须有 project 在作用范围内。** 在拥有 `.skillshare/config.yaml`（由 `skillshare init -p`
  创建）的项目内运行，传入 `-p`，或者把该条目放在
  [`mcp.projects`](#manage-several-projects-from-the-global-config) 下的某个项目根目录里。
  在 global `mcp.servers` 中没有 project 在作用范围内，因此会被拒绝。
- **`disabled` 必须单独存在。** 该条目只能带 `targets`。添加 `command`、`url`、
  `env`、`headers`、`piOptions` 或 `tools` 会报错。
- **`targets` 可以省略。** 此时该条目会跟随项目的 targets：每次同步时，它都会写入
  该项目所用且支持按项目开关的 client。在 `mcp.projects` 下，Skillshare 还知道
  同名的 global server，因此范围会进一步缩小到该 server 写入的那些 client。之后更改项目的
  targets 时，无需修改该条目。
  如果想自行决定，请列出 `targets`；该列表中出现不受支持的 client 会报错。
- **名称必须匹配。** Skillshare 不会读取 Agent 的 global 文件，因此
  无法检查该文件中是否确实存在这个名称的 server。如果名称什么都没匹配到也无妨：
  Agent 会忽略它。
- **要重新打开它**，移除该条目（`skillshare mcp remove company-docs`）
  并同步。该开关会从它被写入的那个文件中移除：project 自己的文件，
  对 Claude Code 而言则是 `~/.claude.json`。
- **Skillshare 自身定义的 server 不需要这个方法。** 只需在该 server 上取消选择
  该 Agent，下一次同步就会移除它的条目。

在仪表盘中，这就是 **关闭全局服务器** 按钮。在 project mode 中它位于 **服务器** 标题旁边；
在项目的 MCP 标签页中，则位于 **添加服务器** 旁边。

## 通过 global 配置管理多个项目 {#manage-several-projects-from-the-global-config}

Project mode 会把每个项目的 MCP 设置保存在该项目的
`.skillshare/config.yaml` 中，并且需要在该文件夹内同步。如果你更希望
把所有项目放在同一个地方，可以在 **global** 配置的 `mcp.projects` 下列出这些项目文件夹。
之后在任意位置运行一次 `skillshare sync mcp`，就会在同一个计划中写入 global
文件和每个项目的文件。

```yaml
# ~/.config/skillshare/config.yaml
mcp:
  servers:
    context7:
      command: npx
      args: ["-y", "@upstash/context7-mcp"]
      targets: [claude, opencode]
  projects:
    ~/work/project01:
      targets: [claude, opencode]
      servers:
        context7:                  # 仅在此项目中关闭
          disabled: true
    ~/work/project02:
      servers:
        internal-docs:             # 仅存在于此项目中
          url: https://example.com/mcp
          targets: [opencode]
```

项目只需列出与 global 配置不同的部分。像 `context7` 这样的 global server 不需要在这里
添加条目：Agent 会同时读取它的 global 文件和 project 的文件，因此它已经会在每个项目中
加载。一条 `disabled` 条目会[在该文件夹中将它关闭](#turn-off-a-global-server-in-one-project)，
对其中列出的所有 client 生效。

每个 key 都是一个项目文件夹：可以是绝对路径，也可以是以 `~` 开头的路径。它下面写的是
该项目自己的 `config.yaml` 在 `mcp` 下会包含的同样的 `targets` 和 `servers`，
并且它们会被写入相同的 [project 文件](#native-destinations)。没有 `targets` 的
项目会继承 global 的 `mcp.targets`。

当同一个 server 出现在多个位置时，预览会指明对应的文件：

```text
context7     add          opencode (~/.config/opencode/opencode.json)
context7     add          opencode (~/work/project01/opencode.json)
```

从列表中移除某个项目后，下一次同步会移除 Skillshare 在那里写入的条目，
与移除一个 server 的行为相同。

要让多个项目使用同一个 server，可以用 YAML anchor 定义一次，然后
复用它：

```yaml
mcp:
  projects:
    ~/work/project01:
      servers:
        internal-docs: &internal-docs
          url: https://example.com/mcp
          targets: [opencode]
    ~/work/project02:
      servers:
        internal-docs: *internal-docs
```

请把 anchor 保留在 `mcp.projects` 内部。指向 `mcp.servers` 上某个 anchor 的别名（alias）
也能工作，但 `skillshare mcp add` 和仪表盘会重写 `mcp.servers`；它们在
保存时会把这样的别名完整展开写出，以保证文件仍然有效，此后它就不再
跟随对该 global server 的后续修改。

### 仪表盘中的项目 {#projects-in-the-dashboard}

在 global mode 下，仪表盘中有一个 **项目** 页面。它会列出
[`projects`](/docs/reference/targets/configuration#projects) 和 `mcp.projects` 下的每个文件夹，
每个项目都有一个 **MCP** 标签页。

![项目的 MCP 标签页：按项目开关的全局 server，以及项目专用的 server](/img/projects-mcp-tab.png)

- **添加项目** 需要填写文件夹和它的 targets。勾选 **MCP** 可以让该文件夹同时列在
  `mcp.projects` 下。
- **MCP** 标签页列出每个 global server，并各带一个开关。关闭其中一个，会保存一条
  不带 `targets` 的 `disabled` 条目，因此它会如[上文](#turn-off-a-global-server-in-one-project)所述
  跟随项目的 targets；重新打开则会移除该条目。其下方是只存在于该项目中的 server。
- 已关闭的 server 会显示它在哪些 Agent 中被关闭的 logo。如果项目的某个 Agent 没有
  按项目的开关，该行会说明这个 server 在那里仍会加载。如果某个条目列出了自己的
  `targets`，且与项目的 targets 不同，就会出现 **改成与项目一致**：它会把该条目重新保存为
  不带 `targets` 的形式。
- 该标签页 Sync 框中的 **Sync MCP** 会写入整个 MCP 计划，并说明其中有多少更改
  位于此项目之外。项目页面顶部的 **Sync project** 只写入此项目的 skills、agents 和 MCP。
- **默认值** 位于 **MCP** 标签页底部，用于编辑 `mcp.targets`。
- 当项目自己的 Agent 文件中有 Skillshare 未管理的 server 时，该标签页会在列表上方
  说明这一点，并提供 **Import**。参见[下文](#unmanaged-servers)。

保存只会重写你修改过的那个项目。其他项目的 YAML 保持原样，
包括 anchor 和别名（alias），写成 `~/work/app` 的文件夹也会保留它的 `~`。与
本页其他地方一样，保存只会修改 `config.yaml`；写入文件的是 Sync。

限制：

- `mcp.projects` 只会从 global 配置中读取。包含它的 project 配置会
  被拒绝。
- 没有命令可以编辑它：`skillshare mcp add` 管理 `mcp.servers`，并让
  `mcp.projects` 保持原样。请在 `config.yaml` 中编辑它，或者在
  [仪表盘](#projects-in-the-dashboard)中编辑。
- 以 Claude Code 为 target 的 `disabled` 条目会写入 `~/.claude.json`，也就是写入
  global server 的同一个文件，因为 Claude Code 把自己按项目区分的关闭列表保存在那里。
  server 本身则保持原样。
- 如果某个文件夹同时也有自己的 `.skillshare/config.yaml` 在管理同一个条目，
  计划会报告冲突，而不是将其覆盖。

## 在 Agent 启动 server 之前检查它们 {#check-servers-before-an-agent-starts-them}

```bash
skillshare mcp check
skillshare mcp check docs github --json
skillshare mcp check --no-dns
```

`mcp check` 会针对 source 中的每个 server（或只针对指定的 server）回答“它按同步后的样子能否正常工作？”。
在 global 配置中，它还会检查 [`mcp.projects`](#manage-several-projects-from-the-global-config)
下每个根目录的 server，并按该根目录读取每个 Agent 的规则和同步状态。它是只读的：除非加上 [`--live`](#probe-servers-live)，
否则不会启动 server、发送 HTTP 请求、运行命令或写入文件。

| 检查项 | 级别 |
|---|---|
| `env`、`headers` 或 `bearerToken` 中的 `fromEnv` 变量未设置或为空 | error |
| 在 `PATH` 上找不到本地 server 的 `command`（开头的 `~/` 会被展开） | error |
| 远程 server 的主机无法通过 DNS 解析（限时 3 秒；用 `--no-dns` 跳过） | warning |
| 某个 Agent 的规则拒绝该 server，例如 Claude Code 保留的名称 | error |
| 某个 Agent 的条目与 source 冲突，与 `sync mcp --dry-run` 相同 | error |
| 某个 Agent 的条目尚未写入或尚未更新 | warning |
| 该 server 设置了 `targets: []`，只保存在 Skillshare 中 | info |
| 某个所选 Agent 无法容纳该 server [工具策略](#tool-policy)的某部分 | warning |

变量的值永远不会被打印。只要发现任何 error，命令就以 1 退出，否则以 0 退出；warning 永远不会
导致失败。未知的 server 名称是一个 error，并会列出已知的名称。一个名称会选中 global 和每个项目中
所有同名的 server，已知名称也包括项目 server。

在终端中，项目 server 的标题会注明它所属的项目：

```text
✓ docs
  · claude: in sync
✗ docs  (project ~/work/app)
  ✗ command no-such-mcp-binary was not found on PATH
  ! claude: not synced yet; run skillshare sync mcp
```

使用 `--json` 时，报告的结构如下：

```json
{
  "servers": [
    {
      "name": "docs",
      "ok": false,
      "findings": [
        { "level": "error", "check": "env", "target": "", "message": "bearerToken reads DOCS_TOKEN, which is not set", "subject": "DOCS_TOKEN" },
        { "level": "warning", "check": "sync", "target": "claude", "message": "not synced yet; run skillshare sync mcp" }
      ]
    },
    {
      "name": "docs",
      "project": "/home/me/work/app",
      "ok": true,
      "findings": [
        { "level": "info", "check": "sync", "target": "claude", "message": "in sync" }
      ]
    }
  ],
  "summary": { "errors": 1, "warnings": 1 }
}
```

`check` 是 `env`、`command`、`url`、`dns`、`client-rule`、`sync`、`targets`、`tools` 或 `live` 之一。
`target` 表示 Agent 或账户；当该发现针对的是 server 本身时为空。
`subject` 在 `env`、`command` 和 `dns` 发现中表示变量、命令或主机；在成功的 `live` 探测中表示
server 报告的名称；在 `live` 登录 warning 中表示资源元数据 URL；其他情况下省略。
`project` 是该 server 所在的 `mcp.projects` 根目录，以绝对路径表示（开头的 `~` 会被展开）；
global server 则省略此字段。`summary` 统计报告中的每个 server，包括项目 server。

在仪表盘中，MCP 页面 Sync 框中的 **检查** 按钮会运行同样的检查。它在有 server 可检查时才会出现，
只在点击时运行，会在 server 列表上方显示摘要，并在每个 server 下方显示它的 error 或 warning；
重新加载页面后不会保留任何内容。MCP 页面只列出 global server，因此它的摘要和各行都不包含
项目 server，即使与某个 global server 同名也是如此。项目的 MCP 标签页在其 Sync 框中
有自己的 **检查**，用于报告该项目自己的 server。变量从启动 `skillshare ui` 的终端中读取。

### 实时探测 server {#probe-servers-live}

```bash
skillshare mcp check --live
skillshare mcp check docs --live --timeout 30s --json
```

`--live` 会先运行静态检查，然后联系每个所选且没有 error 的 server。有 error 的 server
或已禁用的条目不会被联系；会有一条 `info` 发现说明原因。

- **本地（stdio）server。** Skillshare 在你当前的环境中以 `args` 启动 `command`，并加上该
  server 的 `env`，其中每个 `fromEnv` 值都从你的 shell 中读取。项目 server 在其项目文件夹中
  启动，global server 在当前目录中启动。这会像 Agent 一样在你的机器上运行该 server 的代码，
  因此只对你信任的 server 使用 `--live`。Skillshare 会发送 `server/discover`。如果 server
  返回的 error 不是 MCP 协议错误，或者未在超时时间的三分之一内响应，就会被视为早于
  MCP 2026-07-28 的版本，改用 `initialize` 握手。之后 Skillshare 调用 `tools/list` 统计
  工具数量并停止该 server：先关闭 stdin，然后向该 server 的进程组依次发送 SIGTERM 和
  SIGKILL。在 Windows 上则直接终止该进程。
- **远程（Streamable HTTP）server。** Skillshare 带上该 server 的 `headers` 和 `bearerToken`
  POST `server/discover`，并读取 JSON 或 SSE 响应。不带 MCP error 的 `400`、`404` 或 `405`
  会回退到 `initialize`。`401` 是一个 warning，“sign-in required”，并附带
  `WWW-Authenticate` header 中的资源元数据 URL。Skillshare 永远不会登录或启动 OAuth。

每个 server 的整个探测过程只有一个时间限制：10 秒，或 `--timeout`（例如 `30s` 或 `1m`）。
最多同时探测四个 server。不带 `--live` 使用 `--timeout` 是一个 error。

| 结果 | 级别 |
|---|---|
| server 已响应：它的名称和版本、协议版本以及工具数量 | info |
| 远程 server 需要登录（HTTP 401） | warning |
| 命令无法启动、提前退出，或未及时响应 | error |
| 协议错误、不支持的协议版本，或其他任何 HTTP 状态 | error |

本地 server 失败时，消息末尾会附上其 stderr 的最多五行。`env`、`headers` 和 `bearerToken`
的值会从每条消息中移除；短于四个字符的值保持原样。值按写入的原样传递：Skillshare 永远不会
运行 Pi 的 `!command` 值，也不读取 `piOptions`。退出码遵循同样的规则，发现任何 error 时为 1。
`--live` 不写入任何文件，也不写入操作日志条目。

使用 `--json` 时，已响应的 server 还会有一个 `live` 对象：

```json
{
  "name": "docs",
  "ok": true,
  "findings": [
    { "level": "info", "check": "live", "target": "", "message": "responds: docs-server 1.4.0, protocol 2026-07-28, 12 tool(s)", "subject": "docs-server" }
  ],
  "live": { "protocolVersion": "2026-07-28", "serverInfo": { "name": "docs-server", "version": "1.4.0" }, "tools": 12 }
}
```

`serverInfo` 是 server 对自己的描述，没有任何验证。当 server 未被探测或探测失败时，
省略 `live`。

仪表盘的 **检查** 按钮只运行静态检查。仪表盘只在一个地方探测 server：server 对话框
[工具区块](#tool-policy-dashboard)中的 **加载工具**，它会按对话框当前的字段启动 server
一次，列出它的工具。使用 `--json` 时，`live` 还包含 `toolNames`，即 `tools/list` 返回的名称。

## 停止管理某个 server {#stop-managing-a-server}

```bash
skillshare mcp remove docs --keep-files
```

这会把 `docs` 从 source 中移除，并忘记 Skillshare 曾为它写入过哪些 Agent 条目。
不会有任何 Agent 文件被更改。从此以后，这些条目归你所有：sync 既不会移除，也不会更新它们。
`--keep-files` 不能与 `--sync` 同时使用。终端的 remove 向导以 **Stop managing** 提供此选项，
仪表盘的删除对话框也一样，包括 MCP 页面和项目的 **MCP** 标签页。

只有你移除它的那个作用范围会改变。停止管理某个 global server，不会影响项目中同名的
server，反之亦然。若要重新管理某个条目，请 import 它。

## Skillshare 未管理的 server {#unmanaged-servers}

仪表盘会读取当前作用范围以及 `mcp.projects` 下每个文件夹的 Agent 配置文件，
查找此 source 未定义、且没有任何 Skillshare 配置在管理的 server。找到时，server 列表
上方会有一条提示，说明有多少个、位于哪些 Agent 中。**Import** 会打开导入，并预先选中
其中第一个 Agent。项目的 **MCP** 标签页会针对该项目自己的文件显示同样的提示；
它的导入会读取该项目的文件，并把 server 保存到该项目。没有可连接对象的条目
（例如 Goose 的内置扩展）不计入。

### 接管 Agent 已有的条目

当你以某个 Agent 文件已在使用的名称添加 server 时，sync 不会覆写该条目。
计划会报告冲突 `existing entry is not managed`，并且在你为该条目做出选择之前
不会写入任何文件：

- 从该 Agent 导入：`skillshare mcp import NAME --from CLIENT`，或仪表盘中该冲突的
  **Import from** 按钮，例如 **Import from Cursor**。与 source 相符的条目会按原样被接管。若 conflict 位于
  `mcp.projects` 下的文件夹，按钮会读取该文件夹的文件，并导入到那个 project。
- 用 source 定义替换它：仪表盘中的 **Replace with source**，或在 import 时使用
  `--replace`。

## 安全性与限制

- JSONC 注释和不相关的设置会被保留。发生变化的、由 Skillshare 拥有的条目
  会作为一个整体被替换，因此这些条目内部的注释可能会改变。只有
  Skillshare 写入的字段会被比较和替换；Agent 专属的字段，例如超时，会被
  保留。
  由 Agent 自动填充的默认值，例如 `"type": "stdio"`、空的 `env` 或
  header 名称大小写，不算作变更。用 `enabled: false` 或 `disabled: true`
  关闭一个被管理的 server 会被报告为冲突。
  Pi 是例外：只修改 `enabled` 不会造成所有权冲突；同步时仍以 source 的 `piOptions.enabled` 为准。
- 当 Agent 在同一文件中重写不相关的设置时，预览仍然有效，就像 Claude Code
  对 `~/.claude.json` 所做的那样。只有该文件的 MCP 条目发生变化才需要
  新的预览。
- Codex 和 Grok 的编辑支持普通的 `[mcp_servers.NAME]` 表及其子表。
  被更新的条目会保持原位，CRLF 换行符也会被保留。
  内联/点号形式的 MCP 定义必须先转换为表格形式才能写入；
  否则会被拒绝，且不会修改文件。
- 原生文件的 symlink、格式错误的文件以及重复的 JSON 属性会阻止
  写入。被 symlink 的 Skillshare `config.yaml` 会被写入其目标文件。文件权限会被保留；新的原生
  文件、所有权记录和备份使用私有权限。
- 已经与 source 匹配的条目会被报告为未变更且不会写入，例如在
  拉取队友的更改之后。如果该配置以前从未管理过它，例如从某个 Agent
  导入后才勾选该 Agent，计划会显示 `adopt`：sync 会把它记为受管理，但不改动文件；
  之后移除该 server 或取消勾选该 Agent 就会移除它。你在 Agent 里自己关闭的
  server 仍归你所有。一个不同的、无人管理的条目需要导入或显式的
  按条目替换；只要另一个 Skillshare 配置文件仍然存在，就不能覆盖它的
  所有权。如果该文件已经不在了，它就永远无法释放该条目，因此冲突会说明
  这是一个残留条目，并指出是哪个文件，然后在你执行显式的导入或替换时
  接管它；在终端中和在仪表盘的那条冲突上都可以这样做。只是读取不到的
  文件，例如位于未挂载的磁盘上，仍然算作拥有者还在。
- 仪表盘的 MCP 设置只有在浏览器通过 `localhost` 或 IP 地址打开
  仪表盘时才能工作。通过域名访问，包括反向
  代理，MCP 请求会返回 403，因为 DNS 重绑定攻击总是使用
  域名。
- 凭据使用环境引用；没有密钥存储、OAuth 会话同步、
  持续健康监控、软件包安装、gateway、注册表或插件同步。
  `mcp check --live` 是唯一会启动或调用 server 的命令。
- 本版本不支持 VS Code Insiders、自定义 profile、远程工作区和旧版 SSE。
- VS Code 目前不会在 `headers` 内部替换 `${env:VARIABLE}`
  （[microsoft/vscode#336232](https://github.com/microsoft/vscode/issues/336232)），
  因此同步到 VS Code 的 header 和 `bearerToken` 引用在该问题修复之前，
  到达 server 时都是未解析的原始文本。
- 本地操作记录保存在 Skillshare state 目录下的 `mcp/` 中：
  `state.json`、写入期间的 `pending.json`，以及 `backups/`（每个
  Agent 文件保留最新的 20 份）。请勿将此
  目录作为可移植的清单来共享。


## 工具策略 {#tool-policy}

`tools` 决定一个 server 的哪些工具会提供给模型。只需在 server 上写一次；
同步时 Skillshare 会把它转换成各 Agent 自己的字段。

```yaml
mcp:
  servers:
    github:
      command: github-mcp
      targets: [pi, codex, copilot, opencode]
      tools:
        allow: [get_*, search_code, list_issues]
        deny: [get_secret]
```

```bash
skillshare mcp add github --target pi --target codex --tools-allow 'get_*,search_code' --tools-deny get_secret -- github-mcp
skillshare mcp edit github --tools-allow ''          # clear the allow list
skillshare mcp import github --from claude --target pi --tools-deny get_secret
```

| 字段 | 含义 |
|---|---|
| `allow` | 设置后，只保留匹配的工具 |
| `deny` | 移除匹配的工具，即使 `allow` 也匹配它们 |

`allow` 和 `deny` 中的条目是工具名称，其中 `*` 匹配任意字符。其他
通配符（`? [ ] { }`）、空格和逗号都会被拒绝，重复列出的名称也一样。
如果 `deny` 列表移除了 `allow` 保留的所有工具，就会报错。`disabled` 条目不能设置
`tools`。这两个标志可用于 `mcp add`、`mcp edit` 和 `mcp import`；列表用逗号分隔，
空值会清除对应的部分。Pi 如何提供工具不属于策略：那是 Pi 的 `exposure`，在
[`piOptions`](#pi-options) 中设置。

### 各 Agent 收到的内容 {#tool-policy-agents}

并非每个 Agent 都能容纳策略的每个部分。Skillshare 会写入该 Agent 有文档说明的
格式所支持的内容，并指出其余部分；它绝不会静默丢弃任何部分。

| Agent | 写入的内容 | 不会应用 |
|---|---|---|
| [Pi](https://github.com/earendil-works/pi/blob/v0.99.0/packages/coding-agent/docs/mcp.md) | `toolExposure` 中先写入被拒绝的工具（`hidden`），再写入允许的工具，设置了 `allow` 时最后写入 `"*": "hidden"` | 无 |
| [Codex](https://developers.openai.com/codex/config-reference) | `enabled_tools` 和 `disabled_tools`，只写精确名称。Codex 会在 `enabled_tools` 之后应用 `disabled_tools` | `allow` 中的 `*` 通配符；`deny` 中无法折叠进精确 `allow` 列表的 `*` 通配符 |
| [Copilot CLI](https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/add-mcp-servers) | `tools`：允许的精确名称减去被拒绝的名称，否则为 `["*"]` | `allow` 中的 `*` 通配符；当 `allow` 没有列出精确名称时的 `deny`，因为 Copilot 没有拒绝列表 |
| [OpenCode](https://opencode.ai/docs/permissions/)、[Kilo Code](https://kilo.ai/docs/code-with-ai/platforms/cli#permissions) | 无 | 全部。两者只在顶层 `permission` 映射中过滤工具，其 key 为 `<server>_<tool>`，位于该 server 条目之外 |
| 其他所有 Agent | 无 | 全部 |

在 Pi 中，精确的工具名称优先于任何通配符，因此被某个拒绝通配符匹配到的允许精确名称
不会写入 `toolExposure`。允许的工具会使用 server 的 `piOptions.exposure`；当它未设置或为
`hidden` 时，则使用 Pi 的默认值 `codemode`：因此 `hidden` 搭配 `allow` 表示只有
允许的工具可见。

未应用的部分会出现在三个地方：

- 同步计划中，每个 Agent 一行 warning，列出相关 server：

  ```text
  ! tool policy not applied for opencode: allow, deny (github)
  ```

  使用 `--json` 时，同样的文字位于计划的 `notices` 中。
- [`mcp check`](#check-servers-before-an-agent-starts-them) 中，每个 Agent 一条 `tools`
  warning。
- 仪表盘中，位于 server 对话框的工具区块和 **查看各 Agent 会写入的配置** 中。
  仪表盘不会为这些情况显示页面级提示，下文已停用的 Pi 设置也一样。

Codex 的 `enabled_tools` 和 `disabled_tools` 是受管理字段：清除策略会移除它们，
在 Skillshare 拥有的条目中手动编辑它们会显示为冲突。导入时会把 Codex 的
`enabled_tools`/`disabled_tools` 和 Copilot 的 `tools` 读回 `tools`。只有当搭配 server 的
`exposure` 写入该策略能得到完全相同的 `toolExposure` 时，Pi 的 `toolExposure` 才会变成
`tools`；否则它会保留在 `piOptions` 中，并给出 warning。`exposure` 始终保留在 `piOptions`。

### 仪表盘中的工具设置 {#tool-policy-dashboard}

server 对话框在 targets 之后有一个 **工具** 区块，除 `disabled` 条目外每个 server 都有。
这个区块始终显示。标题旁的信息图标说明这个区块，摘要显示 `全部工具`、策略内容
（例如 `只允许 1 个，排除 2 个`），或加载工具后的 `已选 9 / 14`。

- 标题下方的方框放工具列表。尚未加载时提供 **加载工具**，它会用对话框当前的设置（无论是否
  已保存）启动 server 一次，与 [`mcp check --live`](#probe-servers-live) 是同一种探测。它只在
  点击时运行，也不会保存任何内容，所以新的 server 也能使用。失败时会用通俗的话说明原因，原始错误
  放在旁边信息图标的提示里，按钮则变成 **重试**。之后修改命令、网址或相关设置，已加载的列表会被清除。
- 加载后每个工具都有一个复选框，勾选的工具才会给模型使用。取消勾选会把该工具的完整名称加入
  `deny`；重新勾选会把它从 `deny` 移除，如果非空的 `allow` 仍排除它，则把名称加入 `allow`。
  被 `deny` 通配符排除的工具无法勾选，提示会指出是哪条规则。搜索框可以筛选列表，**全选** 和
  **全不选** 只作用于当前显示的行，刷新按钮会再加载一次列表。
- 方框底部的 **排除规则** 行用于输入 `*` 通配符，以及 server 没有列出的名称：输入一个后按
  Enter。`allow` 有条目时，上方另有一行 **只允许**，用法相同。加载工具前，所有已保存的条目都
  显示在这里。无效的名称，或移除所有允许工具的拒绝列表，会显示在对话框中并阻止 **保存**。
- 再往下，对话框会说明每个所选 Agent 实际会得到什么：哪些会按这份列表提供工具、只应用一部分的
  Agent 会怎么做（例如 Copilot CLI 没有拒绝列表，仍会提供取消勾选的工具），以及哪些不支持筛选。

server 行会用白话显示策略标签（例如 `工具：已排除 2 个工具`），**查看各 Agent 会写入的配置** 会按 Agent 提示它不会应用的部分。

## Pi {#pi}

Pi ≥ 0.99.0 已[内置 MCP](https://github.com/earendil-works/pi/blob/v0.99.0/packages/coding-agent/docs/mcp.md)，
这也是 Skillshare 为 Pi 写入 MCP server 的唯一方式。第三方的
`pi-mcp-adapter` 和 `pi-mcp-extension` 不再作为同步目标位置受支持。

| 作用域 | 文件 |
|---|---|
| Global | `~/.pi/agent/mcp.json`（遵循 `PI_CODING_AGENT_DIR`） |
| Project | `.pi/mcp.json` |

个人 server 以及含凭据的 server 请放入 `~/.pi/agent/mcp.json`。仅在可信项目中，
将项目需要的 server 放入 `.pi/mcp.json`。同名的 project 条目会完整替换 global 条目。
Skillshare 直接编辑文件，提供预览和备份；它不会信任项目、启动 server、安装
扩展，也不会授权 OAuth。

```bash
skillshare mcp add docs --url https://example.com/mcp --target pi --tools-deny 'delete_*' --pi-options '{"exposure":"deferred","timeout":120}' --no-tui
skillshare sync mcp --dry-run
skillshare sync mcp
```

```yaml
mcp:
  servers:
    docs:
      url: https://example.com/mcp
      targets: [pi]
      tools:
        deny: [delete_*]
      piOptions:
        exposure: deferred
        timeout: 120
```

原生输出使用 `command`/`args` 或 `url`，以及 `${NAME}` 环境引用。
同步后，在 Pi 中执行 `/reload` 或启动新会话，并用 `/mcp` 检查连接及授权 OAuth。
简单的 Pi 专用配置可以用 `pi mcp add`，它会编辑 global 文件；加上 `-l` 则写入项目。
`pi mcp list` 会启动每个已启用的 server 来检查连接；`pi mcp login NAME` 需要用户批准。

Pi 的 server 名称只允许字母、数字、`_` 和 `-`；只差在 `-` 和 `_` 的名称会被 Pi
视为同一个 server，因此同步会拒绝第二个。Pi 的项目条目会整条替换 global
中的同名条目；要在单个项目中关闭 global server，请参阅
[在单个项目中关闭一个 global server](#turn-off-a-global-server-in-one-project)。

Pi 1.0.1 起，Pi 的 `/mcp` 可以在项目中添加只有 `enabled`、`exposure` 或 `toolExposure` 的条目，
用来覆盖同名的 global server。它不是 server，所以导入会跳过它。如果项目定义了同名的 server，
同步会报告冲突，直到你替换该条目，或在 Pi 中移除这个覆盖。

### 其他 Pi 设置 {#pi-options}

`piOptions` 保存 Pi 内置 MCP 的其他单个 server 字段。只有 Pi 会收到它们。

- `exposure` 接受 `codemode`（Pi 默认）、`codemode-deferred`（`codemode` 的旧名称）、
  `deferred`、`direct` 或 `hidden`。`toolExposure` 把工具名称或通配符映射到上述某个值：精确名称优先，
  其次是第一个匹配的通配符。导入和 JSON/YAML 转换会保留通配符的顺序。`exposure` 也决定
  [`tools`](#tool-policy) 允许列表保留的工具如何提供。更推荐用 `tools` 代替 `toolExposure`，
  因为它也能作用于其他 Agent；同一个 server 不能同时设置 `tools` 和 `toolExposure`。
- `timeout`（正数秒）、`cwd`、`enabled`、`oauth` 和 `auth` 会被验证。`description`
  等未知字段会原样传递。
- `auth: {provider: NAME}` 会把该 provider 的 `/login` token 作为 bearer token 发送。
  它需要 https 的 `url`（localhost 可用 http），且只能在 global 模式使用，因为 Pi 只从
  global 文件读取它。
- `oauth.authServerMetadataUrl`（Pi 1.0 及以上）必须使用 https（localhost 可用 http），因为 Pi
  会直接信任这份文档，不再自动发现。Pi 1.0 按 server 名称和 URL 保存 OAuth 登录，所以
  重命名 server 或修改它的 `url` 后，需要在 Pi 重新登录。
- 连接字段请使用主表单。`directTools`、`includeTools`、`excludeTools` 以及
  `pi-mcp-adapter` 的其他设置会被拒绝，因为 Pi 的内置 MCP 不会读取它们；请改用 `tools`。
- 顶层 `settings` 和 `autoEnableCodemode` 不是 server 选项：请直接在 Pi 中编辑，
  同步会保留它们。
- 凭据请使用环境引用。可移植的 env/headers 中的 `!command` 字面值会被拒绝，
  `piOptions` 中任何位置的命令值也一样，包括 `oauth.clientId` 这类非 secret 字段。

清空 JSON，或从中移除某个字段时，如果该字段由 Skillshare 写入且未被修改，下次同步
就会把它从 Pi 的文件中移除。你自己在 Pi 中添加的字段会保留。Skillshare 写入、
之后又在 Pi 中被修改的字段会阻止同步，直到你导入它。

```bash
skillshare mcp edit docs --pi-options '{"timeout":60}' --no-tui
skillshare mcp edit docs --pi-options '{}' --no-tui
```

在仪表盘中，server 对话框的 Pi 区块有 **工具暴露模式** 和 **其他 Pi 设置**。
**Pi 设置** 和 **工具暴露模式** 旁边的信息图标会解释它们，**Pi 设置** 旁边的链接
会打开 Pi 的 MCP 文档。对话框会在你保存之前标出 **其他 Pi 设置** 中的
`pi-mcp-adapter` 字段。工具区块有设置时，**工具暴露模式** 仍可编辑；此时只有
**其他 Pi 设置** 中的 `toolExposure` 会被拒绝，因为它由 `tools` 写入。server 行上的 Pi 标签
会用简短文字显示暴露模式，例如 `codemode` 显示为 `通过代码`。

### 从 0.22 升级 Pi {#pi-migration}

升级后第一次同步之前，请先在 Pi 中确认两件事：

- **Pi 0.99.0 或更高版本。** Skillshare 现在只把 Pi 的 server 写入 `mcp.json`，由 Pi 在
  0.99.0 加入的内置 MCP 读取。较旧的 Pi 不会读取这个文件，因此在更新 Pi 之前，这些 server
  都不会加载。Skillshare 不会检查 Pi 的版本。
- **如果 Pi 仍安装着 `pi-mcp-adapter` 或 `pi-mcp-extension`，请将其移除。** Pi 的
  [MCP 文档](https://github.com/earendil-works/pi/blob/v0.99.0/packages/coding-agent/docs/mcp.md)
  说明，已安装且注册了 `/mcp` 的 extension 会取代内置 MCP。`pi-mcp-extension` 自身也会读取
  `mcp.json`；`pi-mcp-adapter` 从 3.0.0 起不再读取它，所以 Skillshare 迁移过去的 server 不会
  再通过 adapter 加载。

当同步把 server 从这两个 extension 迁走时，`sync mcp --dry-run`、`sync mcp` 和 `--json`
会提示一次：

```text
! Pi's built-in MCP needs Pi 0.99.0 or later; on older Pi these servers stop loading until Pi is updated. If pi-mcp-adapter or pi-mcp-extension is still installed in Pi, remove it, because it can take the place of Pi's built-in MCP
```

以下情况会出现这条提示：同步移除 Skillshare 写在 `mcp-adapter.json` 中的条目、改写它为
`pi-mcp-extension` 写的条目，或发现只有这两个 extension 会读取的设置（`piExtension:
pi-mcp-adapter` 或 `pi-mcp-extension`、`directTools`，以及下方列出的 `piOptions` 字段）。
那次同步之后就不会再出现。

0.23.0 移除了 Pi 模式选择（`piExtension`：`builtin`、`pi-mcp-adapter`、
`pi-mcp-extension`）、`piOptionsPrune` 开关和 `directTools`。旧的配置仍然可以加载。
`sync mcp --dry-run` 和 `sync mcp` 会为发现的每一类已停用设置打印一条 warning，
并列出相关 server，例如：

```text
! Pi now uses its built-in MCP; the next sync updates the config: context7, local (shop)
```

只在 `mcp.projects` 下某个项目中找到的 server，会在括号中显示该项目文件夹。

下一次同步会做的事：

| 0.23.0 之前 | 同步之后 |
|---|---|
| `piExtension: builtin` | 移除该 key；其他不变 |
| `piExtension: pi-mcp-extension` | 移除该 key。该条目原本就在 `mcp.json` 中，因此会在那里以内置格式重写 |
| `piExtension: pi-mcp-adapter` | 移除该 key。server 会写入 `mcp.json`，Skillshare 在 `mcp-adapter.json` 中写入的条目会被移除。你自己添加到 `mcp-adapter.json` 的条目保持原样 |
| `piOptionsPrune` | 移除该 key。同步总会移除由 Skillshare 写入且未被修改、已被清除的字段（[见上文](#pi-options)） |
| server 上的 `directTools` | `true` → `piOptions.exposure: direct`；`"search"` → `deferred`；名称列表 → `piOptions.toolExposure`，其中这些工具为 `direct` |
| `mcp.directTools`，或 `mcp.projects` 下某个项目的 `directTools` | 该默认值会写入每个送往 Pi 且没有自身值的 server，转换方式同上。项目的 `false` 会覆盖 global 值 |
| `piOptions.includeTools` / `excludeTools` | `tools.allow` / `tools.deny`；同时设置的 `directTools` 仍会转为 `piOptions.exposure` |
| `piOptions` 中其他 `pi-mcp-adapter` 字段：`approveTools`、字符串形式的 `auth`（Pi 自己的 `auth` 对象会保留）、`bearerToken`、`bearerTokenEnv`、`bearerTokenStore`、`caFile`、`debug`、`exposeResources`、`idleTimeout`、`inheritEnv`、`lifecycle`、`protocolVersion`、`requestHeadersCommand`、`requestTimeoutMs`、`searchKeywords`、`socket`、`tasks`、`toolPrefix`、`trace` | 移除，因为 Pi 的内置 MCP 不会读取它们 |

如果 `directTools`、`includeTools` 或 `excludeTools` 会覆盖该 server 已设置的
exposure，或者不是工具名称列表，就会被丢弃，并给出单独的 warning。

第一次应用这些更改的同步，还会保存一份不含已停用设置的 Skillshare 配置：
`config.yaml`，或 `sources.mcp` 指定的文件。写入之前，它会把旧文件以原因 `migrate`
保存到[文件历史](/docs/reference/commands/backup#file-history)中，并为每个文件打印一行：

```text
→ Updated config.yaml for 0.23.0 (backup: <path of the saved version>)
```

这会在 `skillshare sync mcp`、`skillshare sync --all`（即使没有 Agent 文件发生变化）
以及仪表盘的同步中发生。`--dry-run` 和预览不会写入任何内容。如果保存配置失败，
Agent 文件已经写入，而配置保持原样；错误信息会说明这一点，下一次同步会再试一次。
保存成功后，这些 warning 就会消失。

已移除的标志现在会失败并给出提示：

| 标志 | 改用什么 |
|---|---|
| `--pi-extension` | 直接去掉。Pi 始终使用它的内置 MCP |
| `--pi-options-prune` | 直接去掉。同步总会移除 Skillshare 先前写入且未被修改的字段 |
| `--direct-tools` | 对所有工具用 `--pi-options '{"exposure":"direct"}'`，或对 Pi 中的单个工具用 `--pi-options '{"toolExposure":{"TOOL":"direct"}}'` |

`skillshare mcp import --from pi` 仍会读取 `pi-mcp-adapter` 的 `mcp-adapter.json`
（位于 Pi 的 `mcp.json` 旁边），方便你把 server 迁移过来。当两个文件都定义了某个
server 时，以 `mcp.json` 为准。同步会把该 server 写入 Pi 的 `mcp.json`；在
`mcp-adapter.json` 中只会移除它在 0.23.0 之前写入的条目。其中的 `directTools`、`includeTools` 和
`excludeTools` 会按上述方式转换，其他 adapter 专用字段会被省略并给出 warning。
在仪表盘中，**从目标导入** 会把这两个 Pi 文件列为不同的来源。
