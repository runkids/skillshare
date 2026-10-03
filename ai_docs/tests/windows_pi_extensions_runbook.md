# Windows Pi lock and private registration verification

## Scope

Follow-up to #350 / #358: reclaim only unchanged empty expired native locks, retain
racing owners, snapshot acquired Windows file IDs before path reuse, and protect
preserved filtered registrations with Windows ACLs. Test the existing global,
project and account import lifecycles and Pi 0.99.2/1.0.0 native interoperability.
No extension factories, native trust changes, user settings, or global Pi installs.

## Environment

Use a running Linux devcontainer for all builds and a logged-in UTM Windows desktop
user for execution. SYSTEM is setup/transport only. Run both Interactive full and
`runas /trustlevel:0x20000` basic tokens. Record the pinned commit, native architecture,
Node version and Developer Mode state. All guest files belong under
`C:\Users\Public\sstest\`; preserve existing kits and reports.

## Steps

### 1. Probe and build the pinned kit

Run from the repository on the host. Choose an immutable commit and the guest's
native architecture; `ss-pi-ext` below is the task-owned devcontainer.

```bash
bash scripts/windows/utm.sh probe
PIN=$(git rev-parse HEAD)
bash scripts/windows/build-pi-kit.sh "$PIN" ss-pi-ext arm64
```

**Expected**: started VM, nonempty desktop user, architecture ARM64. The builder
prints the full commit, architecture and repository-local ignored `kit.zip` path.
It uses `git archive`, never working-tree source. The kit includes the CLI, plugin
test binary, pinned testdata/native probes, runner and `commit.txt`. An existing kit
is refused rather than overwritten.

### 2. Create and unpack a fresh guest kit (SYSTEM setup only)

Keep these shell variables for later steps. Change the suffix for a repeated run;
never replace an existing kit. This setup script is stored inside the repository.

```bash
KIT=".playwright-mcp/windows-pi-${PIN:0:8}-arm64"
GUEST="C:\\Users\\Public\\sstest\\pi-extensions-${PIN:0:8}"
printf '%s\n' "\$root='$GUEST'" \
  'if (Test-Path $root) { throw "Use a fresh kit." }' \
  'New-Item -ItemType Directory $root | Out-Null' > "$KIT/setup.ps1"
bash scripts/windows/utm.sh ps "$KIT/setup.ps1" 30
bash scripts/windows/utm.sh push "$KIT/kit.zip" "$GUEST\\kit.zip"
printf '%s\n' "\$root='$GUEST'" \
  'Expand-Archive (Join-Path $root "kit.zip") $root' > "$KIT/unpack.ps1"
bash scripts/windows/utm.sh ps "$KIT/unpack.ps1" 40
```

**Expected**: fresh directory and successful archive extraction. Setup does not run
the product. Do not treat `utm.sh ps` transport completion as product evidence.

### 3. Execute full, then basic token acceptance

The first run installs only the two exact native versions into this kit, with
scripts disabled and fresh npm configuration/cache. Wait for full completion before
basic so it can reuse those isolated installations. `-NativeRoot` can instead point
to a previously verified sstest kit; omit `-InstallNative` in that case. Never use a
user profile or global installation as the native prefix.

```bash
bash scripts/windows/utm.sh task sstest-pi-extensions-full "$GUEST\\run.ps1" -- \
  -Root "$GUEST" -Token full -InstallNative
bash scripts/windows/utm.sh pull "$GUEST\\result-full.txt"
```

**Expected**: report ending in `DONE`, `REGRESSIONS_EXIT=0`, and both
`LAUNCHER_<version>_EXIT=0` / `NATIVE_<version>_EXIT=0`. No `ERROR` or `MISSING` markers.
`pull` can exit zero when a file is missing: inspect content, not its exit status.
The initial regression pass intentionally has no `PI_ROOT`; its native-lock test
skips there and must pass separately in both native-version passes.

```bash
bash scripts/windows/utm.sh task sstest-pi-extensions-basic "$GUEST\\run.ps1" --basic -- \
  -Root "$GUEST" -Token basic
bash scripts/windows/utm.sh pull "$GUEST\\result-basic.txt"
```

**Expected**: the same zero markers and `DONE` under the basic token. Native lock
interoperability runs longer than ten seconds. The runner reports failures in its
exit status as well as markers, and exports child logs as `.log.utf8` without
changing the original streams. Inspect `regressions.log.utf8` and native logs; do not
relabel environment skips as executed tests. Symlink creation may be unavailable
without the relevant privilege/Developer Mode; report that case explicitly.

### 4. Check safety results and retain evidence

```bash
bash scripts/windows/utm.sh pull "$GUEST\\full\\regressions.log.utf8"
bash scripts/windows/utm.sh pull "$GUEST\\basic\\regressions.log.utf8"
```

**Expected**: stale empty directory replaced with a distinct file ID; renewed,
replaced, nonempty, future, file and supported symlink cases retained. A replacement
with matching mtime before first verification is refused and survives release.
Registration directories/files have real private DACLs; broadly accessible existing
files cannot be read/reused, and unsafe existing directories keep their ACL/settings
bytes with no new raw record. Global/project/account lifecycle cases pass.

## Pass Criteria

- Full/basic reports and child logs retained, with exact commit/architecture/token.
- Regression and both versions' launcher/native checks exit zero; skips disclosed.
- No shared/user settings, trust, Developer Mode, global installs or existing ACLs changed.
- No broad cleanup: keep earlier failed kits and existing preview/container resources.
