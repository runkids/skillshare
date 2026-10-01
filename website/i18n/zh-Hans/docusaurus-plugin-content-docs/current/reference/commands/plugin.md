---
sidebar_position: 4
---

# plugin

在受支持的工具间管理完整的原生 plugins。Capability 检查会区分安装支持与格式发现。可以从
[在多个工具间共享 plugins](/docs/how-to/daily-tasks/sharing-plugins) 开始。

```bash
skillshare plugin                         # Interactive manager
skillshare plugin add                     # Source → plugin → targets → review
skillshare plugin discover ./my-plugin --json
skillshare plugin add ./my-plugin --target claude --target codex --no-tui
skillshare plugin add ./my-plugin --no-tui   # 先交给 Skillshare 管理，之后再选 target
skillshare plugin import review@team --from claude --no-tui
skillshare plugin list --json
skillshare plugin inspect review --json
skillshare plugin disable review --target codex --no-tui
skillshare sync plugins --dry-run --json
skillshare sync plugins --no-tui
skillshare plugin enable review --target codex --no-tui
skillshare plugin check review --json
skillshare plugin update review --target claude --no-tui
skillshare plugin remove review --no-tui
```

`enable` 和 `disable` 改变的是 **Skillshare 的同步选择**，而不是 Agent 的原生启用状态。取消选择某个
target 会保存该选择。下一次 `sync plugins` 会移除其受管理的安装，但保留定义。再次选择它
则允许下一次 sync 重新安装。未受管理的 plugins 不受影响。

`add` 不带 `--target`（或在交互选单中什么都不选）会把 plugin 交给 Skillshare 管理，但不安装到任何地方。之后可以用相同的 source 与 `--name` 加上 target，或在 dashboard 中从该 plugin 的行勾选；安装的是当时的 source 内容。这类 plugin 在最后一个 target 被移除后仍会保留。不带 `--target` 的 `remove NAME` 会把它从 Skillshare 移除。

## 命令

| Command | Behavior |
|---|---|
| `list` | Configured bindings and native installation state; interactive manager in a terminal |
| `discover SOURCE` | Inspect a local directory, `owner/repo`, or HTTPS Git repository |
| `add [SOURCE]` | Choose and install a whole plugin with its target adapter |
| `import [NATIVE-ID]` | Adopt an existing installation without reinstalling or enabling it |
| `inspect NAME` | Inspect one managed package |
| `sync [NAME]` | Reconcile selected targets and retry incomplete native operations |
| `check [NAME]` | Compare source content with the recorded digest; never update |
| `update [NAME]` | Review source changes and use a supported native update operation |
| `enable / disable [NAME]` | Include/exclude a target in the next sync |
| `remove [NAME]` | Uninstall managed bindings and remove their definitions |

裸命令会在终端中提示缺失的输入。非交互式的变更命令需要显式输入。
`sync` 和 `check` 可以对所有 packages 进行操作。`sync --all` **不**包含 plugins；请显式使用
`sync plugins`。

## 选项

| Option | Meaning |
|---|---|
| `--target TARGET` | Repeatable selection: `claude`, `codex`, `cursor`, `antigravity` (`agy` alias), `antigravity-cli`, `copilot`, `grok`, `pi`, `opencode`, or the name of [another account of an Agent](#accounts); see the capability table below |
| `--plugin NAME` | Select one plugin from a source marketplace |
| `--name NAME` | Logical package name when adding or importing |
| `--from TARGET` | Import from Claude, Codex, Antigravity CLI, Copilot, Grok, Pi, OpenCode, or [another account of an Agent](#accounts) |
| `--dry-run`, `-n` | Preview without changing Skillshare or Agent configuration |
| `--source-ref REF` | Git branch, tag, or commit for `discover`, `add`, and `update`; remote sources only |
| `--entry PATH` | Explicit built OpenCode JS/TS entry, relative to the package root (`discover` and `add`) |
| `--revision ID` | Refuse application if source, configuration, or native inventory changed since preview |
| `--json` | Machine-readable output; disables TUI |
| `--no-tui` | Disable interactive menus; also respects `tui: false` |
| `--global`, `-g` | Global Skillshare config and native user scope |
| `--project`, `-p` | Project config; Claude, Antigravity, Pi, or OpenCode (never global fallback) |

JSON 输出包含来源路径和原生标识符。不要在来源 URL 中放置凭据。一次失败的变更操作仍可能
对部分 targets 返回成功结果；当任何 target 失败时，CLI 会以非零代码退出。重试之前请先
检查结果。

## Target 支持

| Target | Format | Global | Project | Update |
|---|---|:---:|:---:|---|
| Claude Code | `.claude-plugin/plugin.json` | Yes | Yes | Native update |
| Codex | `.codex-plugin/plugin.json` or Agent Plugins root manifest | Yes | No | 已验证的原生重新安装 |
| Cursor | `.cursor-plugin/plugin.json` or Agent Plugins root manifest | Yes | No | Replace reviewed local copy |
| Antigravity Desktop | Root `plugin.json` with an explicit name | Yes | Yes | Replace reviewed local copy |
| Pi | `package.json` with `pi` resources, or `pi-package` keyword and conventional resource folders | Yes | Yes, with native project trust | Refresh managed source snapshot |
| OpenCode | `package.json` with an SDK dependency, `.opencode/plugins/` entry, or explicit `--entry` | Yes | Yes | Refresh managed source snapshot |

| Antigravity CLI | Native root manifest or Claude manifest accepted by `agy` | Yes | No | Update natively to preserve enablement |
| GitHub Copilot CLI | `.plugin/plugin.json`, `.github/plugin/plugin.json`, Claude manifest, or Agent Plugins root manifest | Yes | No | Refresh reviewed source, only if native enabled state is known and enabled |
| Grok Build | `.grok-plugin/plugin.json` or Claude manifest | Import/remove only; native trust required for install | No | Update natively |
| Kimi Code | `kimi.plugin.json` or `.kimi-plugin/plugin.json` | Discovery only | No | Not automated |
| Hermes | `.hermes-plugin/plugin.yaml` | Discovery only | No | Not automated |
| Devin | `.devin-plugin/plugin.json` | Discovery only | No | Not automated |

Kimi 的非交互式生命周期、Hermes 的 profile 清单/同意流程，以及 Devin 的本地清单/信任/云端区分，
这些适配器尚未验证。它们的格式会在 discovery 阶段展示，但安装会被禁用并给出原因。来源声明
支持某个 target 并不意味着 Skillshare 就能管理它。`list --json` 和 `discover --json` 包含
`targetDefinitions`，列出允许的操作；discovery 还会为每种格式暴露 `targetInfo`，包含版本、
组件、entry 和验证问题。损坏的 manifest 只会隔离到其对应的 target；格式错误的 catalog
会以警告形式报告，而不会隐藏有效的格式。

### 某个 Agent 的另一个账号 {#accounts}

声明为[某个 Agent 的另一个账号](/docs/reference/targets/configuration#agent-config-dir)的 target 同样是一个 plugin target，适用于 `claude`、`codex` 和 `pi`。Skillshare 会通过 `CLAUDE_CONFIG_DIR`、`CODEX_HOME` 或 `PI_CODING_AGENT_DIR`，针对该账号的配置目录运行该 Agent 自己的 CLI：

```yaml
targets:
  claude-work:
    agent: claude
    config_dir: ~/.claude-work
```

```bash
skillshare plugin add owner/repo --target claude-work
skillshare plugin import demo@market --from claude-work
```

设置了 [`cli`](/docs/reference/targets/configuration#agent-config-dir) 的账号会改用那个兼容的 CLI，例如 Pi 账号用 `omo`，并针对同一个配置目录运行。Pi 账号还会设置 `SENPI_CODING_AGENT_DIR` 和 `OMO_CODING_AGENT_DIR`，因为 Pi 的 fork 会先读这两个变量，再读 `PI_CODING_AGENT_DIR`。找不到 CLI 时操作会失败；Skillshare 不会改用 Agent 本身的 CLI。

这个账号会沿用它所属 Agent 的各项操作，并以自己的名称保存自己的绑定，因此一个 plugin 可以只安装在其中一个账号而不装在另一个账号。账号只存在于 global 作用域：项目的 plugins 属于该项目，而不属于某一个账号。`--target` 和 `--from` 都接受账号名称，终端选择器和仪表盘的 Plugins 页面也会把它列在各个 Agent 旁边。

### Cursor 与 Antigravity

这些适配器会将整个 plugin 复制到有文档记录的本地发现目录中。它们不需要 CLI 可执行文件，
也不会修改 marketplace 注册表：

- Cursor：`~/.cursor/plugins/local/<name>`；必须允许本地导入。重新加载
  Cursor 并检查 Customize。已安装的同名 marketplace plugin 优先于本地副本。
- Antigravity desktop：全局为 `~/.gemini/config/plugins/<name>`；在工作区中为 `.agents/plugins/<name>`
  （或已存在的 `_agents/plugins/` 目录）。如果两个工作区目录都存在，请先合并它们。
- 独立的 **agy CLI** 有单独的 plugin store。`antigravity` target 管理的是
  desktop/workspace 的发现路径，而不是该 CLI store。`agy` 只是 Skillshare target 的一个别名；
  使用 `--target antigravity-cli` 表示独立 CLI，`--target agy` 保留其原有的 desktop 含义。

对这些本地 packages 使用 `plugin add`。不支持导入预先存在的本地文件夹或
marketplace 安装。Skillshare 拒绝覆盖非自身拥有的文件夹、symlinks 或本地已编辑的受管理内容。
显式的 Antigravity manifest 名称可以在 Git 检出和快照之间保持身份稳定。

### Pi 与 OpenCode

Pi 使用 `pi install` / `pi remove`；清单读取有文档记录的 package 设置，
而不加载 extension 代码。`PI_CODING_AGENT_DIR` 会被遵循。Pi 的 project trust 必须在 Pi 中建立；
Skillshare 不会替你传递 `--approve`。

OpenCode 会在 `opencode.json` 或已存在的 `opencode.jsonc` 中将受管理的 entry 注册为文件 URL，
并保留注释和无关的条目。Version 1 使用 `plugin`；version 2 使用 `plugins`。`XDG_CONFIG_HOME`
和绝对路径的全局 `OPENCODE_CONFIG` 会被遵循；含糊或不受支持的覆盖会被拒绝。
OpenCode 必须在 PATH 上，以便 Skillshare 能够选择对应版本的 schema。

本地 OpenCode source 必须已经包含其构建好的 entry（`main`、字符串形式的 root export，
或 `index.js`）以及所需的运行时依赖。Skillshare 不会运行构建脚本，也不会将依赖安装到 source 中。
注册成功并不代表模块已成功加载；请在重新加载后检查 OpenCode。

Import 接受普通的 Pi package 来源和普通的 OpenCode 配置条目。带有资源过滤器/选项的
条目会被拒绝，以保留这些设置。已导入的 Pi package 在全局模式下用 `pi update SOURCE` 更新，并保留其设置条目；project 中的则要在 Pi 里更新，因为 `pi update` 也会影响全局 package。已导入的 OpenCode v1 packages 会在其原生工具中更新。
OpenCode v2 的全局导入可以使用其原生的更新命令；project 导入则必须原生更新，
因为 v2 的更新命令是全局的。

```bash
skillshare plugin add ./cursor-plugin --target cursor --no-tui
skillshare plugin add ./agy-plugin --target agy --no-tui -p
skillshare plugin add ./pi-package --target pi --no-tui
skillshare plugin add ./opencode-package --target opencode --no-tui
skillshare plugin import npm:my-pi-package --from pi --name my-package --no-tui
```

## Refs 与显式 entries

**Add plugin** 中的高级选项接受可选的 Git ref 和 OpenCode entry。留空即可使用正常的引导流程。
终端向导接受相同的 flags；自动化场景可以使用：

```bash
skillshare plugin discover obra/superpowers --source-ref v6.3.0 --json
skillshare plugin add owner/repo --source-ref v1.0.0 --target copilot --no-tui
skillshare plugin add ./package --entry dist/plugin.js --target opencode --no-tui
skillshare plugin update review --source-ref v1.1.0 --target claude --dry-run --json
```

Bindings 会记录 `source_ref` 和解析出的 `commit`。安装使用的是已审阅的 commit；`check` 和 `update`
会重新解析配置的 ref，因此分支可以继续推进，而 commit 则保持锁定。`--revision` 是一个预览
token，而不是 Git ref。`--entry` 是相对于每个候选 package root 的路径，且必须已存在；
它不会触发构建或 package manager 安装。

Copilot 和 Antigravity CLI 的安装使用已审阅的本地快照。Imports 没有可供重新安装的已审阅来源：
移除后，需要在原生客户端中安装，然后再次 sync。Grok 也需要在安装/重新安装前建立原生信任。
Skillshare 从不提供原生信任的批准 flags。

## 兼容性与边界

- Claude 需要其原生的 `.claude-plugin/plugin.json` package。
- Codex 接受 `.codex-plugin/plugin.json` 以及可识别的、便携的 root
  `plugin.json` packages。仅限 Claude 的 package 不会被静默转换。
- Sources 可能包含带有本地 plugin 条目的 marketplace。外部 catalog
  catalogs 会按 plugin 名称/路径合并。冲突的路径会被拒绝；外部
  条目会连同说明一起报告，提示直接添加其 repository 或原生安装后再导入。基于命令的 sources
  不会被自动批准。
- 完整的 source 快照会保留 plugin 脚本、资源和安全的相对
  symlinks（包括 `AGENTS.md → CLAUDE.md`）。绝对路径、逃逸路径、悬挂、
  循环以及引用 `.git` 的链接和特殊文件会被拒绝；sources 限制为 20,000 个文件和 100 MiB。
- 原生安装并不能证明运行时已激活。请重启/重新加载
  该 Agent，并在该 Agent 中完成认证或 hook trust。
- Codex 不支持原生 project 安装；全局更新使用已验证的原生重新安装流程。
- 导入 plugin 保留原始 marketplace 身份。Codex `check` 可检视注册的本地/Git 来源；其他没有可检视来源的导入无法推断新版本。
- 更新遇到无法处理的 target 时会跳过并说明原因；该 plugin 的其他 Agent 仍会照常更新，
  被跳过的更新会保留为待处理，留给之后的 sync。
- Removal 会保留共享的 marketplace 注册和受管理的快照；它
  不会直接删除无关的 plugins 或原生缓存。

原生生命周期已在 Claude Code `2.1.276`、Codex CLI
`0.154.0`、Pi `0.85.1` 和 Copilot CLI `1.0.86` 上验证。Antigravity CLI
`1.2.6` 通过隔离的原生 install/list/remove 操作进行了检查。OpenCode `1.18.31` 用于验证
版本感知的注册；v2 schema 由 fixture 测试覆盖。Cursor 和 Antigravity 的
文件系统生命周期在隔离目录中进行了测试，但不代表已验证 GUI 运行时激活。已安装的命令
capabilities 和清单 schemas 会在运行时检查；不受支持的操作会被阻止并给出说明。

## Codex 更新

全局 Codex 更新支持受管理来源，以及具有可审阅本地或 Git marketplace 来源的导入 plugin。先使用 `skillshare plugin update review --target codex --dry-run --json -g`，再以 `--no-tui` 应用。预览包含新旧版本、原生操作、启用状态和受影响的 plugin。原生 `plugin add` 使用已审阅目录的私有副本；导入 Git 来源先执行指定 marketplace 的 `plugin marketplace upgrade NAME --json`。这可能暂时重新安装其他 plugin，但完成前会恢复其缓存内容、权限和原本不存在的状态。后续原生更新仍由 Codex 控制。

更新保留启用状态及原生配置的完整字节，并验证安装版本后才记录成功。失败时恢复原有缓存和 Git marketplace；遇到并发修改冲突时保留私有恢复目录并报告路径。锁只协调 Skillshare，请避免同时写入相同原生目录。账号 `config_dir` 优先于 `CODEX_HOME`，再使用 `~/.codex`。导入 Git ref 由原生配置决定，不能用 `--source-ref` 覆盖。未知状态、缺失的安装缓存、无版本或不支持的来源，以及不安全的恢复文件结构会阻止更新。信任、认证及安装策略仍由 Codex 执行。此流程已在 Codex CLI `0.159.3` 验证。

原生 Codex 会改写 inline TOML plugin 表格，因此预览会阻止此格式；请先使用普通或 dotted 表格。

## 官方格式参考

- [Cursor local plugins](https://prod.cursor.com/docs/plugins)
- [Antigravity desktop plugins](https://www.antigravity.google/docs/plugins)
- [Antigravity standalone CLI plugins](https://www.antigravity.google/docs/cli/plugins)
- [Pi packages](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/packages.md)
- [OpenCode v1 plugins](https://opencode.ai/docs/plugins/)
- [OpenCode v2 plugins](https://opencode.ai/v2/docs/plugins)
