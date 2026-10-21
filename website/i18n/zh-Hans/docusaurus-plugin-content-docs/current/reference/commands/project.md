---
sidebar_position: 21
---

# project

在全局配置中管理项目文件夹，并把 Skill 和 Agent 同步到这些文件夹，无需在每个文件夹中创建 `.skillshare/` 配置。

`project` 始终使用全局配置，即使在项目内部运行也是如此。这与 `-p` / `--project` 不同：后者选择仓库自己的配置，并且会被本命令拒绝。如果队友应当随仓库一起拿到这套设置，请使用 [project mode](/docs/understand/project-skills)。

## 用法 {#usage}

```bash
skillshare project                          # Same as project list
skillshare project list --json
skillshare project show <project> --json
skillshare project add <root> --target T [options]
skillshare project edit <project> [options]
skillshare project sync <project> [--dry-run] [--json]
skillshare project remove <project> [--yes] [--dry-run] [--json]
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
| `--sync` | add, edit | 保存后同步该项目的 Skill 和 Agent；不启用 force |
| `--dry-run`, `-n` | add, edit, sync, remove | 预览，不写入也不提示确认 |
| `--yes` | remove | 跳过确认；非预览状态下以 JSON 输出移除时必须提供 |
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

保存的条目属于 [`projects`](/docs/reference/targets/configuration#projects)。共享的工具文件夹会被归并为同一个 sync Target。普通的 `skillshare sync` 也会同步已注册的项目。

禁用了 Skill 的项目永远不会执行 Skill 同步流程，全局 symlink 模式下也是如此。请求或启用的资源选择为空时，会直接成功，不进行发现、备份或 Target 写入。

### 替换过滤器或重置覆盖设置 {#edit-settings}

```bash
skillshare project edit shop-web --skills-include 'frontend-*' --skills-include 'core-*'
skillshare project edit shop-web --skills-exclude= --skills-mode= --name=
skillshare project sync shop-web --dry-run --json
skillshare project sync shop-web --json
```

add 和 edit 会在写入前校验完整的有效配置。YAML 锚点可能会影响其他条目；变更结果会列出所有受影响的根路径。没有任何变化的 add 或 edit 会返回 `applied: false` 和 `unchanged: true`，但仍会执行 `--sync`。用不同的设置重复执行 add 会失败，并提示改用 `project edit`。自定义名称不能看起来像根路径（例如 `~name`、`./name` 或 `C:name`）。

### 移除注册 {#remove}

```bash
skillshare project remove shop-web --dry-run --json
skillshare project remove shop-web --yes --json
```

移除时会先分离 Skill，再删除注册。整个文件夹的链接会被删除；merge 模式的链接在 Source 内容仍存在时会变成本地副本；copy 模式的文件会保留，只删除其 manifest。仍被某个存续且启用的 Target 拥有的路径会被保留。

CLI edit 会按相同规则分离被移出的 Skill 路径。只要项目的 Skill 仍处于禁用状态，任何 CLI 编辑（包括重命名或仅修改 Agent 设置）也会分离其残留的 Skill 路径，即使此前是在仪表盘中禁用的也一样。复制回本地以及保留存续且启用的 Target 所拥有路径的规则仍然适用。仪表盘保存不会分离 Skill。Agent 文件会原样保留。预览和结果会列出被删除、复制回来以及被保留的路径。

## JSON 与失败处理 {#json-and-failures}

| 命令 | 主要字段 |
|---------|-------------|
| list | `projects` 数组（没有注册项时为空） |
| show | `project` 对象 |
| add, edit, remove | `action`、`project`、`applied`、`affected`、`warnings`；可选的 `sync` 或 `detach` |
| sync | `action`、`project`、`sync`；预览还包含 `applied: false` |

项目对象包含 `root`（保存时的写法）、`path`（展开后的路径）、`name`、`targets`、`skills`、`agents`、`groups`、`missing` 和 `hasOwnConfig`。省略或禁用的资源为 `null`。启用的资源会给出其设置。文本格式的 `project list` 和 `project show` 会报告开/关状态。sync 结果包含 `results`、`failed`、`warnings`，以及发现与过滤的诊断信息。

某个 Target 失败时，sync 会在报告已完成的 Target 的同时以退出码 1 结束。sync 预览会在 `failed` 中报告冲突并以退出码 0 结束；无效输入仍然会失败。如果 add/edit 保存成功但其 sync 失败，注册会保持已保存状态；修复 Target 后用 `project sync` 重试。缺失的项目文件夹仍然可以 edit，但无法 sync。

edit 和移除不是事务性的：之后的分离或保存失败可能会留下先前已完成的文件系统变更。JSON 结果会报告已完成的 `detach`、`registration`（`kept`、`updated` 或 `removed`）以及 `error`。edit 保存失败时为 `applied: false`，并描述已持久化的项目；保存成功但随后 sync 失败时仍为 `applied: true`。重试前请检查这些字段。在任何变更发生前的失败使用错误 payload。list、show、预览和 JSON 输出会跳过启动时的迁移、清理和版本检查；文本输出的 add、edit、sync 和 remove 与其他命令一样执行它们。

本命令只管理 Skill 和 Agent。MCP 项目设置和 hooks 使用各自的命令；它不会转换 Target，也不会编辑项目自己的配置。

## 并发写入项目 {#concurrency}

CLI 和仪表盘对项目的修改共享一把 `gofrs/flock` 锁，锁文件位于 skillshare 的 state 目录中，按解析后的全局配置目标区分；它们最多等待 10 秒，获取锁后会重新加载配置。配置的符号链接共享同一把锁。读取和 dry run 不会创建或获取这把锁。其他配置写入方，包括 `target`、MCP、hooks 以及普通的 `Config.Save()` 调用方，都不在这一保护范围内；通过这些路径的并发写入仍可能覆盖修改。

## 相关 {#related}

- [Many Projects, One Config](/docs/how-to/recipes/many-projects-one-config)
- [项目配置字段](/docs/reference/targets/configuration#projects)
- [MCP 项目设置](/docs/reference/commands/mcp#manage-several-projects-from-the-global-config)
