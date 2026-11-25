# CLI E2E Runbook: Installs from PyPI Wheels

Verify issue #236: a binary installed from a `skillshare-cli` wheel by uv, pipx or pip is recognized, `skillshare upgrade` prints the installer's command instead of replacing the binary, a direct install is not mistaken for one, and each installer removes the binary cleanly.

## Scope

- The wheel is built with `scripts/release/release.py` from archives that hold a local build, so the test runs this branch's code.
- uv (`uv tool install`), pipx, pip in a virtual environment, `uv pip` in a uv venv without pip, and `uvx` each get their own upgrade command.
- `skillshare upgrade --cli` leaves the installed binary byte-for-byte unchanged.
- A copy of the same binary outside a Python prefix stays a direct install.
- Not covered: Windows, where uv and pipx copy the executable onto PATH (see the upgrade page), and musl/Alpine. Install the wheel there by hand when changing the packaging.

## Environment

Run in the devcontainer with a fresh ssenv and network access: steps fetch uv and pipx, and `upgrade` asks PyPI for the latest `skillshare-cli`. The local build reports version `0.0.1`, so every published release is newer. Until the first release is on PyPI, `upgrade` cannot find a version and steps 3 to 6 print `couldn't check for a newer version` instead.

## Steps

### 1. Build a wheel from a local binary

```bash
rm -rf /tmp/pypi && mkdir -p /tmp/pypi/dist /tmp/pypi/stage
cd /workspace && go build -ldflags "-X main.version=0.0.1" -o /tmp/pypi/stage/skillshare ./cmd/skillshare
cd /tmp/pypi/stage
for target in darwin_amd64 darwin_arm64 linux_amd64 linux_arm64; do
  tar -czf "/tmp/pypi/dist/skillshare_0.0.1_${target}.tar.gz" skillshare
done
cp skillshare skillshare.exe
for target in windows_amd64 windows_arm64; do
  python3 -c 'import sys, zipfile; zipfile.ZipFile(sys.argv[1], "w").write("skillshare.exe")' "/tmp/pypi/dist/skillshare_0.0.1_${target}.zip"
done
python3 -c 'import sys; sys.path.insert(0, "/workspace/scripts/release"); import release; from pathlib import Path; release.build_wheels(Path("/tmp/pypi/dist"), "0.0.1"); release.verify_wheels(Path("/tmp/pypi/dist"), "0.0.1")'
ls /tmp/pypi/dist/*.whl | wc -l
```

**Expected:**
- exit_code: 0
- regex: ^6$

### 2. Install uv and pipx outside the system Python

```bash
curl -LsSf https://astral.sh/uv/install.sh | env UV_INSTALL_DIR=/tmp/pypi/uv-bin UV_NO_MODIFY_PATH=1 sh >/dev/null
/tmp/pypi/uv-bin/uv --version
```

**Expected:**
- exit_code: 0
- regex: ^uv \d+\.\d+

### 3. uv tool install is detected as uv

```bash
export UV_TOOL_DIR=/tmp/pypi/uv-tools UV_TOOL_BIN_DIR=/tmp/pypi/uv-tool-bin
/tmp/pypi/uv-bin/uv tool install --no-index --find-links /tmp/pypi/dist skillshare-cli >/dev/null
before=$(sha256sum "$UV_TOOL_BIN_DIR/skillshare" | cut -d' ' -f1)
"$UV_TOOL_BIN_DIR/skillshare" upgrade --cli
after=$(sha256sum "$UV_TOOL_BIN_DIR/skillshare" | cut -d' ' -f1)
test "$before" = "$after" && echo binary-unchanged
```

**Expected:**
- exit_code: 0
- installed from PyPI, run: uv tool upgrade skillshare-cli
- binary-unchanged

### 4. pipx install is detected as pipx

```bash
export PIPX_HOME=/tmp/pypi/pipx PIPX_BIN_DIR=/tmp/pypi/pipx-bin
/tmp/pypi/uv-bin/uvx pipx install --pip-args "--no-index --find-links /tmp/pypi/dist" skillshare-cli >/dev/null
before=$(sha256sum "$PIPX_BIN_DIR/skillshare" | cut -d' ' -f1)
"$PIPX_BIN_DIR/skillshare" upgrade --cli
after=$(sha256sum "$PIPX_BIN_DIR/skillshare" | cut -d' ' -f1)
test "$before" = "$after" && echo binary-unchanged
```

**Expected:**
- exit_code: 0
- installed from PyPI, run: pipx upgrade skillshare-cli
- binary-unchanged

### 5. pip in a virtual environment is detected as pip

```bash
/tmp/pypi/uv-bin/uv venv --seed /tmp/pypi/venv >/dev/null 2>&1
/tmp/pypi/venv/bin/pip install --no-index --find-links /tmp/pypi/dist skillshare-cli >/dev/null
/tmp/pypi/venv/bin/skillshare upgrade --cli
```

**Expected:**
- exit_code: 0
- installed from PyPI, run: pip install --upgrade skillshare-cli

### 6. uv pip in a uv venv and uvx each get their own command

```bash
export UV_CACHE_DIR=/tmp/pypi/uv-cache
/tmp/pypi/uv-bin/uv venv /tmp/pypi/uv-venv >/dev/null 2>&1
VIRTUAL_ENV=/tmp/pypi/uv-venv /tmp/pypi/uv-bin/uv pip install --no-index --find-links /tmp/pypi/dist skillshare-cli >/dev/null 2>&1
/tmp/pypi/uv-venv/bin/skillshare upgrade --cli
/tmp/pypi/uv-bin/uvx --no-index --find-links /tmp/pypi/dist --from skillshare-cli skillshare upgrade --cli
```

**Expected:**
- exit_code: 0
- installed from PyPI, run: uv pip install --upgrade skillshare-cli
- installed from PyPI, run: uvx --from skillshare-cli@latest skillshare

### 7. The same binary outside a Python prefix stays a direct install

```bash
mkdir -p /tmp/pypi/direct/bin && cp /tmp/pypi/stage/skillshare /tmp/pypi/direct/bin/skillshare
/tmp/pypi/direct/bin/skillshare upgrade --cli --dry-run
```

**Expected:**
- exit_code: 0
- would download
- Not installed from PyPI

### 8. Each installer removes the binary

```bash
export UV_TOOL_DIR=/tmp/pypi/uv-tools UV_TOOL_BIN_DIR=/tmp/pypi/uv-tool-bin PIPX_HOME=/tmp/pypi/pipx PIPX_BIN_DIR=/tmp/pypi/pipx-bin
/tmp/pypi/uv-bin/uv tool uninstall skillshare-cli >/dev/null
/tmp/pypi/uv-bin/uvx pipx uninstall skillshare-cli >/dev/null
/tmp/pypi/venv/bin/pip uninstall -y skillshare-cli >/dev/null
ls "$UV_TOOL_BIN_DIR/skillshare" "$PIPX_BIN_DIR/skillshare" /tmp/pypi/venv/bin/skillshare 2>/dev/null | wc -l
```

**Expected:**
- regex: ^0$

## Pass Criteria

- Steps 3 to 6 each print their own installer's command, and steps 3 and 4 leave the binary unchanged.
- Step 7 offers a download, so a direct install is not mistaken for a PyPI one.
- Step 8 leaves no `skillshare` binary behind.
