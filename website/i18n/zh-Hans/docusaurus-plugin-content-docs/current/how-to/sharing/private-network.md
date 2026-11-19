---
sidebar_position: 5
---

# Machines on a Private Network

当你自己的多台机器已经可以通过 SSH 互相访问时（例如通过 Tailscale、WireGuard 或家庭网络），就可以在这些机器之间使用 skillshare，无需 GitHub 或任何托管服务。

本页涵盖三件事：

- [通过你自己某台机器上的 repository 同步](#ssh-remote)，而不是使用托管的 Git 服务
- 在眼前这台机器上[打开另一台机器的 dashboard](#remote-dashboard)
- 通过 MCP [向另一台机器上的 Agent 提供你的 skill](#remote-skills)

示例中把机器称为 `mac-mini` 和 `laptop`。请使用你的网络给它们的名称，例如 Tailscale MagicDNS 名称。

## Before You Start {#before-you-start}

确认 SSH 无需输入密码即可连接，并且对方机器能找到 skillshare：

```bash
ssh you@laptop skillshare version
```

通过 SSH 执行的命令不会加载你的交互式 shell 配置文件，因此可能找不到 skillshare 的安装目录。如果提示找不到命令，请使用那台机器上 `which skillshare` 输出的完整路径（例如在 Apple silicon 上用 Homebrew 安装时为 `/opt/homebrew/bin/skillshare`），或者把该目录加入非交互式 shell 会读取的文件中的 `PATH`，例如 `~/.zshenv`。

## 通过你某台机器上的 repository 同步 {#ssh-remote}

`push` 和 `pull` 适用于任何 Git remote，因此你自己某台机器上的 bare repository 可以取代 GitHub。

在存放 repository 的机器上（它也可以是你要同步的机器之一）：

```bash
git init --bare ~/skillshare.git
```

在第一台机器上，通过 SSH 把 skillshare 连接到它并 push：

```bash
skillshare init --remote you@mac-mini:skillshare.git
skillshare push
```

`you@mac-mini:skillshare.git` 是相对于该用户家目录的路径；使用完整路径的 `ssh://you@mac-mini/home/you/skillshare.git` 同样可行。在存放 repository 的那台机器上，可以改用本地路径（`~/skillshare.git`）。如果 skillshare 已经设置了其他 remote（例如 GitHub），`init` 不会改动它；请用 `git -C ~/.config/skillshare remote set-url origin you@mac-mini:skillshare.git` 切换，然后运行 `skillshare push`。

新建的 bare repository 会把 `master`（或你的 `init.defaultBranch`）作为默认分支，而这可能不是第一台机器推送的分支。其他机器从默认分支开始，所以请把它指向已推送的分支：

```bash
git --git-dir ~/skillshare.git branch                            # 已推送的分支，例如 main
git --git-dir ~/skillshare.git symbolic-ref HEAD refs/heads/main
```

如果跳过这一步，下一台机器上的 `init` 将拉取不到任何内容。

然后在其他每台机器上运行相同的 `init --remote`。之后就可以使用常规命令：

```bash
skillshare push    # 在修改了 skill 的机器上
skillshare pull    # 在另一台机器上；pull 也会运行 sync
```

若想不登录另一台机器就更新它，可以通过 SSH 运行它的 `pull`：

```bash
skillshare push && ssh you@laptop skillshare pull
```

[Cross-Machine Sync](/docs/how-to/sharing/cross-machine-sync) 中的其余内容都原样适用：首次 pull 的行为、冲突，以及 `push` 和 `pull` 会带走哪些内容。

:::tip
这个 bare repository 现在是你的 skill 唯一一份不在你编辑用的机器上的副本。请像备份其他数据一样备份它，或者再添加第二个 remote，例如私有的托管 repository。
:::

## 打开另一台机器的 dashboard {#remote-dashboard}

每台机器都有自己的 dashboard，并且只管理那台机器。从其他位置打开它有两种方式。

### 通过 SSH 隧道

对方机器上没有任何服务监听本机以外的地址。先在后台启动它的 dashboard，再把一个本地端口转发过去：

```bash
ssh you@laptop skillshare ui start
ssh -N -L 19421:127.0.0.1:19420 you@laptop
```

如果该 dashboard 已经在私有地址上运行（见下文），`ui start` 会报告它正运行在那个地址上；请改为转发到那个地址，例如 `-L 19421:100.101.102.103:19420`。

打开 `http://localhost:19421`。使用本地端口 `19421`，是为了避免与这台机器上已在 `19420` 运行的 dashboard 冲突。

### 使用私有网络地址

让 dashboard 监听你的私有网络分配给那台机器的地址，例如它的 Tailscale IP（`tailscale ip -4`）：

```bash
skillshare ui start --host 100.101.102.103
```

在网络中的任何机器上打开 `http://100.101.102.103:19420`。dashboard 会直接响应 IP 地址。如果想改用名称打开，请用该名称启动它，或者信任你输入的确切地址：

```bash
skillshare ui start --host laptop.example.ts.net
skillshare ui start --host 100.101.102.103 --trusted-origin http://laptop:19420
```

任何其他名称都会得到 403，以保护 dashboard 免受 DNS rebinding 攻击。完整规则请参阅 [反向代理](/docs/reference/commands/ui#reverse-proxy)。

:::warning
dashboard 没有登录机制。任何能访问其地址的人都可以使用全部功能，包括会写入 Agent 所执行命令的 MCP、Hooks 与 Plugins 页面。请只把它绑定到你所控制的网络上的地址，限制谁能访问它（例如使用 Tailscale 访问规则），并且不要在会加入不受信任网络的机器上使用 `--host 0.0.0.0`。
:::

从 dashboard 打开的编辑器会在 dashboard 所在的机器上启动，而不是在你浏览的这台机器上。

## 向另一台机器上的 Agent 提供你的 skill {#remote-skills}

当 Agent 运行在一台没有自己的 skillshare 设置的机器上时，它可以通过网络从 `skillshare mcp serve` 读取你的 skill，而不是使用同步的文件夹：

```bash
SKILLSHARE_MCP_TOKEN=change-me skillshare mcp serve --http 100.101.102.103:8765 --http-allow-insecure
```

只在自身会加密流量的网络（例如 Tailscale 或 WireGuard）上使用 `--http-allow-insecure`；其他情况请提供 HTTPS。把 Agent 的 MCP client 指向 `http://100.101.102.103:8765/`，并以该 token 作为 bearer token。传输规则、TLS 选项以及提供的内容，请参阅 [通过 MCP 提供 skill](/docs/reference/commands/mcp#serve-skills-over-mcp)。

## Limits {#limits}

- `push` 和 `pull` 会带走 skills、agents 和 extras，但不包括每台机器的 `config.yaml`。Plugins、MCP server 和 hooks 需要在每台机器上各自设置；请参阅 [Plugins, MCP and Hooks](/docs/how-to/sharing/cross-machine-sync#plugins-mcp-hooks)。
- 用户名不同的机器会保存不同的家目录路径。如果你用其他方式共享 `config.yaml`，请开启 `preserve_tilde_on_save`（参见 [Configuration](/docs/reference/targets/configuration)）。
- 在 macOS 上，通过 SSH 启动的命令可能无法访问登录钥匙串。如果 `ssh you@laptop skillshare pull` 因 HTTPS remote 的凭据而失败，请为那台机器改用 SSH remote 或 token 环境变量。

## See Also

- [Cross-Machine Sync](/docs/how-to/sharing/cross-machine-sync) — 通过托管的 Git 服务同步
- [ui](/docs/reference/commands/ui) — Dashboard 参数与后台服务
- [mcp](/docs/reference/commands/mcp#serve-skills-over-mcp) — 通过 MCP 提供 skill
- [init](/docs/reference/commands/init) — 以 `--remote` 执行 init
