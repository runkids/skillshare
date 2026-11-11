# CLI E2E Runbook: Automatic Version Check Preference

Verify issue #846: global opt-out, environment precedence, silent doctor output and manual check availability.

## Scope

- Global preference persists even when called from a project.
- A process override does not change the saved preference.
- CLI/doctor and the Web version API do not expose cached update notifications while disabled.
- Manual checks and upgrades remain supported.

## Environment

Run in the devcontainer with a fresh ssenv. Use a current release-version build for the optional live-network manual check.

## Steps

### 1. Save the opt-out

```bash
ss updates off --json
```

**Expected:**
- jq: .enabled == false and .configured == false and .envOverride == false

### 2. Environment override wins without changing the saved value

```bash
SKILLSHARE_AUTO_UPDATE_CHECK=true ss updates --json
```

**Expected:**
- jq: .enabled == true and .configured == false and .envOverride == true

### 3. Doctor stays quiet about newer versions

```bash
ss updates off --json >/dev/null
ss doctor -g --json > "$HOME/updates-doctor.json"
result=$?
test "$result" -le 1
jq '{quiet: (.version.update_available == false and (.version.latest // "") == "")}' "$HOME/updates-doctor.json"
```

**Expected:**
- jq: .quiet == true

### 4. Turn checks back on

```bash
ss updates on --json
```

**Expected:**
- jq: .enabled == true and .configured == true and .envOverride == false

### 5. Manual check and upgrade commands remain discoverable

```bash
ss updates --help
ss upgrade --help
```

**Expected:**
- Check the CLI version now
- skillshare upgrade

## Pass Criteria

All automated steps pass. Separately inspect Web Settings and desktop Updates:
disable the switch, reload, confirm no update dialog/native notification/version badge,
then use the explicit Check action and confirm its result does not re-enable alerts.
