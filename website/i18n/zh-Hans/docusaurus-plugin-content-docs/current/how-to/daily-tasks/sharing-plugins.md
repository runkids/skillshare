---
sidebar_position: 9
---

# 跨工具管理 plugin

一个 plugin 可以包含 Skill、MCP 连接、hook、脚本以及其他
协同工作的文件。Skillshare 会保持该套件的完整性，并让你选择
哪些工具能接收它。你不需要编写 YAML 就能开始使用。

## 添加你的第一个 plugin

在控制台中，打开 **Plugins → Add plugin**：

1. 粘贴一个 GitHub 仓库（`owner/repo`）、HTTPS Git URL，或本地目录。
2. 如果该 Source 中包含多个 plugin，请先选择一个，然后再选择兼容的工具。
3. 检查变更内容并应用它们。

![Add plugin 对话框：发现的 plugin，以及兼容与不支持的 targets](/img/plugins-add-dialog.png)

大多数用户只需要一个仓库与目标复选框即可。**Advanced options** 可以新增
一个 Git ref，用于选择某个 release。Discovery 会分别显示每个 Target 的组件与
兼容性。当 OpenCode 因为其入口无法被检测而被列为不受支持时，该行上的
**Set entry path** 会取用已构建好的文件，并重新在 Source 中搜索，
同时保留你已选择的内容。安全的相对仓库符号链接会被保留。

同样的引导式流程也可以在终端中使用：

```bash
skillshare plugin add
```

Claude Code、Codex、Copilot、Antigravity CLI、Grok、Pi 或 OpenCode CLI 必须安装在**运行 Skillshare 后端的机器上**。
Codex 的话，Codex 桌面 app 内置的 CLI 也算；Codex 装在其他位置的机器，请设置 [`SKILLSHARE_CODEX_CLI`](/docs/reference/appendix/environment-variables#skillshare_codex_cli)。
Cursor 与 Antigravity 桌面版则会在其本地 plugin 目录中直接接收完整的文件。
运行在容器内的控制台，无法管理只安装在主机上的 plugin。
请改用本地 CLI，或让 Skillshare 与这些原生客户端运行在同一台机器上。

安装目标为 **Claude Code、Codex、Cursor、Antigravity Desktop、Antigravity CLI、
GitHub Copilot CLI、Pi 与 OpenCode**。Grok 在导入前需要先完成原生安装
与信任设置。Kimi、Hermes 与 Devin 的格式是可被发现的；但其自动
安装功能目前不可用，界面会说明原因。
请选择针对你的 Target 所发布的格式；Skillshare 不会在工具之间转换 plugin。
Project mode 支持 Claude、Antigravity Desktop、Pi 与 OpenCode。
Antigravity Desktop（`antigravity` 或 `agy`）与 CLI（`antigravity-cli`）使用各自独立的
存储。请选择你实际使用的那一个。

## 已经安装过了吗？

选择 **Import installed**，然后选取一个原生安装。Import 只会记录
该安装，而不会重新安装、复制其身份验证信息，或更改它在该 Agent 中是否
启用。Import 适用于 Claude、Codex、Copilot、Antigravity CLI、Grok、Pi 与 OpenCode；
Cursor 与 Antigravity 本地套件请改用 **Add plugin**。

```bash
skillshare plugin import review@team --from claude --no-tui
```

如果同一个逻辑套件在不同工具中使用不同的原生发行版，请针对每种
发行版使用相同的 `--name` 与对应的 Target，分别执行 add/import。
Skillshare 不会依据显示名称来推断等价关系。

导入绑定的是这台机器上的原生安装。要在另一台机器上使用同一个 plugin，请改为从它的
source 添加；请参阅 [Cross-Machine Sync — Plugins](/docs/how-to/sharing/cross-machine-sync#plugins)。

## 选择同步到哪里

每个受管理的绑定都有一个复选框。该复选框的含义是**在同步中包含此
Target**，而不是“在该 Agent 内部启用”。

- 勾选它，然后同步以安装缺失的 plugin。
- 打开某个 plugin 的行，也会列出（未勾选）该 Source 拥有对应套件的其他
  Agent。勾选其中一个会打开安装预览。Source 无法提供服务的 Agent，会在该行
  末尾计数显示，点击该数字即可查看原因。
- 添加 plugin 时可以一个 Agent 都不勾。它会留在 Skillshare 中并显示 **尚未选择 Agent**，在你于该行勾选 Agent 之前不会安装任何东西。
- 取消勾选，然后同步以移除该受管理的安装。
- 该套件定义仍会保留，你之后可以再次选取该 Target。
- 在 Claude 或 Codex 内部被停用的 plugin 会保持停用状态；请在该工具中
  管理其原生设置。

在控制台中，Plugins 页面右上角的 **Sync** 方框，会列出下一次同步将为
每个 Agent 安装或移除的内容。其按钮会打开预览；在你确认之前，任何
Agent 都不会发生变更。plugin 列表会立即显示，而方框下方的 **Agents**
栏位则会在各 Agent 的 CLI 逐一响应后陆续填入。
运行后，方框会列出结果：失败排在最前，接着是其他变更，结果相同的 Agent 合并成一行。没有变化的项目收成一行 **Unchanged**，展开后每个 plugin 一行，Pi 包单独列出。预览也用同样方式，把不会变动的项目收在要执行的变更下方。列表会显示每个 Pi 包的已安装版本，读取自 Pi 安装的包。

```bash
skillshare plugin disable review --target codex --no-tui
skillshare sync plugins --dry-run
skillshare sync plugins --no-tui
```

某个 plugin 的菜单中有 **View files**：这是其 Source 的经审核本地副本，
为只读，并以渲染后的 Markdown 呈现。被 import 的 plugin 没有本地副本，因此
也没有此选项。

Plugin 与一般的 Skill 及 MCP 同步是彼此独立的。它们所打包的
组件不会同时被复制进独立的 Skillshare Source 中。

## 更新与恢复 {#updates-and-recovery}

使用 **Check updates**，然后针对受支持的 Target 检视一次更新。Claude 可以透过其
原生 CLI 进行更新。Codex 会用经审核的快照再次 add 该 plugin；但若它在
Codex 中被停用则不会更新，因为 add 会把它重新启用；导入的 Codex plugin 则通过升级其 marketplace 来更新。
Cursor 与 Antigravity 会在检查本地编辑后替换受管理的本地副本。
Pi 与 OpenCode 会更新经审核的快照。Copilot 可以在保留已知启用状态的
同时刷新一份经审核的 Source。Antigravity CLI 与 Grok 的更新
仍留在原生工具中处理；被 import 套件的限制请参见命令参考文档。
更新无法处理的 Target 会被跳过并说明原因，该 plugin 的其他 Agent 仍会照常更新。

Pi 的 npm 包没有可比对的 source，所以 **Check updates** 会拿已安装版本和 npm 上的最新版本比较。发现新版本时，该行会显示 `old → new` 和 **Update** 按钮，检查结果里每个找到的 plugin 也各有一个；两者都会先打开该 plugin 的更新预览。

如果某个 Target 失败，结果仍会保留那些成功的部分。请先解决该原生
客户端的身份验证或配置问题，然后再次同步该 Target：

```bash
skillshare sync plugins review --target claude --no-tui
```

快照由 Skillshare 拥有。如果其内容已在外部被编辑过，Skillshare 会阻止
替换操作，以便你先保留那些编辑内容。Source 摘要（digest）用于侦测
变更；它们并不保证每一次原生安装或被 import 的 marketplace 都能在
不同机器间可重现。

如需自动化、范围细节与所有参数，请参见 [plugin](/docs/reference/commands/plugin)。
