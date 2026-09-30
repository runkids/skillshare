---
sidebar_position: 5
---

# completion

Generate and install shell completion scripts for tab-completion of commands, subcommands, and flags.

## When to Use

- You want tab-completion for skillshare commands and flags
- You set up a new machine or shell environment
- You use an alias (e.g. `ss`) and want completions to work with it

## Synopsis

```bash
# Auto-install (recommended)
skillshare completion bash --install
skillshare completion zsh --install
skillshare completion fish --install
skillshare completion powershell --install
skillshare completion nushell --install

# Output script to stdout (advanced)
skillshare completion bash
skillshare completion zsh > ~/.zsh/completions/_skillshare
```

## Supported Shells

| Shell | Install Path |
|-------|-------------|
| bash | `~/.local/share/bash-completion/completions/skillshare` |
| zsh | `~/.zsh/completions/_skillshare` |
| fish | `~/.config/fish/completions/skillshare.fish` |
| powershell | `~/.config/powershell/completions/skillshare.ps1` |
| nushell | `~/.config/nushell/completions/skillshare.nu` |

## Flags

| Flag | Description |
|------|-------------|
| `--install` | Write completion script to the standard install path |
| `--help`, `-h` | Show usage |

## Completion Scope

The generated scripts provide tab-completion for:

- **Commands** — all top-level commands (`sync`, `install`, `list`, etc.)
- **Subcommands** — `target add/remove/list`, `trash list/restore/delete/empty`, `hub add/list/remove/default/index`, `extras init/list/remove/collect/source`, `audit rules disable/enable/severity/reset/init`, `backup files list/show/restore`, `ui start/stop`, `hooks add/disable/edit/enable/import/list/remove/restore/sync`
- **Flags** — per-command flags with short forms (`--dry-run`/`-n`, `--force`/`-f`, etc.)
- **Global flags** — `--project`/`-p`, `--global`/`-g`

## Alias Support

The bash, zsh, and PowerShell scripts automatically detect aliases pointing to `skillshare` and register completions for them:

```bash
alias ss=skillshare
source <(skillshare completion bash)
ss sy<Tab>  # → ss sync
```

Fish aliases (which are function wrappers) inherit completions automatically.

For Nushell, alias completions are inherited natively when using `alias ss = skillshare` in your config.

## Post-Install Steps

After `--install`, you may need to activate the completions:

### Bash

```bash
# Restart your shell, or:
source ~/.local/share/bash-completion/completions/skillshare
```

### Zsh

Add to your `.zshrc` (if not already present):

```bash
fpath=(~/.zsh/completions $fpath)
autoload -Uz compinit && compinit
```

Then restart your shell or run `exec zsh`.

### Fish

Completions are available automatically in new fish sessions.

### PowerShell

Add to your PowerShell profile (`echo $PROFILE`):

```powershell
. ~/.config/powershell/completions/skillshare.ps1
```

### Nushell

Add to your Nushell config (`$nu.config-path`):

```nu
source ~/.config/nushell/completions/skillshare.nu
```

## Example

```
$ skillshare completion bash --install
✔ Completion script installed to /home/user/.local/share/bash-completion/completions/skillshare

ℹ Restart your shell or run:
  source /home/user/.local/share/bash-completion/completions/skillshare
```

## See Also

- [doctor](./doctor.md) — Diagnose environment issues
- [version](./version.md) — Show CLI version
