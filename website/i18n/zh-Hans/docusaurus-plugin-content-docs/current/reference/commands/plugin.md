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
| `add [SOURCE]` | Choose and install a whole plugin with its target adapter, or an `npm:` package through Pi |
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
| Codex | `.codex-plugin/plugin.json` or Agent Plugins root manifest | Yes | No | 刷新经审核的来源后再次 add，仅限在 Codex 中为启用状态 |
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

Pi 保留同一包的第一条全局注册与最后一条 project 注册。如果更早的全局来源或更晚的 project 来源无法确认 identity，Skillshare 无法证明哪条注册具有优先权，因此可能被覆盖的条目（包括继承的 project delta）会保持 Unknown／只读，不会通过删除 URL query 来猜测 identity。不受这项不确定性影响、已确认的条目仍可编辑。

Pi 使用 `pi install` / `pi remove`；清单读取有文档记录的 package 设置，
而不加载 extension 代码。`PI_CODING_AGENT_DIR` 会被遵循。Pi 的 project trust 必须在 Pi 中建立；
Skillshare 不会替你传递 `--approve`。

#### pi.dev 的 npm 包

`plugin add npm:<包>` 会通过 Pi 本身安装发布在 npm 上的包，例如 [pi.dev](https://pi.dev/packages) 列出的包：

```bash
skillshare plugin add npm:@scope/package --target pi --dry-run --json -g
skillshare plugin add npm:@scope/package@1.2.0 --target pi --no-tui -g
```

Pi 会下载包并运行它的 install script，Skillshare 无法事先检查内容；添加前请先在 pi.dev 或 npm 确认包。`discover` 不接受 npm 来源，npm 来源也不接受 `--source-ref`、`--entry` 或 `--plugin`。只有 Pi target 能接受 npm 来源，包括运行 `pi` 的 Pi 账号；运行其他可执行文件的账号，请用那个可执行文件安装后再导入。搭配 `--project` 时，Pi 会把包装进项目的设置；项目一旦有 `.pi` 文件夹，要先在 Pi 信任这个项目，Pi 才会修改它的包。

Pi 每个包名只保留一条。Pi 已经有相同来源时，`add` 会导入它；同一包的其他版本则会安装，由 Pi 替换那一条的来源。`update` 会运行 `pi update`，但固定在精确版本的包 Pi 会保持原版本，请改用新版本重新添加。如果你关闭了包里的某些 extension，Pi 会把这些规则保留到新版本，Skillshare 也会重新记录，之后重装时会还原。已经由另一个 Skillshare 包管理的 Pi 包会被拒绝，请改为更新或移除那一个。

在 dashboard 的添加对话框，可以直接粘贴 `pi install npm:<包>` 命令或包的 pi.dev 网址，两者都会转成对应的 `npm:` 来源。

#### 选择 package 的 extension

在 dashboard 中，`pi` 和 Pi 账号的 target 页面有一个 **Extensions** 标签页。它列出该 target 的 `settings.json` 中每个包条目，以及其过滤规则选中的 extension。开关会在该条目的 `extensions` 列表中写入一条精确的 `+path` 或 `-path` 规则。**Remove rule** 会删除该文件的精确规则（无论写成相对路径还是绝对路径），之后该文件由其余规则决定；结果会显示在预览中。应用前一定会先显示预览，并且只修改这些列表：条目的其他键、`skills`、`prompts` 和 `themes` 过滤规则、glob 和 `!` 规则，以及文件的其余部分都保持原样。字符串条目会变成 `{"source": ...}`，以便放入规则。对于字符串条目，Pi 只从包的 `pi` manifest 读取 skills、prompts 和 themes；对象条目则会在 manifest 没有列出时，从包的 `skills`、`prompts`、`themes` 文件夹加载它们。有这类文件夹的包，其字符串条目是只读的，因为 Skillshare 无法确认转换后这些资源保持原样。指向单个文件的来源也是只读的，因为 Pi 会直接加载它并忽略过滤规则。如果预览后文件已被修改，或 Pi 正持有设置锁，就不会写入任何内容。写入期间 Skillshare 会以与 Pi 相同的方式持有这个锁，一旦失去就不写入。每次应用前都会保存一份变更的 extension 列表及文件变更前后哈希的记录。成功应用的记录会保留，不会自动清理；应用失败时只移除该次新建的记录。这不是 `settings.json` 的副本，无法用来恢复整份设置文件。这个标签页会列出设置中的所有包，包括直接用 Pi 安装的，例如 [pi.dev](https://pi.dev/packages) 上的 `npm:` 包。Skillshare 只通过 `plugin` 安装和移除包：`plugin add` 接受本地目录、Git 来源或 [npm 包](#pidev-的-npm-包)，`plugin import --from pi` 则可接管用 Pi 安装的包。

Skillshare 读取包时不会运行它们，因此这个标签页显示的是设置选中了哪些文件（**设置**列），而不是 Pi 是否已加载它们；应用后请重新加载 Pi。设置指明但包中不存在的文件会标记为不存在。Skillshare 无法判断的选择会显示**无法判断**，并附上原因与修改方式，绝不猜测开或关。编辑需要该 target 自己的 Pi 是 0.99.2 及以上（用 Pi 本身验证过的最旧版本），且设置是严格的 JSON；更旧的版本为只读，标签页会显示检测到的版本。运行其他程序的 Pi 账号为只读，Skillshare 也不会运行它。列表为 `[]`（不加载任何文件）的条目是只读的，由 Skillshare 无法评估的模式（例如 `?` 对上 emoji）决定的 extension 也是只读的。来源为空的条目，或来源、规则中含有未配对的 UTF-16 代理项转义或无效 UTF-8 的条目，因为 Skillshare 无法像 Pi 一样准确读取，会保持原样并设为只读。Pi 只采用包的第一个全局条目，所以当 Skillshare 无法读取那个条目时，同一个包后面的条目也是只读的。

同步到 Pi 的项目在项目页面上也有这个标签页。它显示项目设置叠加在全局设置之上后，每个包选中的内容，并标明是继承自 `pi (global)` 还是项目覆盖。开关只会把规则保存到项目的 `.pi/settings.json`，做法与 `pi config` 相同：全局包会得到一个项目条目 `{"source": ..., "autoload": false, "extensions": [...]}`，只改变它指明的文件，全局条目保持不变。本地来源会写成相对于 `.pi` 的路径，npm 或 git 来源则按全局设置的写法。移除这类条目的最后一条项目规则时，只有不会让更早注册的 filters 生效才移除该条目；否则保留空的 winning override。只有明确的 JSON `false` 才表示 delta，`autoload: null` 不是 `false`。没有对应全局条目、且 `autoload: false` 的项目条目只会加载它用 `+` 指明的文件。文件及其 `.pi` 文件夹只会在应用时创建。全局设置和 Pi 的 `trust.json` 永远不会被写入，Skillshare 也不会替你信任项目：Pi 只有在信任项目时才会使用项目设置。含有凭据或查询字符串的全局来源不会被复制到项目中，所以该包在项目中是只读的；项目设置中有 Skillshare 无法读取的条目时，所有包都是只读的。应用时会持有 Pi 对项目文件的锁，并在写入前再次检查两个设置文件和包。Pi 自己的 `extensions` 文件夹中的 extension（包括 [extras](./extras.md) 链接到那里的文件）以只读方式列出，并说明在哪里修改；项目会列出自己的文件夹（Pi 只有在信任项目时才会读取）和全局文件夹。

OpenCode 会在 `opencode.json` 或已存在的 `opencode.jsonc` 中将受管理的 entry 注册为文件 URL，
并保留注释和无关的条目。Version 1 使用 `plugin`；version 2 使用 `plugins`。`XDG_CONFIG_HOME`
和绝对路径的全局 `OPENCODE_CONFIG` 会被遵循；含糊或不受支持的覆盖会被拒绝。
OpenCode 必须在 PATH 上，以便 Skillshare 能够选择对应版本的 schema。

本地 OpenCode source 必须已经包含其构建好的 entry（`main`、字符串形式的 root export，
或 `index.js`）以及所需的运行时依赖。Skillshare 不会运行构建脚本，也不会将依赖安装到 source 中。
注册成功并不代表模块已成功加载；请在重新加载后检查 OpenCode。

全局 npm 注册没有 managed cache 时显示 Unknown／只读，不代表尚未安装；Pi 可能使用 Skillshare 不探查的 legacy global npm/pnpm 路径。

Import 接受普通 Pi source，以及Pi 0.99.2 及以上来源与选项格式受支持的 filtered object。预览只显示保留的字段名称，不显示 opaque 值。导入不修改原生设置或已安装文件；原始条目保存在 Skillshare 私有状态，共享配置仅存 digest。sync/update 保留当前条目；卸载前保存最新选项，重新安装时先恢复 object，避免暂时按默认规则启用其他资源。同一次 Apply 批量恢复时，只接受此次操作自己写出的精确内容；其他设置变动仍会阻止后续恢复。这些 bindings 必须保留私有状态：记录丢失、被修改或属于其他 target 时拒绝恢复。Windows 的新注册目录以受保护的 owner/SYSTEM ACL 创建。现有目录与记录若允许当前用户及特权 SYSTEM/Administrators 之外的主体访问，或无法验证 ACL，便拒绝导入或恢复。不会修改现有 ACL；请保留记录，由所有者修复访问保护后再重试。不确定的来源或优先级、不支持的编码，以及 Pi 会规范化的本地引用仍为只读。普通 OpenCode 条目可导入，filtered OpenCode 条目仍拒绝。超过 Pi 10 秒过期阈值的空锁目录，只有 inode 与 mtime 未变动时才能回收；新锁、更新或被替换的锁、非空目录、文件与 symlink 一律保留。过期不代表持有者已终止，最后检查与移除不是原子 CAS。已导入的 Pi package 在全局模式下用 `pi update SOURCE` 更新，并保留其设置条目；project 中的则要在 Pi 里更新，因为 `pi update` 也会影响全局 package。已导入的 OpenCode v1 packages 会在其原生工具中更新。
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
- Codex 原生 project 安装不由此适配器提供。同步选择
  对全局 Codex 安装仍然有效。
- Codex 没有 update 命令，因此更新会用刷新后的快照再次 add 该 plugin。
  add 总会启用它，所以在 Codex 中被停用的 plugin 会被跳过。导入的 Codex
  plugin 会用 `codex plugin marketplace upgrade NAME` 更新，这会重新安装 Codex
  从该 marketplace 安装的所有 plugin，Codex 启动时也会这样做。
- 更新遇到无法处理的 target 时会跳过并说明原因；该 plugin 的其他 Agent 仍会照常更新，
  被跳过的更新会保留为待处理，留给之后的 sync。
- 已导入的 plugins 保留其原始的 marketplace 身份。`check` 无法为没有 source 的
  已导入 plugin 推断发布可用性。若已导入的 Claude 或 Codex plugin 的原生
  marketplace 已经不在，sync 和 update 会跳过该 target 并说明原因。在从未添加该 marketplace 的另一台
  机器上也会出现同样的情况。请在 Agent
  中恢复该 marketplace，或移除该 target 后从 source 重新添加 plugin；Skillshare
  不会自行把已导入的 plugin 改到其他 source。
- Skillshare 为每个受管理的 Claude/Codex plugin 注册一个 marketplace，命名为
  `skillshare-<plugin>-<hash>`（较早的安装保留 `skillshare-<hash>`）。移除或排除该
  plugin 时，即使 plugin 已经不在，也会一并移除这个 marketplace；清理失败会在下次
  sync 重试。若 marketplace 不见了，update 会重新注册。位于其他路径的同名注册与已导入
  plugin 的 marketplace 不会被动到；快照与原生缓存会保留。
- 这些注册指向本机的 Skillshare 状态目录，user 与 project 设置都一样。通过 Git 或
  dotfile 管理工具共享 Agent 设置，会把其他机器上不存在的路径带过去；请在每台机器上
  从 source 添加 plugin。
- Claude 也会把 skills 目录里带有 plugin manifest 的 skill 文件夹读成名为
  `<name>@skills-dir` 的 plugin，而且同名的 plugin 只加载一个。添加同名的 Claude plugin
  时，预览会说明这一点：Claude 会加载该 plugin 并跳过那个 skill 文件夹，直到其中一个改名或移除。

原生生命周期已在 Claude Code `2.1.276`、Codex CLI
`0.154.0`、Pi `0.85.1` 和 Copilot CLI `1.0.86` 上验证。Antigravity CLI
`1.2.6` 通过隔离的原生 install/list/remove 操作进行了检查。OpenCode `1.18.31` 用于验证
版本感知的注册；v2 schema 由 fixture 测试覆盖。Cursor 和 Antigravity 的
文件系统生命周期在隔离目录中进行了测试，但不代表已验证 GUI 运行时激活。已安装的命令
capabilities 和清单 schemas 会在运行时检查；不受支持的操作会被阻止并给出说明。

## 官方格式参考

- [Cursor local plugins](https://prod.cursor.com/docs/plugins)
- [Antigravity desktop plugins](https://www.antigravity.google/docs/plugins)
- [Antigravity standalone CLI plugins](https://www.antigravity.google/docs/cli/plugins)
- [Pi packages](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/packages.md)
- [OpenCode v1 plugins](https://opencode.ai/docs/plugins/)
- [OpenCode v2 plugins](https://opencode.ai/v2/docs/plugins)
