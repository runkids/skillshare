---
sidebar_position: 8
---

# 在 Herdr 中使用 skillshare

> 在 agents 旁边放一个 skillshare 面板：不必离开 [Herdr](https://herdr.dev)，就能看到哪些需要更新或 sync，并直接处理。

![Herdr 中的 skillshare 面板](/img/herdr-panel-demo.png)

## 前提条件

- Herdr 0.1.12 及以上
- Herdr 连接的每台机器都已安装 skillshare，位置在 `~/.local/bin`、`~/go/bin`、`/opt/homebrew/bin` 或 `/usr/local/bin`

## 设置

### Mod：保留当前 layout（推荐）

使用独立的 [Mod 分享包](https://github.com/runkids/skillshare/tree/main/herdr/skillshare-mod)，添加面板而不替换整个窗口。把下面这段 prompt 贴给 Herdr 里的 agent：

```text
Read ~/.config/herdr-app/AGENTS.md, then add this mod from https://github.com/runkids/skillshare/tree/main/herdr/skillshare-mod. Follow its RECIPE.md: add only the skillshare toggle and panel to my current layout, preserving my status bar, usage meters and all other customizations. Reuse existing skillshare nodes instead of duplicating them.
```

这个分享包没有 `layouts/` 文件夹。Recipe 只添加或复用 skillshare 开关和面板，保留你的状态栏、CLI 额度、主题及其他自定义设置。Herdr 内置的 layout 会先复制再修改。

### 从 Herdr layouts 安装

这个 layout 已上架到 [Herdr layouts](https://herdr.dev/layouts/runkids/skillshare--herdr--skillshare/)。把下面这段 prompt 贴给 Herdr 里的 agent：

```text
Read ~/.config/herdr-app/AGENTS.md, then add this layout: herdr.dev/layouts/runkids/skillshare--herdr--skillshare
```

agent 会先读完所有文件再添加 layout，你自己的 layout 不会被改动。用 `ctrl-shift-l` 切换 layout。

### 手动安装

面板位于 skillshare 仓库的 [`herdr/skillshare`](https://github.com/runkids/skillshare/tree/main/herdr/skillshare)。

1. 把 `mods/skillshare/` 复制到 `~/.config/herdr-app/mods/skillshare/`。
2. 把 `layouts/herdr-skillshare.json` 和 `layouts/herdr-skillshare.png` 复制到 `~/.config/herdr-app/layouts/`，再在 Herdr 设置中切换到 `herdr-skillshare` layout。想保留自己的 layout，就按文件夹里 `README.md` 列出的两个 block 加进去。
3. 运行 `herdr-app check ~/.config/herdr-app/mods/skillshare`。

tab bar 会出现 **skillshare** 按钮，用来显示或隐藏面板。切换 space 时，面板会保持原来的开关状态。

## 更新面板

无论使用哪种安装方式，请 agent 更新时，都只备份并替换已安装的 skillshare mod。保持当前选中的 layout，保留所有 layout、自定义状态栏、额度显示、设置、快捷键及其他 mod。如需调整 layout，只合并 skillshare 自己的节点，不重新应用完整 layout。

面板显示时，每小时向 GitHub 查询一次最新的 skillshare release。有新的 release 发布时，面板标题栏会出现 **Update panel**。它会先询问，再用该 release 附带的文件替换 `~/.config/herdr-app/mods/skillshare/` 里的文件，随后 Herdr 会重新加载面板。被替换的文件保留在该文件夹的 `.bak-update-<version>`；你对这些文件做的修改不会保留。你的 layout 不会被改动。

在这个按钮出现之前安装的面板不会检查更新。请手动把最新 release 的 `mods/skillshare/` 复制到 `~/.config/herdr-app/mods/skillshare/` 一次；之后的更新就能用按钮完成。

## 面板功能

面板跟随 Herdr 显示的 space，在该 space 的机器上、聚焦中 terminal 的文件夹里运行 skillshare。

- **Overview** 先给出答案：“Everything is in sync”，或者有多少个 target 需要同步，并提供一个 Sync 按钮。下面列出 source 中每种资源的数量，以及通向每个 target 的一行，可以看出哪些 target 在等待同步。标签页其余部分只列出需要你处理的内容：等待同步的、有更新的，以及 hooks 通知。在有 `.skillshare/config.yaml` 的项目中，标签页旁边的 All · Project · Global 可以切换 scope。
- **Library** 列出 skills、agents、MCP servers、plugins 和 hooks，分为“Needs sync”“Update available”“In sync”三组，已禁用的项目归入“Off”。对于 skills 和 agents，“In sync”会把同一个仓库的项目收进可折叠的分组；切换到 List 则显示为平铺列表。
- **行**会打开预览。skill 显示其 `SKILL.md` 和文件，MCP server 显示隐藏了密钥值的设置，预览的 Targets 标签页显示各 target 的状态。skills 和 agents 提供 Update、Diff、Disable（已禁用的则是 Enable；下次 sync 时应用）、Uninstall，以及“Ask &lt;agent&gt; to audit…”：确认后把该项交给 Herdr 中聚焦的 agent。顶层声明的 hooks 提供 Enable、Disable 和 Remove，每项都先确认；Sync 会应用保存的更改。
- hooks 计数包含具名条目和待清理条目；诊断通知有单独的行。hooks 冲突会阻止所有资源同步，需先在 CLI 或仪表板中解决。无法读取计划时，条目会在分组外显示同步状态未知。
- **Install** 接受仓库 URL。包含多个 skills 的仓库会列出清单供你挑选。面板会先 clone，对 clone 运行 [`skillshare audit`](/docs/reference/commands/audit)，通过才安装。audit 拦下时会显示 findings，并提供“Install anyway”（`--force`）和“Ask &lt;agent&gt; to review…”。
- **Source** 显示 source 仓库的状态，对话框提供 Pull、Commit and push 和 Sync both ways。pull 遇到两台机器都改过的文件时，可以从左右对照预览逐个文件选择要保留的版本。`.metadata.json` 会逐条合并，与 [`skillshare pull`](/docs/reference/commands/pull) 的做法相同。
- **Machines** 显示每台 Herdr 机器的 skillshare 版本、remote、source 和 target 状态以及 doctor 警告，提供 Pull and sync；有新版本时也提供 Update。有未提交或未推送更改的机器会改为提供 Sync both ways，因为普通的 pull 会被拒绝。

面板显示时每 20 秒刷新一次，每 10 分钟运行 `skillshare check`，隐藏时不做任何事。

## 相关

- [Sync](/docs/reference/commands/sync)
- [Push](/docs/reference/commands/push) 和 [Pull](/docs/reference/commands/pull)
- [Audit](/docs/reference/commands/audit)
