---
sidebar_position: 21
---

# project

管理 global 配置会同步 skills 与 agents 的项目文件夹，无需在每个文件夹创建 `.skillshare/`。除非选择只保存在本机，项目都保存在 `config.yaml` 中。

`project` 始终使用全局配置，即使在项目内部运行也是如此。这与 `-p` / `--project` 不同：后者选择仓库自己的配置，并且会被本命令拒绝。如果队友应当随仓库一起拿到这套设置，请使用 [project mode](/docs/understand/project-skills)。

## 用法 {#usage}

```bash
skillshare project                          # Same as project list
skillshare project list --json
skillshare project show <project> --json
skillshare project add <root> --target T [options]
skillshare project edit <project> [options]
skillshare project sync <project> [--dry-run] [--json]
skillshare project localize <project> [--dry-run] [--json]
skillshare project share <project> [--yes] [--dry-run] [--json]
skillshare project remove <project> [--shared] [--yes] [--dry-run] [--json]
```

`add` 需要一个已存在的文件夹和至少一个 Target。其他命令接受项目的有效名称（自定义名称或文件夹 basename）或其根路径。要明确选择根路径，请在相对路径前加上 `./` 或 `../`。如果一个不带前缀的参数同时匹配某个项目的名称和另一个项目的根路径，命令会报告这两个候选项，而不会替你选择其中一个。符号链接别名会保持为独立的注册项。

## 参数 {#flags}

| 参数 | 适用于 | 说明 |
|------|------------|-------------|
| `--target T` | add, edit | Target 工具；可重复使用或使用逗号分隔列表。add 时必填；edit 时会替换整个列表 |
| `--name N` | add, edit | 自定义名称；`--name=` 恢复为文件夹 basename |
| `--skills`, `--no-skills` | add, edit | 启用或禁用 Skill；add 时默认启用 |
| `--agents`, `--no-agents` | add, edit | 启用或禁用 Agent；add 时默认禁用 |
| `--skills-mode MODE`, `--agents-mode MODE` | add, edit | `merge`、`symlink` 或 `copy`；空值会清除覆盖设置 |
| `--skills-naming NAMING` | add, edit | `flat`、`standard` 或 `prefixed`；空值表示继承 |
| `--skills-include P`, `--skills-exclude P` | add, edit | 重复使用以指定多个 glob 模式；替换对应的列表；空值会清空列表 |
| `--agents-include P`, `--agents-exclude P` | add, edit | Agent 使用相同的替换规则；过滤器需要 merge 或 copy 模式 |
| `--machine-local` | add | 只保存在本机，不写入 `config.yaml` |
| `--shared` | add, remove | add：即使 [`projects_storage`](#local-registrations) 为 `machine-local`，也保存到 `config.yaml`。remove：同时移除本机所覆盖项目在 `config.yaml` 中的副本 |
| `--sync` | add, edit | 保存后同步该项目的 Skill 和 Agent；不启用 force |
| `--dry-run`, `-n` | add, edit, sync, remove, localize, share | 预览，不写入也不提示确认 |
| `--yes` | remove, share | 跳过确认；非预览状态下以 JSON 输出移除时必须提供。share 只在 `config.yaml` 中已有该项目的不同设置时才会询问 |
| `--json` | all | 结构化输出；同时也请检查进程退出码 |
| `--global`, `-g` | all | 显式指定全局范围（也是本命令的默认值） |
| `--help`, `-h` | all | 显示帮助 |

指定资源设置会启用该资源，包括清除已有的 `enabled: false`。省略设置时会保留其禁用状态。edit 只修改你提供的设置；启用一个已有资源时会保留它的设置。相互冲突的开关，以及禁用与设置的组合（例如 `--no-skills --skills-mode copy`）都会被拒绝。没有 Agent 命名方式的参数。与 [`target`](/docs/reference/commands/target) 不同，这些 include/exclude 参数会替换整个列表，而不是逐个添加或删除模式。

## 示例 {#examples}

### 注册、预览并同步 {#register-and-sync}

```bash
skillshare project add ~/work/shop-web --target claude,codex \
  --skills-mode copy --skills-include 'frontend-*' --agents --dry-run
skillshare project add ~/work/shop-web --target claude,codex \
  --skills-mode copy --skills-include 'frontend-*' --agents --sync
skillshare project show shop-web --json
```

项目保存在 `config.yaml` 的 [`projects`](/docs/reference/targets/configuration#projects) 下，因此使用该文件的每台机器都会获得它。对于只存在于本机的文件夹，请加上 `--machine-local`；参见[项目的保存位置](#local-registrations)。共享的工具文件夹会合并为一个 sync target。普通的 `skillshare sync` 也会同步所有项目。

禁用了 Skill 的项目永远不会执行 Skill 同步流程，全局 symlink 模式下也是如此。请求或启用的资源选择为空时，会直接成功，不进行发现、备份或 Target 写入。

### 替换过滤器或重置覆盖设置 {#edit-settings}

```bash
skillshare project edit shop-web --skills-include 'frontend-*' --skills-include 'core-*'
skillshare project edit shop-web --skills-exclude= --skills-mode= --name=
skillshare project sync shop-web --dry-run --json
skillshare project sync shop-web --json
```

add 和 edit 会在写入前校验完整的有效配置。编辑会保存在项目所在的位置：`config.yaml`，或仅本机项目所在的本机。在 `config.yaml` 中，编辑通过 YAML 锚点声明的项目，也会改变该锚点的别名。变更结果会列出受影响的根路径。没有任何变化的 add 或 edit 会返回 `applied: false` 和 `unchanged: true`，但仍会执行 `--sync`。用不同的设置重复执行 add 会失败，并提示改用 `project edit`。自定义名称不能看起来像根路径（例如 `~name`、`./name` 或 `C:name`）。

### 移除项目 {#remove}

```bash
skillshare project remove shop-web --dry-run --json
skillshare project remove shop-web --yes --json
```

移除会从项目所在的位置删除它。从 `config.yaml` 移除项目，会使所有使用该文件的机器都失去该项目，确认提示中也会说明这一点。如果本机覆盖了 `config.yaml` 中的项目，移除只会删除本机的副本：`config.yaml` 中的版本重新生效，JSON 结果会在 `remaining` 中报告。加上 `--shared` 则两者都移除。

移除时会先分离 Skill，再删除项目。整个文件夹的链接会被删除；merge 模式的链接在 Source 内容仍存在时会变成本地副本；copy 模式的文件会保留，只删除其 manifest。仍被某个存续且启用的 Target 拥有的路径会被保留。

CLI edit 会按相同规则分离被移出的 Skill 路径。只要项目的 Skill 仍处于禁用状态，任何 CLI 编辑（包括重命名或仅修改 Agent 设置）也会分离其残留的 Skill 路径，即使此前是在仪表盘中禁用的也一样。复制回本地以及保留存续且启用的 Target 所拥有路径的规则仍然适用。仪表盘保存不会分离 Skill。Agent 文件会原样保留。预览和结果会列出被删除、复制回来以及被保留的路径。

## JSON 与失败处理 {#json-and-failures}

| 命令 | 主要字段 |
|---------|-------------|
| list | `projects` 数组（没有注册项时为空） |
| show | `project` 对象 |
| add, edit, remove, localize, share | `action`、`project`、`applied`、`affected`、`warnings`；可选的 `sync`、`detach` 或 `remaining`（remove） |
| sync | `action`、`project`、`sync`；预览还包含 `applied: false` |

项目对象包含 `root`（保存时的写法）、`path`（展开后的路径）、`name`、`targets`、`skills`、`agents`、`groups`、`missing`、`hasOwnConfig`、`origin`（`shared` 或 `local`）、`originPath`（所属文件）和 `sharedCopy`（`config.yaml` 中也声明了的本机项目）。省略或禁用的资源为 `null`。启用的资源会给出其设置。文本格式的 `project list` 和 `project show` 会报告开/关状态。sync 结果包含 `results`、`failed`、`warnings`，以及发现与过滤的诊断信息。

某个 Target 失败时，sync 会在报告已完成的 Target 的同时以退出码 1 结束。sync 预览会在 `failed` 中报告冲突并以退出码 0 结束；无效输入仍然会失败。如果 add/edit 保存成功但其 sync 失败，注册会保持已保存状态；修复 Target 后用 `project sync` 重试。缺失的项目文件夹仍然可以 edit，但无法 sync。

edit 和移除不是事务性的：之后的分离或保存失败可能会留下先前已完成的文件系统变更。JSON 结果会报告已完成的 `detach`、`registration`（`kept`、`updated` 或 `removed`）以及 `error`。edit 保存失败时为 `applied: false`，并描述已持久化的项目；保存成功但随后 sync 失败时仍为 `applied: true`。重试前请检查这些字段。在任何变更发生前的失败使用错误 payload。所有 project 命令都会跳过启动迁移、清理与版本检查。

本命令只管理 Skill 和 Agent。MCP 项目设置和 hooks 使用各自的命令；它不会转换 Target，也不会编辑项目自己的配置。

## 项目的保存位置 {#local-registrations}

项目保存在以下两处之一：

- **`config.yaml`**（`origin: shared`，默认）。使用该配置文件的每台机器都会获得该项目，例如通过 dotfiles 或 `skillshare push`。
- **仅本机**（`origin: local`）。位于配置目录之外的注册表，适用于只存在于一台机器上的文件夹。

`project add` 默认保存到 `config.yaml`，除非传入 `--machine-local`。若要将本机保存设为默认，请在 `config.yaml` 中设置 [`projects_storage: machine-local`](/docs/reference/targets/configuration#projects-storage)；此时 `--shared` 可把单个项目保存到 `config.yaml`。仪表板的 **添加项目** 对话框提供同样的选择，并以该设置作为初始值。编辑和移除都在项目所保存的位置进行。

在两处之间移动已保存的项目：

```bash
skillshare project localize shop-web --dry-run --json   # keep a copy on this machine only
skillshare project share shop-web --dry-run --json      # move it into config.yaml
```

`localize` 把 `config.yaml` 中的项目复制到本机，不会更改 `config.yaml`、解除链接或同步。其他机器仍使用 `config.yaml` 的版本；本机使用自己的副本。`share` 把仅本机的项目写入 `config.yaml`，并删除本机副本。当 `config.yaml` 中已有同名项目但设置不同时，`share` 会先询问再替换；`--yes` 则不询问直接替换。如果项目已经在目标位置，两者都不做任何事，也都不会同步。

本机项目会完整替换相同规范化根目录（展开 `~`、清理路径；项目符号链接根目录各自独立）的 `config.yaml` 条目。v0.29.0 保存的移除操作会在该机器上隐藏 `config.yaml` 中的项目，在重新添加该项目之前一直保持隐藏。不同根目录的名称必须唯一，同一层的重复规范化根目录会被拒绝。无论项目本身保存在哪里，项目的 MCP server 和 hooks 始终位于 `config.yaml` 的 `mcp.projects` 与 `hooks.projects` 下。

存储位置为 `$XDG_STATE_HOME/skillshare/projects/<identity>.yaml`；未指定时，Unix 使用 `~/.local/state/skillshare/projects`，Windows 使用 `%LOCALAPPDATA%\skillshare\state\projects`。请保持状态目录为本机位置。标识由解析后的配置文件目标决定；配置符号链接别名共享状态，移动目标会使用新注册。读取不创建文件，无效版本或内容会拒绝加载。

仪表板的 **Convert** 操作会在一次写入中，把普通 target 替换为 `config.yaml` 中的 `projects` 声明。已经声明、被本机覆盖或在本机被隐藏的根目录无法转换。原始配置编辑器会在写入前，把提交的 YAML 与本机项目合并后再验证。

## 并发写入项目 {#concurrency}

CLI 与仪表板共享本机状态锁，最多等待 10 秒并在获取后重新加载。候选数据会在解除链接与写入前检查共享与本机快照；其他共享配置写入者不受此保护。仪表板 Convert 是明确的共享配置更改，会拒绝已声明、隐藏或本机覆盖的根目录。原始配置编辑器会先验证提交内容与本机项目的合并结果。 像 `share` 这样同时更改两处的操作会先写入 `config.yaml`，再写入本机注册表，因此写入被中断时也不会丢失项目。

## 相关 {#related}

- [Many Projects, One Config](/docs/how-to/recipes/many-projects-one-config)
- [项目配置字段](/docs/reference/targets/configuration#projects)
- [MCP 项目设置](/docs/reference/commands/mcp#manage-several-projects-from-the-global-config)
