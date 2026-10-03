---
sidebar_position: 5
---

# completion

生成并安装 shell 补全脚本，为命令、子命令和标志提供 Tab 补全。

## 何时使用

- 你希望为 skillshare 命令和标志启用 Tab 补全
- 你设置了一台新机器或新的 shell 环境
- 你使用别名（例如 `ss`）并希望补全能与它配合工作

## 语法

```bash
# 自动安装（推荐）
skillshare completion bash --install
skillshare completion zsh --install
skillshare completion fish --install
skillshare completion powershell --install
skillshare completion nushell --install

# 将脚本输出到 stdout（进阶用法）
skillshare completion bash
skillshare completion zsh > ~/.zsh/completions/_skillshare
```

## 支持的 Shell

| Shell | 安装路径 |
|-------|-------------|
| bash | `~/.local/share/bash-completion/completions/skillshare` |
| zsh | `~/.zsh/completions/_skillshare` |
| fish | `~/.config/fish/completions/skillshare.fish` |
| powershell | `~/.config/powershell/completions/skillshare.ps1` |
| nushell | `~/.config/nushell/completions/skillshare.nu` |

## 标志

| 标志 | 描述 |
|------|-------------|
| `--install` | 将补全脚本写入标准安装路径 |
| `--help`, `-h` | 显示用法 |

## 补全范围

生成的脚本为以下内容提供 Tab 补全：

- **命令**——所有顶级命令（`sync`、`install`、`list` 等）
- **子命令**——`target add/remove/list`、`trash list/restore/delete/empty`、`hub add/list/remove/default/index`、`extras init/list/remove/collect/source`、`audit rules disable/enable/severity/reset/init`、`backup files list/show/restore`、`ui start/stop`、`hooks add/disable/edit/enable/import/list/remove/restore/sync`
- **标志**——各命令的标志及其简写形式（`--dry-run`/`-n`、`--force`/`-f` 等）
- **全局标志**——`--project`/`-p`、`--global`/`-g`

## 别名支持

bash、zsh 和 PowerShell 脚本会自动检测指向 `skillshare` 的别名，并为它们注册补全：

```bash
alias ss=skillshare
source <(skillshare completion bash)
ss sy<Tab>  # → ss sync
```

Fish 的别名（其本质是函数包装器）会自动继承补全。

对于 Nushell，在配置中使用 `alias ss = skillshare` 时，别名补全会被原生继承。

## 安装后的步骤

执行 `--install` 后，你可能需要激活补全：

### Bash

```bash
# 重启 shell，或者：
source ~/.local/share/bash-completion/completions/skillshare
```

### Zsh

添加到你的 `.zshrc`（如果尚未添加）：

```bash
fpath=(~/.zsh/completions $fpath)
autoload -Uz compinit && compinit
```

然后重启 shell 或运行 `exec zsh`。

### Fish

在新的 fish 会话中会自动可用补全。

### PowerShell

添加到你的 PowerShell profile 中（`echo $PROFILE`）：

```powershell
. ~/.config/powershell/completions/skillshare.ps1
```

### Nushell

添加到你的 Nushell 配置中（`$nu.config-path`）：

```nu
source ~/.config/nushell/completions/skillshare.nu
```

## 示例

```
$ skillshare completion bash --install
✓ Completion installed to ~/.local/share/bash-completion/completions/skillshare

Next
  source ~/.local/share/bash-completion/completions/skillshare  load it in this shell, or restart it
```

## 另请参阅

- [doctor](./doctor.md) — 诊断环境问题
- [version](./version.md) — 显示 CLI 版本
