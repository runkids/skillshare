# CLI E2E Runbook: Interrupted Backup Staging Cleanup

Ref: [GitHub issue #556](https://github.com/runkids/skillshare/issues/556)

## Scope

- Manual cleanup reclaims `.snapshot-*` even when no completed snapshot exists.
- Listing excludes staging; dry-run includes its reclaimed bytes without changing files or creating a lock file.
- Cleanup preserves a recent completed snapshot beside orphaned staging.
- A real backup process holds the lock during copying; manual cleanup and dry-run report busy rather than deleting its staging.
- Killing that process releases the lock and makes its partial copy reclaimable.
- Plain-text global sync runs automatic cleanup after its pre-sync backup.

This runbook covers global skills backups on Linux. It does not validate
project or agent backup flows, Windows locking, the Web API, or dashboard rendering.

## Environment

Run inside the devcontainer. Requires Python 3 and mdproof. The repository's
`ai_docs/tests/mdproof.json` builds the current CLI and provides per-runbook HOME
isolation. Each step additionally uses a fresh directory beneath that HOME for
its config and data; steps do not share fixtures or modify real user backups.
Use mdproof directly, not nested inside an initialized ssenv. Override the
shared setup and teardown with `:`: these self-contained steps need no global
initialization, and must not clean the shared `/tmp` fixtures of other runbooks.

```text
docker exec "$CONTAINER" env SKILLSHARE_DEV_ALLOW_WORKSPACE_PROJECT=1 \
  SKILLSHARE_DEV_USE_BINARY=1 mdproof --setup ':' --teardown ':' --report json \
  /workspace/ai_docs/tests/backup_staging_cleanup_runbook.md
```

Fixtures remain within the runbook's temporary HOME. The only forced process
termination targets the child backup created by Step 3. It uses the freshly
built binary directly so the PID belongs to the CLI, not a build wrapper.

## Step 1: Staging-only listing, non-mutating preview, and cleanup

A seven-byte partial file makes reclaimed-size reporting exact. This legacy
backup root initially has no `.backup.lock`.

```bash
set -euo pipefail
ROOT=$(mktemp -d "$HOME/backup-staging-only.XXXXXX")
export XDG_CONFIG_HOME="$ROOT/config" XDG_DATA_HOME="$ROOT/data"
export XDG_STATE_HOME="$ROOT/state" XDG_CACHE_HOME="$ROOT/cache"
unset SKILLSHARE_CONFIG
BACKUPS="$XDG_DATA_HOME/skillshare/backups"
mkdir -p "$BACKUPS/.snapshot-orphan"
printf partial > "$BACKUPS/.snapshot-orphan/partial.txt"

ss backup -g --list > "$ROOT/list.txt"
grep -F 'No backups found' "$ROOT/list.txt"
! grep -F '.snapshot-orphan' "$ROOT/list.txt" || exit 1
ss backup -g --cleanup --dry-run > "$ROOT/preview.txt"
grep -F 'Would remove 1 backup directory, freeing 7 B' "$ROOT/preview.txt"
test "$(cat "$BACKUPS/.snapshot-orphan/partial.txt")" = partial
test ! -e "$BACKUPS/.backup.lock"
ss backup -g --cleanup > "$ROOT/cleanup.txt"
grep -F 'Removed 1 backup directory, freed 7 B' "$ROOT/cleanup.txt"
grep -F '0 backups left' "$ROOT/cleanup.txt"
test ! -e "$BACKUPS/.snapshot-orphan"
test -f "$BACKUPS/.backup.lock"
ss backup -g --cleanup > "$ROOT/repeat.txt"
grep -F 'No backups to clean up' "$ROOT/repeat.txt"
echo 'staging-only cleanup and preview PASS'
```

Expected:
- exit_code: 0
- staging-only cleanup and preview PASS

## Step 2: Preserve a completed snapshot beside orphaned staging

Create the completed snapshot through the CLI rather than assuming a fixed
timestamp. Disabling count and size limits must not disable orphan cleanup.

```bash
set -euo pipefail
ROOT=$(mktemp -d "$HOME/backup-staging-mixed.XXXXXX")
export XDG_CONFIG_HOME="$ROOT/config" XDG_DATA_HOME="$ROOT/data"
export XDG_STATE_HOME="$ROOT/state" XDG_CACHE_HOME="$ROOT/cache"
unset SKILLSHARE_CONFIG
BACKUPS="$XDG_DATA_HOME/skillshare/backups"
mkdir -p "$XDG_CONFIG_HOME/skillshare" "$ROOT/source" "$ROOT/target"
printf 'completed payload' > "$ROOT/target/local.txt"
cat > "$XDG_CONFIG_HOME/skillshare/config.yaml" <<EOF
source: $ROOT/source
mode: merge
targets:
  claude:
    skills:
      path: $ROOT/target
backup:
  max_count: 0
  max_size_mb: 0
EOF
ss backup -g claude > "$ROOT/create.txt"
SNAPSHOT=$(find "$BACKUPS" -mindepth 1 -maxdepth 1 -type d ! -name '.snapshot-*')
test -n "$SNAPSHOT"
test "$(cat "$SNAPSHOT/claude/local.txt")" = 'completed payload'
mkdir -p "$BACKUPS/.snapshot-orphan"
printf partial > "$BACKUPS/.snapshot-orphan/partial.txt"
ss backup -g --list > "$ROOT/list.txt"
! grep -F '.snapshot-orphan' "$ROOT/list.txt" || exit 1
ss backup -g --cleanup --dry-run > "$ROOT/preview.txt"
grep -F 'Would remove 1 backup directory, freeing 7 B' "$ROOT/preview.txt"
test -f "$BACKUPS/.snapshot-orphan/partial.txt"
test -f "$SNAPSHOT/claude/local.txt"
ss backup -g --cleanup > "$ROOT/cleanup.txt"
grep -F 'Removed 1 backup directory, freed 7 B' "$ROOT/cleanup.txt"
grep -F '1 backup left' "$ROOT/cleanup.txt"
test ! -e "$BACKUPS/.snapshot-orphan"
test "$(cat "$SNAPSHOT/claude/local.txt")" = 'completed payload'
echo 'completed snapshot preserved PASS'
```

Expected:
- exit_code: 0
- completed snapshot preserved PASS

## Step 3: Protect a live copy, then reclaim it after SIGKILL

Create 20,000 small target files (about 20 MB) to give the observer time to see
partial copying. Stop only after staging contains a copied file. If copying
finishes before interception, fail explicitly instead of passing with a fake
orphan. A `finally` block kills and reaps the child on every failure path.

```bash
set -euo pipefail
ROOT=$(mktemp -d "$HOME/backup-staging-killed.XXXXXX")
export ROOT
export XDG_CONFIG_HOME="$ROOT/config" XDG_DATA_HOME="$ROOT/data"
export XDG_STATE_HOME="$ROOT/state" XDG_CACHE_HOME="$ROOT/cache"
unset SKILLSHARE_CONFIG
python3 - <<'PY'
import os
from pathlib import Path
import signal
import subprocess
import time

binary = '/workspace/bin/skillshare'
root = Path(os.environ['ROOT'])
config = Path(os.environ['XDG_CONFIG_HOME']) / 'skillshare'
backups = Path(os.environ['XDG_DATA_HOME']) / 'skillshare/backups'
target = root / 'target'
config.mkdir(parents=True)
target.mkdir()
(root / 'source').mkdir()
(config / 'config.yaml').write_text(
    f'source: {root}/source\nmode: merge\ntargets:\n'
    f'  claude:\n    skills:\n      path: {target}\n'
)
for index in range(20000):
    (target / f'{index:05d}.txt').write_bytes(b'x' * 1024)

def cli(*args):
    return subprocess.run([binary, *args], text=True, capture_output=True, timeout=30)

with (root / 'child.log').open('w') as log:
    child = subprocess.Popen([binary, 'backup', '-g', 'claude'], stdout=log, stderr=log)
    try:
        deadline = time.monotonic() + 30
        staging = None
        while time.monotonic() < deadline:
            assert child.poll() is None, 'backup finished before interception'
            candidates = list(backups.glob('.snapshot-*'))
            if candidates:
                sample = candidates[0] / '00000.txt'
                if sample.is_file() and sample.read_bytes() == b'x' * 1024:
                    staging = candidates[0]
                    os.kill(child.pid, signal.SIGSTOP)
                    break
            time.sleep(0.001)
        assert staging is not None, 'timed out waiting for partial staging'
        # Wait for the stop before inspecting files or exercising cleanup.
        pid, status = os.waitpid(child.pid, os.WUNTRACED)
        assert pid == child.pid and os.WIFSTOPPED(status), 'child did not stop'
        partial_files = list(staging.iterdir())
        assert partial_files, 'staging contains no copied files'
        assert sample.read_bytes() == b'x' * 1024
        assert len(partial_files) < 20000, 'intercepted after copying finished'
        inode = (backups / '.backup.lock').stat().st_ino
        for args in [('backup', '-g', '--cleanup'),
                     ('backup', '-g', '--cleanup', '--dry-run')]:
            result = cli(*args)
            assert result.returncode != 0, result.stdout + result.stderr
            assert 'backup directory is busy' in result.stdout + result.stderr
            assert sample.read_bytes() == b'x' * 1024
            assert len(list(staging.iterdir())) == len(partial_files)
        print('live copy and busy reporting PASS')

        child.kill()
        assert child.wait(timeout=10) == -signal.SIGKILL
        result = cli('backup', '-g', '--list')
        assert result.returncode == 0 and 'No backups found' in result.stdout
        result = cli('backup', '-g', '--cleanup', '--dry-run')
        assert result.returncode == 0, result.stdout + result.stderr
        assert 'Would remove 1 backup directory' in result.stdout
        assert sample.exists()
        result = cli('backup', '-g', '--cleanup')
        assert result.returncode == 0, result.stdout + result.stderr
        assert 'Removed 1 backup directory' in result.stdout
        assert '0 backups left' in result.stdout
        assert not staging.exists()
        assert not list(backups.glob('.snapshot-*'))
        assert (backups / '.backup.lock').stat().st_ino == inode
        print('SIGKILL orphan reclamation PASS')
    finally:
        if child.poll() is None:
            child.kill()
        child.wait(timeout=10)
PY
```

Expected:
- exit_code: 0
- live copy and busy reporting PASS
- SIGKILL orphan reclamation PASS

## Step 4: Automatic cleanup after pre-sync backup

Use plain-text sync: the JSON path does not invoke `backupTargetsBeforeSync`.
The target's local file proves that a new completed snapshot was created, while
the separate source skill proves that synchronization actually ran.

```bash
set -euo pipefail
ROOT=$(mktemp -d "$HOME/backup-staging-auto.XXXXXX")
export XDG_CONFIG_HOME="$ROOT/config" XDG_DATA_HOME="$ROOT/data"
export XDG_STATE_HOME="$ROOT/state" XDG_CACHE_HOME="$ROOT/cache"
unset SKILLSHARE_CONFIG
BACKUPS="$XDG_DATA_HOME/skillshare/backups"
mkdir -p "$XDG_CONFIG_HOME/skillshare" "$ROOT/source/probe" "$ROOT/target"
printf 'local before sync' > "$ROOT/target/local.txt"
cat > "$ROOT/source/probe/SKILL.md" <<'EOF'
---
name: probe
description: Local backup cleanup E2E fixture
---
# Probe
EOF
cat > "$XDG_CONFIG_HOME/skillshare/config.yaml" <<EOF
source: $ROOT/source
mode: merge
targets:
  claude:
    skills:
      path: $ROOT/target
EOF
mkdir -p "$BACKUPS/.snapshot-orphan"
printf partial > "$BACKUPS/.snapshot-orphan/partial.txt"
ss sync -g > "$ROOT/sync.txt"
test ! -e "$BACKUPS/.snapshot-orphan"
SNAPSHOT=$(find "$BACKUPS" -mindepth 1 -maxdepth 1 -type d ! -name '.snapshot-*')
test -n "$SNAPSHOT"
test "$(cat "$SNAPSHOT/claude/local.txt")" = 'local before sync'
test -L "$ROOT/target/probe"
cmp "$ROOT/source/probe/SKILL.md" "$ROOT/target/probe/SKILL.md"
echo 'automatic pre-sync backup cleanup PASS'
```

Expected:
- exit_code: 0
- automatic pre-sync backup cleanup PASS

## Pass Criteria

- All four steps pass without skipped steps.
- Both fresh staging-only and mixed roots reclaim exactly the orphaned directory.
- Dry-run leaves payloads intact and does not create a missing lock file.
- Cleanup refuses to touch a stopped but live CLI writer; SIGKILL makes the same partial copy reclaimable.
- The persistent lock inode is not replaced during preview or cleanup.
- Automatic pre-sync cleanup removes orphaned staging while keeping a real restore point and completing sync.
