#!/bin/bash
# Offline installer regressions. Run as a non-root user inside the devcontainer.
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
INSTALL_SCRIPT="${INSTALL_SCRIPT:-$PROJECT_ROOT/install.sh}"
TEST_ROOT=$(mktemp -d)
trap 'rm -rf "$TEST_ROOT"' EXIT

mkdir -p "$TEST_ROOT/mock-bin" "$TEST_ROOT/archive"
cat > "$TEST_ROOT/archive/skillshare" <<'SH'
#!/bin/sh
echo installed >> "$INSTALL_TEST_EXEC_LOG"
echo 'skillshare mock v0.0.0'
SH
tar czf "$TEST_ROOT/release.tar.gz" -C "$TEST_ROOT/archive" skillshare

cat > "$TEST_ROOT/mock-bin/curl" <<'SH'
#!/bin/sh
case "$*" in
  *-sI*) printf 'location: https://github.com/runkids/skillshare/releases/tag/v0.0.0\r\n' ;;
  *) cat "$INSTALL_TEST_ARCHIVE" ;;
esac
SH
cat > "$TEST_ROOT/mock-bin/uname" <<'SH'
#!/bin/sh
case "$1" in
  -s) echo "$INSTALL_TEST_OS" ;;
  -m) echo x86_64 ;;
esac
SH
cat > "$TEST_ROOT/mock-bin/sudo" <<'SH'
#!/bin/sh
echo "$*" >> "$INSTALL_TEST_SUDO_LOG"
# This fixture owns the protected directory; make the simulated sudo move possible.
if [ "$1" = mv ]; then
  case "$3" in
    "$INSTALL_TEST_ROOT"/*) chmod u+w "$3" ;;
    *) echo 'Refusing sudo outside the test fixture' >&2; exit 1 ;;
  esac
fi
"$@"
SH
chmod +x "$TEST_ROOT/mock-bin/"*

run_install() {
  local case_home="$1" case_path="$2" case_os="$3"
  shift 3
  mkdir -p "$case_home"
  env -u INSTALL_DIR HOME="$case_home" PATH="$case_path" \
    INSTALL_TEST_OS="$case_os" INSTALL_TEST_ARCHIVE="$TEST_ROOT/release.tar.gz" \
    INSTALL_TEST_ROOT="$TEST_ROOT" \
    INSTALL_TEST_EXEC_LOG="$case_home/executed" INSTALL_TEST_SUDO_LOG="$case_home/sudo" \
    "$@" sh "$INSTALL_SCRIPT" > "$case_home/output" 2>&1 || {
      cat "$case_home/output"
      return 1
    }
}

assert_contains() {
  if ! grep -Fq -- "$2" "$1"; then
    cat "$1"
    echo "FAIL: expected $2" >&2
    exit 1
  fi
}

TEST_PATH="$TEST_ROOT/mock-bin:/usr/bin:/bin"
for test_os in Linux Darwin; do
  case_home="$TEST_ROOT/$test_os"
  run_install "$case_home" "$TEST_PATH" "$test_os"
  test -x "$case_home/.local/bin/skillshare"
  test ! -e "$case_home/sudo"
  assert_contains "$case_home/output" 'skillshare mock v0.0.0'
  assert_contains "$case_home/output" "export PATH=\"$case_home/.local/bin:\$PATH\""
  echo "PASS: $test_os default creates a user-local directory without sudo"
done

case_home="$TEST_ROOT/in-path"
run_install "$case_home" "$case_home/.local/bin:$TEST_PATH" Linux
test ! -e "$case_home/sudo"
if grep -Fq 'export PATH=' "$case_home/output"; then
  echo 'FAIL: unexpected PATH warning' >&2
  exit 1
fi
echo 'PASS: no PATH warning when the installed binary is selected'

mkdir -p "$TEST_ROOT/legacy-bin"
cat > "$TEST_ROOT/legacy-bin/skillshare" <<'SH'
#!/bin/sh
echo legacy >> "$INSTALL_TEST_EXEC_LOG"
SH
chmod +x "$TEST_ROOT/legacy-bin/skillshare"
case_home="$TEST_ROOT/shadowed"
run_install "$case_home" "$TEST_ROOT/legacy-bin:$case_home/.local/bin:$TEST_PATH" Linux
assert_contains "$case_home/output" "$TEST_ROOT/legacy-bin/skillshare takes precedence"
test "$(cat "$case_home/executed")" = installed
echo 'PASS: verifies the new binary and reports an older PATH entry'

case_home="$TEST_ROOT/custom"
custom_dir="$TEST_ROOT/custom bin/nested"
run_install "$case_home" "$TEST_PATH" Linux INSTALL_DIR="$custom_dir"
test -x "$custom_dir/skillshare"
test ! -e "$case_home/.local/bin/skillshare"
test ! -e "$case_home/sudo"
assert_contains "$case_home/output" "export PATH=\"$custom_dir:\$PATH\""
run_install "$case_home" "$TEST_PATH" Linux INSTALL_DIR="$custom_dir"
test "$(wc -l < "$case_home/executed")" -eq 2
echo 'PASS: custom directories, spaces, and repeated installs'

if [ "$(id -u)" -eq 0 ]; then
  echo 'FAIL: run as a non-root user to verify the protected-directory sudo path' >&2
  exit 1
fi
case_home="$TEST_ROOT/protected"
protected_dir="$TEST_ROOT/protected-bin"
mkdir -p "$protected_dir"
chmod u-w "$protected_dir"
run_install "$case_home" "$TEST_PATH" Linux INSTALL_DIR="$protected_dir"
test -x "$protected_dir/skillshare"
assert_contains "$case_home/sudo" "mv "
assert_contains "$case_home/output" "Need sudo to install to $protected_dir"
echo 'PASS: explicit protected directory retains sudo installation'

cat > "$TEST_ROOT/archive/skillshare" <<'SH'
#!/bin/sh
echo 'version verification failed' >&2
exit 1
SH
tar czf "$TEST_ROOT/release.tar.gz" -C "$TEST_ROOT/archive" skillshare
case_home="$TEST_ROOT/invalid-binary"
if run_install "$case_home" "$TEST_PATH" Linux > /dev/null; then
  echo 'FAIL: installer ignored a failing installed binary' >&2
  exit 1
fi
assert_contains "$case_home/output" 'version verification failed'
echo 'PASS: a failed version check fails installation even when PATH is missing'

echo 'All offline installer regressions passed.'
