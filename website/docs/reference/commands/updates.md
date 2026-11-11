---
sidebar_position: 4
---

# updates

Control automatic version checks and update notifications for the CLI, built-in skill,
web dashboard and desktop app. This is a **global preference**, including in project mode.

## Usage

```bash
skillshare updates             # Show the effective preference
skillshare updates off         # Disable automatic checks and notifications
skillshare updates on          # Restore automatic checks
skillshare updates check       # Explicitly check the CLI version now
skillshare updates --json      # Read the saved and effective preference
```

| Option | Description |
|--------|-------------|
| `on` / `off` | Save the global preference |
| `check` | Check the CLI version now, bypassing the preference and version cache |
| `--json` | Output JSON |
| `--help`, `-h` | Show help |

The JSON preference has `enabled` (effective), `configured` (saved, or the default),
and `envOverride` fields. Setting a preference does not change an environment override.

## Configuration {#configuration}

Automatic checks are enabled by default. To opt out, set this in the global `config.yaml`:

```yaml
updates:
  auto_check: false
```

Or override it for a process and its children:

```bash
SKILLSHARE_AUTO_UPDATE_CHECK=false skillshare status
```

Precedence: **environment variable > config.yaml > true**. Accepted values are
`true`/`false`, `1`/`0`, `t`/`f` and their Go boolean case variants.
Empty or invalid values are ignored. Environment overrides are never written back.

With checks disabled, ordinary commands and `doctor` do not check release versions
or show update notifications, even when an older check cached a newer release.
The web UI does not automatically show an update dialog or version-status badges.
An already-open dashboard refreshes the local preference every five seconds (without
checking releases), so turning checks off from the CLI or app also hides existing prompts.

In Web **Settings → General** and desktop **Settings → Updates**, the automatic-check
switch writes the same global preference. An environment override locks the switch.
The desktop app refreshes external preference changes within a minute and hides
version badges as well as native version notifications. Its CLI must support
`updates` to read/write the saved preference; older CLIs keep the existing default
and require a CLI upgrade to enable the switch. Environment opt-out still works.

## Manual checks and upgrades {#manual-checks}

`skillshare updates check`, Web Settings' **Check for updates**, desktop **Check**,
and [upgrade](/docs/reference/commands/upgrade) remain available.
Manual results are shown where requested, without re-enabling automatic notifications.
Desktop skill, agent, plugin and repository source checks are not release-version checks
and are unchanged.

See [environment variables](/docs/reference/appendix/environment-variables).
