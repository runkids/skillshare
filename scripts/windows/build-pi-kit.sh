#!/usr/bin/env bash
# Build the Pi regression kit from a pinned commit, inside the chosen devcontainer.
set -euo pipefail
root=$(cd "$(dirname "$0")/../.." && pwd)
sha=$(git -C "$root" rev-parse --verify "${1:?git-ref}^{commit}")
container=${2:?devcontainer-name}
arch=${3:-arm64}
case "$arch" in arm64|amd64) ;; *) echo 'Use arm64 or amd64.' >&2; exit 1;; esac
out="$root/.playwright-mcp/windows-pi-${sha:0:8}-$arch"
[ ! -e "$out/kit.zip" ] || { echo "Kit already exists: $out" >&2; exit 1; }
mkdir -p "$out"
snapshot=$(docker exec "$container" mktemp -d /tmp/pi-extensions-kit.XXXXXXXX)
git -C "$root" archive "$sha" | docker exec -i -e SNAPSHOT="$snapshot" -e PIN="$sha" -e ARCH="$arch" "$container" bash -c '
set -euo pipefail
mkdir "$SNAPSHOT/src" "$SNAPSHOT/out"
tar -x -C "$SNAPSHOT/src"
cd "$SNAPSHOT/src"
export PATH=/usr/local/go/bin:$PATH
GOOS=windows GOARCH="$ARCH" go build -ldflags "-X main.version=head-${PIN:0:8}" -o "$SNAPSHOT/out/ss.exe" ./cmd/skillshare
GOOS=windows GOARCH="$ARCH" go test -c ./internal/plugin -o "$SNAPSHOT/out/plugin.test.exe"
python3 - <<"PY"
from pathlib import Path
import os, zipfile
base = Path(os.environ["SNAPSHOT"])
with zipfile.ZipFile(base / "out/kit.zip", "w", zipfile.ZIP_DEFLATED) as kit:
    for name in ("ss.exe", "plugin.test.exe"):
        kit.write(base / "out" / name, name)
    for directory in ("internal/plugin", "scripts/pi"):
        for file in (base / "src" / directory).rglob("*"):
            if file.is_file():
                kit.write(file, file.relative_to(base))
    kit.write(base / "src/scripts/windows/e2e-pi-extensions.ps1", "run.ps1")
    kit.writestr("commit.txt", os.environ["PIN"] + "\n")
PY
'
docker cp "$container:$snapshot/out/kit.zip" "$out/kit.zip"
printf 'commit=%s\narch=%s\nkit=%s/kit.zip\n' "$sha" "$arch" "$out"
