---
sidebar_position: 5
---

# completion

產生並安裝 shell 的補全（completion）腳本，讓 commands、subcommands 與 flags 都能使用 Tab 補全。

## When to Use

- 你想要讓 skillshare 的 command 與 flag 支援 Tab 補全
- 你正在設定新機器或 shell 環境
- 你使用別名（例如 `ss`）並希望補全也能對它生效

## Synopsis

```bash
# 自動安裝（建議）
skillshare completion bash --install
skillshare completion zsh --install
skillshare completion fish --install
skillshare completion powershell --install
skillshare completion nushell --install

# 把腳本輸出到 stdout（進階用法）
skillshare completion bash
skillshare completion zsh > ~/.zsh/completions/_skillshare
```

## Supported Shells

| Shell | 安裝路徑 |
|-------|-------------|
| bash | `~/.local/share/bash-completion/completions/skillshare` |
| zsh | `~/.zsh/completions/_skillshare` |
| fish | `~/.config/fish/completions/skillshare.fish` |
| powershell | `~/.config/powershell/completions/skillshare.ps1` |
| nushell | `~/.config/nushell/completions/skillshare.nu` |

## Flags

| Flag | Description |
|------|-------------|
| `--install` | 把補全腳本寫入標準安裝路徑 |
| `--help`, `-h` | 顯示用法 |

## Completion Scope

產生的腳本提供以下項目的 Tab 補全：

- **Commands** — 所有頂層 command（`sync`、`install`、`list` 等）
- **Subcommands** — `target add/remove/list`、`trash list/restore/delete/empty`、`hub add/list/remove/default/index`、`extras init/list/remove/collect/source`、`audit rules disable/enable/severity/reset/init`、`backup files list/show/restore`、`ui start/stop`、`hooks add/disable/edit/enable/import/list/remove/restore/sync`
- **Flags** — 每個 command 的 flag 及其簡寫（`--dry-run`/`-n`、`--force`/`-f` 等）
- **Global flags** — `--project`/`-p`、`--global`/`-g`

## Alias Support

bash、zsh，以及 PowerShell 的腳本會自動偵測指向 `skillshare` 的別名，並為它們註冊補全：

```bash
alias ss=skillshare
source <(skillshare completion bash)
ss sy<Tab>  # → ss sync
```

Fish 的別名（本質上是函式包裝器）會自動繼承補全功能。

在 Nushell 中，只要在設定中使用 `alias ss = skillshare`，別名補全就會原生繼承。

## Post-Install Steps

執行 `--install` 之後，你可能需要啟用補全功能：

### Bash

```bash
# 重新啟動你的 shell，或：
source ~/.local/share/bash-completion/completions/skillshare
```

### Zsh

加入你的 `.zshrc`（如果尚未加入）：

```bash
fpath=(~/.zsh/completions $fpath)
autoload -Uz compinit && compinit
```

接著重新啟動你的 shell，或執行 `exec zsh`。

### Fish

新的 fish session 會自動套用補全功能。

### PowerShell

加入你的 PowerShell profile（`echo $PROFILE`）：

```powershell
. ~/.config/powershell/completions/skillshare.ps1
```

### Nushell

加入你的 Nushell 設定（`$nu.config-path`）：

```nu
source ~/.config/nushell/completions/skillshare.nu
```

## Example

```
$ skillshare completion bash --install
✓ Completion installed to ~/.local/share/bash-completion/completions/skillshare

Next
  source ~/.local/share/bash-completion/completions/skillshare  load it in this shell, or restart it
```

## See Also

- [doctor](./doctor.md) — 診斷環境問題
- [version](./version.md) — 顯示 CLI 版本
