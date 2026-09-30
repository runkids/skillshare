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
skillshare hooks restore BACKUP_ID --dry-run
skillshare hooks restore BACKUP_ID
```

没有子命令时列出 entries。`add`／`edit` 从 `--file` 读取 Entry JSON 或 YAML；导入不指定名称时列出候选项（每个 Agent event 或文件各一项），指定名称才保存。保存导入即原地接管读到的注册：下次同步直接 adopt，无需 `--replace`；未导入的 event 保持不受管理。导入只读取配置或代码，不执行。新增、编辑、导入、启用、停用与移除默认只保存来源，加上 `--sync` 才同步。同步与恢复会重新预览。

| Option | Meaning |
|---|---|
| `--file PATH` | Entry JSON/YAML；导入时为原生配置或代码 |
| `--from AGENT` | 原生 Agent 格式或要读取的 Agent |
| `--sync` | 保存并同步 |
| `--replace` | 明确替换已有来源 entry 或该 entry 冲突的原生输出 |
| `--dry-run, -n` | 仅预览，不保存或写入 |
| `--json` | 结构化输出 |
| `--revision ID` | 要求匹配指定预览 revision |
| `--global, -g` | 使用 global 配置 |
| `--project, -p` | 使用 project 配置 |

`hooks list --json` 显示备份 ID 和完整目标路径。配置变化后，旧预览失效；保存或同步前需要重新预览。

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

| Option | Meaning |
|---|---|
| `description` | 可选描述 |
| `enabled` | 默认 true；false 保留来源，下次同步移除未被修改的自有输出 |
| `bindings` | Agent ID 到原生 binding 的映射 |
| `bindings.AGENT.events` | command／配置型 Agent 的原生 event map |
| `bindings.AGENT.code` | Pi、Amp、OpenCode 的原生 extension/plugin 代码 |
| `bindings.AGENT.files` | 可选 UTF-8 脚本文件，以相对文件名为 key |

Agent ID 为 `claude`、`codex`、`gemini`、`copilot`、`cursor`、`droid`、`qwen`、`antigravity`、`pi`、`amp`、`opencode`；`factory` 是 `droid` 的别名，`antigravity-cli` 和 `agy` 是 `antigravity` 的别名。event、matcher、handler type、command、timeout 单位和 payload 均保留原生格式，不自动跨 Agent 转换。event 名称会对照各 command Agent 文档列出的事件检查：未知名称（例如拼错的 `Stopp`）在预览和 plan 的 `warnings` 中显示警告，但不阻止同步，因为 Agent 会陆续新增事件。Pi、Amp、OpenCode 的代码不检查。Pi、Amp、OpenCode 的代码及 imports 须匹配已安装版本，发布为独立的 `skillshare-NAME.ts`，不生成共享执行引擎。command binding 的脚本位于 Agent 配置目录的 `hooks/skillshare/NAME/`；command 保留你提供的原生 macro 或明确路径。请在预览中确认完整路径。

## 原生目标路径

| Agent | Global | Project | Format |
|---|---|---|---|
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

global scope 使用原生配置目录的环境变量 override；project scope 只写入项目，不回退到 global。Codex inline TOML 等其他来源仍独立存在。Antigravity 及其 CLI（`agy`）读取同一份 `hooks.json`；每个 hook 是一个以其名称命名的 block，导入时保留原名。CLI 的 `~/.gemini/antigravity-cli/settings.json` 中的 hooks 保持独立。Droid 发布独立 hooks 文件会影响原生加载来源，请先检查已有 inline hooks。


Droid 存在有效 inline hooks 时，同步会拒绝创建独立文件。先导入并检查，移除原 inline hooks 后再同步。

## 项目、冲突与恢复

global 配置的 `hooks.projects` 将绝对项目路径映射到相同 Entry 格式的 `entries`，可在“项目 → Hooks”管理。已有 `.skillshare/config.yaml` 的项目须使用 project scope；单个项目同步只处理该项目。

同步保留无关配置和非 Skillshare 管理的 hooks。内容相同不代表所有权。自有输出若被外部修改，停用、移除及恢复也会报告冲突。明确替换只作用于选定 entry；预览列出完整动作和路径。共享文件的每一行 plan 会列出该 entry 新增（`+`）、更新（`~`）、移除（`−`）的 event，JSON plan 的 `events` 也包含相同信息。`update` 表示该 entry 在文件中仍有注册，`remove` 表示完全离开该文件。编辑会保留文件原有格式（紧凑或缩进）；Skillshare 添加的 `hooks` key 在最后一个 hook 移除时一并移除。

备份恢复原生输出并保留后续无关改动，不改写来源定义。使用“设置 → 备份 → Hooks”或 `hooks restore` 先预览再恢复。

**已同步**只表示 Skillshare 已写入配置。请按 Agent 原生流程重启／重新加载；信任、启用 hooks 和代码兼容性由 Agent 控制。管理操作不执行 hook command，也不自动改变原生信任。
