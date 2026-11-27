# CLI E2E Runbook: Installs from pixi

Verify that a binary pixi installed is recognized, `skillshare upgrade` prints pixi's command instead of replacing the binary, and a direct install is not mistaken for one. Raised on PR #1032.

## Scope

- `pixi global install` of the `skillshare` conda package from the `github-releases` channel, `pixi add` of that package in a pixi project, and `pixi add --pypi skillshare-cli` in a pixi project each get their own upgrade command.
- The branch's binary replaces the one pixi linked into each environment, so the test runs this branch's code inside a real pixi environment.
- `skillshare upgrade --cli` leaves that binary byte-for-byte unchanged.
- A copy of the same binary outside a pixi environment stays a direct install.
- Not covered: Windows, conda or mamba environments that pixi did not create, and `pixi exec`.

## Environment

Run in the devcontainer with network access: the steps fetch pixi, the conda package and the wheel, and `upgrade` asks GitHub and PyPI for the latest version. The local build reports version `dev`, so every published release is newer. Run each binary from `/tmp`, not `/workspace`, so it does not pick up the repository's project config. Each step runs in its own shell, so steps 2 and 3 set `HOME`, `PIXI_HOME` and `PATH` again.

## Steps

### 1. Build the branch and install pixi

```bash
cd /workspace && go build -o /tmp/ss-branch ./cmd/skillshare
export HOME=/tmp/pxh PIXI_HOME=/tmp/pxh/.pixi PATH=/tmp/pxh/.pixi/bin:$PATH
rm -rf "$HOME" && mkdir -p "$HOME" && cd "$HOME"
curl -fsSL https://pixi.sh/install.sh | PIXI_NO_PATH_UPDATE=1 bash >/dev/null 2>&1
pixi --version
```

**Expected:**
- exit_code: 0
- regex: ^pixi \d+\.\d+

### 2. Install skillshare three ways

```bash
export HOME=/tmp/pxh PIXI_HOME=/tmp/pxh/.pixi PATH=/tmp/pxh/.pixi/bin:$PATH
cd "$HOME"
pixi global install -q -c https://prefix.dev/github-releases skillshare >/dev/null 2>&1
mkdir pc && cd pc && pixi init -q . >/dev/null 2>&1
sed -i 's|^channels = \[\(.*\)\]|channels = [\1, "https://prefix.dev/github-releases"]|' pixi.toml
pixi add -q skillshare >/dev/null 2>&1
mkdir ../pp && cd ../pp && pixi init -q . >/dev/null 2>&1
pixi add -q python >/dev/null 2>&1 && pixi add -q --pypi skillshare-cli >/dev/null 2>&1
ls "$PIXI_HOME/envs/skillshare/bin/skillshare" "$HOME/pc/.pixi/envs/default/bin/skillshare" "$HOME/pp/.pixi/envs/default/bin/skillshare" | wc -l
```

**Expected:**
- exit_code: 0
- regex: ^3$

### 3. Each pixi install prints its own command and keeps the binary

```bash
export HOME=/tmp/pxh PIXI_HOME=/tmp/pxh/.pixi PATH=/tmp/pxh/.pixi/bin:$PATH
unchanged=0
for b in "$PIXI_HOME/envs/skillshare/bin/skillshare" "$HOME/pc/.pixi/envs/default/bin/skillshare" "$HOME/pp/.pixi/envs/default/bin/skillshare"; do
  cp --remove-destination /tmp/ss-branch "$b"   # break pixi's hard link to its cache
  before=$(sha256sum "$b" | cut -d' ' -f1)
  (cd /tmp && "$b" upgrade --cli 2>&1 | grep CLI)
  [ "$before" = "$(sha256sum "$b" | cut -d' ' -f1)" ] && unchanged=$((unchanged + 1))
done
echo "unchanged: $unchanged"
```

**Expected:**
- exit_code: 0
- installed with pixi, run: pixi global update skillshare
- installed with pixi, run: pixi upgrade skillshare
- installed from PyPI, run: pixi upgrade skillshare-cli
- regex: ^unchanged: 3$

### 4. The same binary outside pixi is a direct install

```bash
cp /tmp/ss-branch /tmp/ss-direct
cd /tmp && /tmp/ss-direct upgrade --cli --dry-run 2>&1 | grep CLI
```

**Expected:**
- exit_code: 0
- would download
- Not installed with pixi

## Pass Criteria

- Each pixi install prints its pixi command and its binary is unchanged.
- The direct copy would still download.
