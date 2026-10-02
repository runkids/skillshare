#!/usr/bin/env bash
# Drive a local UTM Windows guest for skillshare verification. Host-side only: it never runs the
# product on macOS. Builds happen in the devcontainer; the guest runs them. See the
# skillshare-windows-utm skill and wiki/testing.md "Windows Verification".
#
#   utm.sh probe                       VM state, guest agent, logged-in user, arch, Developer Mode
#   utm.sh build <ref> [arm64|amd64]   pinned git archive -> $OUT/ss.exe, ss-ui-dist.zip, ss-version.txt
#   utm.sh push-manual                 push the build + manual-setup.ps1 to C:\Users\Public (hands-on test)
#   utm.sh push <local> <guest-path>   push one file (the guest folder must exist)
#   utm.sh pull <guest-path>           print a guest file (exits 0 even when it is missing)
#   utm.sh ps <file.ps1> [timeout-s]   run a script as SYSTEM and print its output
#   utm.sh task <name> <guest.ps1> [--basic] [-- script args]
#                                      run a guest script as the desktop user (scheduled task)
#   utm.sh clean                       unregister sstest-* tasks, remove C:\Users\Public\sstest
#
# Env: VM (default Windows), GUEST_USER (default Willie), OUT (default ${TMPDIR:-/tmp}/skillshare-utm).
set -euo pipefail

VM=${VM:-Windows}
GUEST_USER=${GUEST_USER:-Willie}
OUT=${OUT:-${TMPDIR:-/tmp}/skillshare-utm}
# The Homebrew symlink fails with "Application not found"; call the app bundle directly.
U=${UTMCTL:-$HOME/Applications/UTM.app/Contents/MacOS/utmctl}
[ -x "$U" ] || U=/Applications/UTM.app/Contents/MacOS/utmctl
ROOT=$(cd "$(dirname "$0")/../.." && pwd)

die() { echo "utm.sh: $*" >&2; exit 1; }

container() {
  local c
  c=$(docker compose -f "$ROOT/.devcontainer/docker-compose.yml" ps -q skillshare-devcontainer 2>/dev/null)
  [ -n "$c" ] || die "devcontainer is not running; run make devc-up"
  echo "$c"
}

# Run a PowerShell script on the guest as SYSTEM. utmctl exec is asynchronous and returns no
# output, so the script writes to a file ending in a marker and the host polls it.
run_ps() {
  local file=$1 timeout=${2:-120} id=$RANDOM$RANDOM tmp
  tmp=$(mktemp)
  {
    echo "\$ErrorActionPreference='Continue'"
    echo "& {"; cat "$file"; echo "} *>&1 | Out-File -Encoding utf8 C:\\Windows\\Temp\\utm-$id.txt"
    echo "'UTMDONE' | Out-File -Encoding utf8 -Append C:\\Windows\\Temp\\utm-$id.txt"
  } > "$tmp"
  "$U" file push "$VM" "C:\\Windows\\Temp\\utm-$id.ps1" < "$tmp"
  rm -f "$tmp"
  "$U" exec "$VM" --cmd powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\\Windows\\Temp\\utm-$id.ps1" >/dev/null 2>&1 || true
  local o
  for _ in $(seq 1 "$timeout"); do
    o=$("$U" file pull "$VM" "C:\\Windows\\Temp\\utm-$id.txt" 2>&1 || true)
    if grep -q UTMDONE <<<"$o"; then sed '$d' <<<"$o" | tr -d '\r'; return 0; fi
    sleep 1
  done
  die "timed out waiting for $file"
}

# Inline PowerShell through run_ps; a heredoc keeps backslashes intact (zsh echo eats them).
ps_inline() { local f; f=$(mktemp); cat > "$f"; run_ps "$f" "${1:-120}"; rm -f "$f"; }

cmd=${1:-}; shift || true
case "$cmd" in
  probe)
    "$U" status "$VM" || die "VM $VM not found (grant Automation permission if utmctl reports OSStatus -1712)"
    # -2700 or "guest agent" errors mean the guest is still booting or the agent is not up yet.
    echo x | "$U" file push "$VM" 'C:\Windows\Temp\utm-probe.txt' || die "guest agent not reachable yet; wait for boot"
    ps_inline 60 <<'EOF'
# The native architecture: under x64 emulation on ARM64, env and .NET both report x64.
"arch=" + (Get-ItemProperty 'HKLM:\SYSTEM\CurrentControlSet\Control\Session Manager\Environment').PROCESSOR_ARCHITECTURE
"desktopUser=" + (((Get-CimInstance Win32_Process -Filter "Name='explorer.exe'") | % { (Invoke-CimMethod -InputObject $_ -MethodName GetOwner).User }) -join ',')
"devMode=" + (Get-ItemProperty HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\AppModelUnlock -ErrorAction SilentlyContinue).AllowDevelopmentWithoutDevLicense
"edition=" + (Get-CimInstance Win32_OperatingSystem).Caption
EOF
    ;;
  build)
    ref=${1:?ref}; arch=${2:-arm64}
    sha=$(git -C "$ROOT" rev-parse --short "$ref")
    c=$(container); mkdir -p "$OUT"
    # A pinned archive, so edits in the working tree never leak into the build.
    docker exec "$c" bash -lc "set -e; credential-helper off >/dev/null 2>&1 || true
      rm -rf /tmp/utm-build && mkdir -p /tmp/utm-build/src /tmp/utm-build/out
      git -C /workspace archive $sha | tar -x -C /tmp/utm-build/src
      cd /tmp/utm-build/src && GOOS=windows GOARCH=$arch go build -ldflags '-X main.version=head-$sha' -o /tmp/utm-build/out/ss.exe ./cmd/skillshare
      # pnpm rejects a symlinked node_modules; install from the shared store instead.
      cd ui && pnpm install --frozen-lockfile --offline --store-dir /workspace/.pnpm-store >/dev/null && pnpm run build >/dev/null
      cd dist && python3 -c 'import shutil; shutil.make_archive(\"/tmp/utm-build/out/ss-ui-dist\", \"zip\", \".\")'
      printf head-$sha > /tmp/utm-build/out/ss-version.txt"
    for f in ss.exe ss-ui-dist.zip ss-version.txt; do docker cp "$c:/tmp/utm-build/out/$f" "$OUT/$f"; done
    echo "built head-$sha ($arch) in $OUT"
    ;;
  push-manual)
    for f in ss.exe ss-ui-dist.zip ss-version.txt; do
      [ -f "$OUT/$f" ] || die "missing $OUT/$f; run utm.sh build first"
      "$U" file push "$VM" "C:\\Users\\Public\\$f" < "$OUT/$f"
    done
    "$U" file push "$VM" 'C:\Users\Public\ss-setup.ps1' < "$ROOT/scripts/windows/manual-setup.ps1"
    echo 'In a normal (non-admin) PowerShell on the guest:'
    echo '  powershell -ExecutionPolicy Bypass -File C:\Users\Public\ss-setup.ps1'
    ;;
  push) "$U" file push "$VM" "${2:?guest-path}" < "${1:?local}" ;;
  pull) "$U" file pull "$VM" "${1:?guest-path}" | tr -d '\r' ;;
  ps) run_ps "${1:?file.ps1}" "${2:-120}" ;;
  task)
    name=${1:?name}; script=${2:?guest.ps1}; shift 2
    basic=""; if [ "${1:-}" = --basic ]; then basic=" -Basic"; shift; fi
    [ "${1:-}" = -- ] && shift
    "$U" file push "$VM" 'C:\Users\Public\launch-task.ps1' < "$ROOT/scripts/windows/launch-task.ps1"
    ps_inline 60 <<EOF
& C:\\Users\\Public\\launch-task.ps1 -Name '$name' -User '$GUEST_USER' -Script '$script' -ScriptArgs '$*'$basic
EOF
    ;;
  clean)
    ps_inline 120 <<'EOF'
Get-ScheduledTask -TaskName sstest-* -ErrorAction SilentlyContinue | Unregister-ScheduledTask -Confirm:$false
# rmdir does not follow junctions, so test junctions are removed without touching their targets.
if (Test-Path C:\Users\Public\sstest) { cmd /c 'rmdir /s /q C:\Users\Public\sstest' }
Remove-Item C:\Users\Public\launch-task.ps1, C:\Windows\Temp\utm-* -Force -ErrorAction SilentlyContinue
"cleaned"
EOF
    ;;
  *) sed -n '2,17p' "$0"; exit 2 ;;
esac
