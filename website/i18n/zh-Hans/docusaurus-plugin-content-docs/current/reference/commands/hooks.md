---
sidebar_position: 4
---

# hooks

管理具名 hooks，将各 Agent 的原生定义同步到配置文件。Dashboard 的 **Hooks** 提供新增、编辑、导入、启用／停用、预览与同步；“目标”、“项目”、“同步”及“设置 → 备份”也有对应入口。

从 hook 每行的菜单可查看各目标的原生配置或代码，包括脚本文件与目标路径。此只读预览仅显示该 hook 的内容，共享文件会保留其他配置。停用的 hook 仍可查看，但不会发布。

## 命令

```bash
skillshare hooks
skillshare hooks list --json
skillshare hooks add check --file ./check.yaml
skillshare hooks edit check --file ./updated-check.yaml --sync
skillshare hooks import --from claude --json
skillshare hooks import imported --from claude --file ./settings.json --dry-run
skillshare hooks disable check --sync
skillshare hooks enable check --sync
skillshare hooks sync --dry-run --json
skillshare hooks sync
skillshare hooks sync check --replace --dry-run
skillshare sync hooks --dry-run --json
skillshare sync hooks
skillshare hooks remove check --sync
skillshare hooks remove check --keep-files
skillshare hooks restore BACKUP_ID --dry-run
skillshare hooks restore BACKUP_ID
```

没有子命令时列出 entries。`add`／`edit` 从 `--file` 读取 Entry JSON 或 YAML；导入不指定名称时列出候选项（每个 Agent event 或文件各一项），指定名称才保存。保存导入即原地接管读到的注册：下次同步直接 adopt，无需 `--replace`；未导入的 event 保持不受管理。导入只读取配置或代码，不执行。新增、编辑、导入、启用、停用与移除默认只保存来源，加上 `--sync` 才同步。同步与恢复会重新预览。

| Option | Meaning |
|---|---|
| `--file PATH` | Entry JSON/YAML；导入时为原生配置或代码 |
| `--from AGENT` | `codex-2` 等账号 target 也可使用。  原生 Agent 格式或要读取的 Agent |
| `--sync` | 保存并同步 |
| `--keep-files` | 配合 `remove` 使用：停止管理该 hook，并让它的原生条目保持原样。不能与 `--sync` 同时使用。参见[下文](#stop-managing-a-hook) |
| `--replace` | 明确替换已有来源 entry 或该 entry 冲突的原生输出 |
| `--dry-run, -n` | 仅预览，不保存或写入 |
| `--json` | 结构化输出 |
| `--revision ID` | 要求匹配指定预览 revision |
| `--global, -g` | 使用 global 配置 |
| `--project, -p` | 使用 project 配置 |

`hooks list --json` 显示备份 ID 和完整目标路径。配置变化后，旧预览失效；保存或同步前需要重新预览。

## 停止管理某个 hook {#stop-managing-a-hook}

```bash
skillshare hooks remove check --keep-files
```

这会从来源移除 `check`，并忘记 Skillshare 为它写入的原生注册和文件。Agent 文件不会变动。之后这些条目就归你所有：同步不会再移除或更新它们，`hooks import` 也会再次列出它们。`--keep-files` 不能与 `--sync` 同时使用。Dashboard 的移除对话框在 Hooks 页面和项目的 **Hooks** 标签页都提供 **停止管理**。**仅从来源移除** 则不同：下次同步时，会从 Agent 文件删掉该 hook 的条目。

只会影响你移除它的那个范围。停止管理 global hook 时，项目中同名的 hook 仍受管理，反之亦然。若要重新管理这些条目，请导入它们。

## 来源字段

声明放在当前 Skillshare 配置的 `hooks.entries`。名称标识一个 entry；`bindings` 指定接收 Agent 和原生定义。空 bindings 保留来源但不发布。

```yaml
hooks:
  entries:
    check:
      description: Run the project's check after Claude finishes
      enabled: true
      bindings:
        claude:
          events:
            Stop:
              - hooks:
                  - type: command
                    command: "make check"
                    timeout: 120
```

传给 `hooks add check --file check.yaml` 的文件只包含 Entry 的 `description`、`enabled`、`bindings`，不包含 `hooks.entries` 外层。

Skillshare 以缩进的 block 格式写入 `hooks` 区段；每次保存也会把之前被挤成一行的 entry 展开。Hooks 页的 **config.yaml** 按钮会打开“设置 → 文件”并定位到 `hooks:` 区段；点击 `hooks` 下的字段，右侧面板会显示说明。**美化** 会展开嵌套的单行区段，`targets: [claude, codex]` 这类短列表则保持一行。

| Option | Meaning |
|---|---|
| `description` | 可选描述 |
| `enabled` | 默认 true；false 保留来源，下次同步移除未被修改的自有输出 |
| `bindings` | Agent ID 到原生 binding 的映射 |
| `bindings.AGENT.events` | command／配置型 Agent 的原生 event map |
| `bindings.AGENT.code` | Pi、Amp、OpenCode 的原生 extension/plugin 代码 |
| `bindings.AGENT.files` | 可选 UTF-8 脚本文件，以相对文件名为 key |

Agent ID 为 `claude`、`codex`、`gemini`、`copilot`、`cursor`、`droid`、`qwen`、`antigravity`、`pi`、`amp`、`opencode` and `git`；`factory` 是 `droid` 的别名，`antigravity-cli` 和 `agy` 是 `antigravity` 的别名。event、matcher、handler type、command、timeout 单位和 payload 均保留原生格式，不自动跨 Agent 转换。event 名称会对照各 command Agent 文档列出的事件检查：未知名称（例如拼错的 `Stopp`）在预览和 plan 的 `warnings` 中显示警告，但不阻止同步，因为 Agent 会陆续新增事件。Pi、Amp、OpenCode 的代码不检查。Pi、Amp、OpenCode 的代码及 imports 须匹配已安装版本，发布为独立的 `skillshare-NAME.ts`，不生成共享执行引擎。command binding 的脚本位于 Agent 配置目录的 `hooks/skillshare/NAME/`；command 保留你提供的原生 macro 或明确路径。请在预览中确认完整路径。 global binding 也可以使用在 `targets` 下声明的账号 target 名称。

## 原生目标路径

| Agent | Global | Project | Format |
|---|---|---|---|
| Git | `$XDG_CONFIG_HOME/git/skillshare/hooks.gitconfig` | `<git-common-dir>/skillshare/hooks.gitconfig` | Git 2.54+ `hook.<name>` |
| [Claude Code](https://code.claude.com/docs/en/hooks) | `~/.claude/settings.json` | `.claude/settings.json` | `hooks` event map with matcher groups |
| [Codex](https://learn.chatgpt.com/docs/hooks) | `~/.codex/hooks.json` | `.codex/hooks.json` | Wrapped `hooks` event map |
| [Gemini CLI](https://geminicli.com/docs/hooks/reference/) | `~/.gemini/settings.json` | `.gemini/settings.json` | `hooks` event map |
| [Copilot CLI](https://docs.github.com/en/copilot/reference/hooks-reference) | `~/.copilot/hooks/skillshare-NAME.json` | `.github/hooks/skillshare-NAME.json` | Version 1, `hooks` event map |
| [Cursor](https://cursor.com/docs/hooks) | `~/.cursor/hooks.json` | `.cursor/hooks.json` | Version 1, native lowerCamelCase events |
| [Factory Droid](https://docs.factory.com/harness/hooks) | `~/.factory/hooks.json` | `.factory/hooks.json` | Unwrapped event map |
| [Qwen Code](https://qwenlm.github.io/qwen-code-docs/en/users/features/hooks/) | `~/.qwen/settings.json` | `.qwen/settings.json` | `hooks` event map |
| [Antigravity](https://antigravity.google/docs/hooks) | `~/.gemini/config/hooks.json` | `.agents/hooks.json` | 具名 hook block，每个 hook 一个 |
| [Pi](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/extensions.md) | `~/.pi/agent/extensions/skillshare-NAME.ts` | `.pi/extensions/skillshare-NAME.ts` | Native extension code |
| [Amp](https://ampcode.com/docs/plugin-api) | `~/.config/amp/plugins/skillshare-NAME.ts` | `.amp/plugins/skillshare-NAME.ts` | Native plugin code |
| [OpenCode](https://opencode.ai/docs/plugins/) | `~/.config/opencode/plugins/skillshare-NAME.ts` | `.opencode/plugins/skillshare-NAME.ts` | Supplied v1/v2 plugin code |

global scope 使用原生配置目录的环境变量 override，但指向账号 `config_dir` 时除外；project scope 只写入项目，不回退到 global。Codex inline TOML 等其他来源仍独立存在。Antigravity 及其 CLI（`agy`）读取同一份 `hooks.json`；每个 hook 是一个以其名称命名的 block，导入时保留原名。CLI 的 `~/.gemini/antigravity-cli/settings.json` 中的 hooks 保持独立。Droid 发布独立 hooks 文件会影响原生加载来源，请先检查已有 inline hooks。Copilot 只在受信任的文件夹中加载 `.github/hooks` 的项目 hooks。


Droid 存在有效 inline hooks 时，同步会拒绝创建独立文件。先导入并检查，移除原 inline hooks 后再同步。

## 某个 Agent 的另一个账号 {#accounts}

声明了 `agent` 和 `config_dir` 的 target 可以用自己的名称接收 hooks。Claude、Codex 和 Pi 账号使用其 Agent 的原生 binding 格式。

```yaml
targets:
  codex-2:
    agent: codex
    config_dir: ~/.codex-2
    skills: {enabled: false}
hooks:
  entries:
    check:
      bindings:
        codex-2:
          events:
            Stop:
              - hooks:
                  - type: command
                    command: "echo checked"
```

| 账号的 Agent | 原生目的地 |
|---|---|
| `claude` | `<config_dir>/settings.json` |
| `codex` | `<config_dir>/hooks.json` |
| `pi` | `<config_dir>/extensions/skillshare-NAME.ts` |

command 脚本写入 `<config_dir>/hooks/skillshare/NAME/`。账号仅用于 global 配置；project 配置与 `hooks.projects` 应绑定 Agent 本身，因为所有账号都会读取相同的项目文件。`config_dir` 不存在时会警告并跳过，不会创建账号目录。

当 `CODEX_HOME`、`CLAUDE_CONFIG_DIR` 或 `PI_CODING_AGENT_DIR` 指向已声明账号的目录时，普通 Agent binding 使用内置默认目录，plan 会显示警告。各账号仍使用自己的 `config_dir`，其他 override 保留现有行为。包括 symlink 在内，解析为同一目录的两个 binding key 不能在一次同步中写入。

如果 target 已移除、目录移动或账号目录不存在，已管理的输出及其所有权记录会保留，并显示警告。恢复 target 或目录即可继续管理。要释放某个条目暂存的所有权而不修改文件，请先预览再执行 `skillshare hooks sync NAME --replace -g`。移除 target 时若 binding 仍使用该名称会显示警告；下次 hooks 同步前也要移除这些 binding。

用 `skillshare hooks import --from codex-2 -g --json` 从账号导入。控制台显示 **codex-2 (Codex)**，并使用 Codex 的事件与编辑器。每个 Codex 目录都需要在 `/hooks` 中单独审核和信任变更的 hooks；同步不会修改信任或执行 hooks。

## Git hooks {#git-hooks}

`bindings.git.commands` 声明 Git 2.54+ 的具名 config hook。每个名称包含
`events: [pre-commit]`、单行 `command` 和可选 `parallel`（Git 2.55+）。
名称不能是 Git event 名，以字母或数字开头，仅含字母、数字、`_`、`-`、`.`，
最多 128 字符，不含 `..`，也不能以 `.` 结尾。同一目标的已启用 entry 名称重复会冲突。
未知 event 只警告。Dashboard 使用 YAML 编辑整个 Git binding；顺序按 entry 名再按 hook 名排序。

Git 会把 event 参数追加到 command，并向每个 hook 提供完整 stdin。
复合 shell 逻辑放在 UTF-8 `files`，用 `command: "{files}/check.sh"` 引用。
`{files}` 展开为各机器上带引用的绝对 helper 路径；文件以 LF 和可执行权限生成。
以 `fi`、`done`、`esac`、`}`、`)` 结尾的 command 会被拒绝。

global 输出为 `$XDG_CONFIG_HOME/git/skillshare/hooks.gitconfig`（默认 `~/.config/git/…`）。
include 写入目标依次为绝对路径 `GIT_CONFIG_GLOBAL`、已有 `~/.gitconfig`、
已有 XDG `git/config`、新建 `~/.gitconfig`。project 输出为
`<git-common-dir>/skillshare/hooks.gitconfig`，由 common `config` 引入。
linked worktree 共享目标，每个 common directory 只声明一个非 bare 仓库顶层 root。

只通过原生 lock 修改可写的普通 config，不穿透 symlink 写入。缺少 include 时显示
**inactive** 和手动添加的准确行。旧 Git 同样 inactive，不生成 fallback dispatcher。
缺少 Git 或 root 时跳过输出并保留所有权记录。手动 include 和 includeIf 仍属用户所有，
不会扩大条件。conditional include 存在不代表已激活。用 `hooks list -g --json` 的
`git`／`projectGit` 查看能力、include 和 `core.hooksPath`。

生成文件由 Skillshare 整体拥有。`--replace` 备份后重新生成整个文件，覆盖外部编辑。
同名外部 hook 只能在可写普通目标 config 内替换；system／include 文件冲突和其他 config
的有效所有权不能替换。`enabled=false` 只警告。preview 和 section backup 不含无关私密配置。
删除或禁用后的同步移除拥有的输出和 include，保留手动 include 与无关设置。
restore 保留后来无关的编辑，不更改 source。Git binding 因共享文件而拒绝 `--keep-files`。

config hook 与 hookdir script 可能都执行；迁移前手动检查重复。Git import、hookdir 识别、
script mode、结构化 editor 和 doctor 留待后续 phase。生成 Windows 格式路径，但 Windows 执行未验证。

全局与项目命令也应使用不同的名称，Git 会合并两个作用域。位于未激活父条件下的嵌套 include 同样保留条件。无法读取或嵌套过深的 include 声明会阻止同步。

## 项目、冲突与恢复

global 配置的 `hooks.projects` 将绝对项目路径映射到相同 Entry 格式的 `entries`，可在“项目 → Hooks”管理。已有 `.skillshare/config.yaml` 的项目须使用 project scope；单个项目同步只处理该项目。

同步保留无关配置和非 Skillshare 管理的 hooks。内容相同不代表所有权。自有输出若被外部修改，停用、移除及恢复也会报告冲突。明确替换只作用于选定 entry；预览列出完整动作和路径。共享文件的每一行 plan 会列出该 entry 新增（`+`）、更新（`~`）、移除（`−`）的 event，JSON plan 的 `events` 也包含相同信息。`update` 表示该 entry 在文件中仍有注册，`remove` 表示完全离开该文件。编辑会保留文件原有格式（紧凑或缩进）；Skillshare 添加的 `hooks` key 在最后一个 hook 移除时一并移除。Skillshare 创建的文件若已没有其他内容就会删除，连同它创建且已清空的文件夹；预览会把这显示为删除文件。原本就存在的文件和文件夹，以及你添加的任何内容都会保留。

备份恢复原生输出并保留后续无关改动，不改写来源定义。使用“设置 → 备份 → Hooks”或 `hooks restore` 先预览再恢复。

**已同步**只表示 Skillshare 已写入配置。请按 Agent 原生流程重启／重新加载；信任、启用 hooks 和代码兼容性由 Agent 控制。管理操作不执行 hook command，也不自动改变原生信任。
