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

## 面板功能

面板跟随 Herdr 显示的 space，在该 space 的机器上、聚焦中 terminal 的文件夹里运行 skillshare。

- **Resources** 列出 skills、agents、MCP servers、plugins 和 hooks，分为“Updates available”“Needs sync”“In sync”三组。在有 `.skillshare/config.yaml` 的项目中，可以用 All · Project · Global 切换 scope。
- **行**会就地展开。skills 和 agents 提供 Update、Diff、Uninstall，以及“Ask &lt;agent&gt; to audit…”：确认后把该项交给 Herdr 中聚焦的 agent。 顶层声明的 hooks 提供 Enable、Disable 和 Remove，每项都先确认；Sync 会应用保存的更改。
- hooks 计数包含具名条目和待清理条目；诊断通知使用相同筛选条件，零条目时也会单独显示。文件已同步时仍显示无法运行的原因，嵌套行显示完整项目路径。仅原生文件需要更改时也提供 Sync；hooks 冲突会阻止所有资源同步，需先在 CLI 或仪表板中解决。 预览失败时，条目会在分组外显示同步状态未知，每个范围显示一条错误通知。
- **Install** 接受仓库 URL。包含多个 skills 的仓库会列出清单供你挑选。面板会先 clone，对 clone 运行 [`skillshare audit`](/docs/reference/commands/audit)，通过才安装。audit 拦下时会显示 findings，并提供“Install anyway”（`--force`）和“Ask &lt;agent&gt; to review…”。
- **Source** 显示 source 仓库的状态，对话框提供 Pull、Commit and push 和 Sync both ways。pull 遇到两台机器都改过的文件时，可以从左右对照预览逐个文件选择要保留的版本。`.metadata.json` 会逐条合并，与 [`skillshare pull`](/docs/reference/commands/pull) 的做法相同。
- **Machines** 显示每台 Herdr 机器的 skillshare 版本、source 状态和 doctor 警告，提供 Pull and sync；有新版本时也提供 Update。

面板显示时每 20 秒刷新一次，每 10 分钟运行 `skillshare check`，隐藏时不做任何事。

## 相关

- [Sync](/docs/reference/commands/sync)
- [Push](/docs/reference/commands/push) 和 [Pull](/docs/reference/commands/pull)
- [Audit](/docs/reference/commands/audit)
