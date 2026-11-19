---
sidebar_position: 5
---

# Machines on a Private Network

Use skillshare across your own machines when they already reach each other over SSH, for example through Tailscale, WireGuard, or a home network. You do not need GitHub or any hosted service.

This page covers three things:

- [Sync through a repository on one of your machines](#ssh-remote) instead of a hosted Git service
- [Open another machine's dashboard](#remote-dashboard) from the machine in front of you
- [Serve your skills to Agents on another machine](#remote-skills) over MCP

The examples call the machines `mac-mini` and `laptop`. Use the names your network gives them, such as Tailscale MagicDNS names.

## Before You Start {#before-you-start}

Check that SSH works without a password prompt and that the other machine finds skillshare:

```bash
ssh you@laptop skillshare version
```

A command run over SSH does not load your interactive shell profile, so it can miss the directory skillshare is installed in. If the command is not found, use the full path that `which skillshare` prints on that machine (for example `/opt/homebrew/bin/skillshare` with Homebrew on Apple silicon), or add that directory to `PATH` in a file non-interactive shells read, such as `~/.zshenv`.

## Sync Through a Repository on One of Your Machines {#ssh-remote}

`push` and `pull` work with any Git remote, so a bare repository on one of your machines can replace GitHub.

On the machine that keeps the repository (it can also be one of the machines you sync):

```bash
git init --bare ~/skillshare.git
```

On the first machine, connect skillshare to it over SSH and push:

```bash
skillshare init --remote you@mac-mini:skillshare.git
skillshare push
```

`you@mac-mini:skillshare.git` is relative to that user's home directory; `ssh://you@mac-mini/home/you/skillshare.git` with a full path works as well. On the machine that holds the repository, you can give the local path (`~/skillshare.git`) instead. If skillshare is already set up with another remote, such as GitHub, `init` leaves it alone; switch with the command `init` prints, such as `git -C ~/.config/skillshare/skills remote set-url origin you@mac-mini:skillshare.git`, then `skillshare push`.

A new bare repository names `master` (or your `init.defaultBranch`) as its default branch, which may not be the branch the first machine pushed. Other machines start from the default branch, so point it at the pushed one:

```bash
git --git-dir ~/skillshare.git branch                            # the pushed branch, such as main
git --git-dir ~/skillshare.git symbolic-ref HEAD refs/heads/main
```

Without this step, `init` on the next machine finds nothing to pull.

Then run the same `init --remote` on each other machine. From then on the usual commands apply:

```bash
skillshare push    # on the machine where you changed skills
skillshare pull    # on another machine; pull also runs sync
```

To update another machine without logging in to it, run its `pull` over SSH:

```bash
skillshare push && ssh you@laptop skillshare pull
```

Everything else in [Cross-Machine Sync](/docs/how-to/sharing/cross-machine-sync) applies unchanged: first-pull behavior, conflicts, and what `push` and `pull` carry.

:::tip
The bare repository is now the only copy of your skills that is not on a machine you edit. Back it up like any other data, or add a second remote such as a private hosted repository.
:::

## Open Another Machine's Dashboard {#remote-dashboard}

Each machine has its own dashboard, and it manages only that machine. There are two ways to open it from elsewhere.

### Through an SSH tunnel

Nothing on the other machine listens beyond itself. Start its dashboard in the background, then forward a local port to it:

```bash
ssh you@laptop skillshare ui start
ssh -N -L 19421:127.0.0.1:19420 you@laptop
```

If that dashboard is already running on a private address (see below), `ui start` reports it running there; forward to that address instead, such as `-L 19421:100.101.102.103:19420`.

Open `http://localhost:19421`. Local port `19421` avoids a clash with a dashboard already running on this machine at `19420`.

### On the private network address

Start the dashboard on the address your private network gives that machine, such as its Tailscale IP (`tailscale ip -4`):

```bash
skillshare ui start --host 100.101.102.103
```

Open `http://100.101.102.103:19420` from any machine on the network. The dashboard answers at an IP address as is. To open it by name instead, start it with that name, or trust the exact address you type:

```bash
skillshare ui start --host laptop.example.ts.net
skillshare ui start --host 100.101.102.103 --trusted-origin http://laptop:19420
```

Any other name gets 403, which protects the dashboard from DNS rebinding. See [Reverse Proxy](/docs/reference/commands/ui#reverse-proxy) for the full rules.

:::warning
The dashboard has no login. Anyone who can reach its address can use all of it, including the MCP, Hooks, and Plugins pages that write commands Agents run. Bind it only to an address on a network you control, restrict who reaches it (for example with Tailscale access rules), and do not use `--host 0.0.0.0` on a machine that joins networks you do not trust.
:::

An editor opened from the dashboard starts on the dashboard's machine, not on the one you browse from.

## Serve Your Skills to Agents on Another Machine {#remote-skills}

When an Agent runs on a machine that has no skillshare setup of its own, it can read your skills from `skillshare mcp serve` over the network instead of a synced folder:

```bash
SKILLSHARE_MCP_TOKEN=change-me skillshare mcp serve --http 100.101.102.103:8765 --http-allow-insecure
```

Use `--http-allow-insecure` only on a network that encrypts traffic itself, such as Tailscale or WireGuard; elsewhere serve HTTPS. Point the Agent's MCP client at `http://100.101.102.103:8765/` with the token as a bearer token. See [Serve skills over MCP](/docs/reference/commands/mcp#serve-skills-over-mcp) for the transport rules, TLS options, and what is served.

## Limits {#limits}

- `push` and `pull` carry skills, agents, and extras, not each machine's `config.yaml`. Plugins, MCP servers, and hooks need their own setup on each machine; see [Plugins, MCP and hooks](/docs/how-to/sharing/cross-machine-sync#plugins-mcp-hooks).
- Machines with different user names save different home paths. If you share `config.yaml` some other way, turn on `preserve_tilde_on_save` (see [Configuration](/docs/reference/targets/configuration)).
- On macOS, a command started over SSH may not reach the login keychain. If `ssh you@laptop skillshare pull` fails on credentials for an HTTPS remote, use an SSH remote or a token environment variable for that machine.

## See Also

- [Cross-Machine Sync](/docs/how-to/sharing/cross-machine-sync) — Sync through a hosted Git service
- [ui](/docs/reference/commands/ui) — Dashboard flags and background server
- [mcp](/docs/reference/commands/mcp#serve-skills-over-mcp) — Serve skills over MCP
- [init](/docs/reference/commands/init) — Init with `--remote`
