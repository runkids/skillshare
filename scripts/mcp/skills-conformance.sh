#!/usr/bin/env bash
# Run the pinned official Skills scenarios against synthetic data in a devcontainer.
# Usage: bash scripts/mcp/skills-conformance.sh <absolute-report-directory>
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
REPORT=${1:?provide an absolute report directory outside the fixture HOME}
[[ "$REPORT" = /* ]] || { echo 'report directory must be absolute' >&2; exit 1; }
REV=96a6f9478b7f2e125cf0360a213383cf57533893
REF="$ROOT/.cache/mcp-conformance"
mkdir -p "$REPORT" "$REF"
if [[ ! -d "$REF/.git" ]]; then
  git -C "$REF" init -q
  git -C "$REF" remote add origin https://github.com/modelcontextprotocol/conformance.git
fi
git -C "$REF" fetch --depth 1 origin "$REV"
git -C "$REF" checkout --detach FETCH_HEAD
test "$(git -C "$REF" rev-parse HEAD)" = "$REV"
(cd "$REF" && npm ci)
printf 'conformance=%s\nspec=649f7aee87d631ac4d1fdd3b12eef0c6264fdb3a\nprotocol=2026-07-28\n' "$REV" > "$REPORT/revisions.txt"

CASE=$(mktemp -d "$HOME/skills-conformance.XXXXXX")
PID=
cleanup() {
  if [[ -n "$PID" ]]; then kill "$PID" 2>/dev/null || true; wait "$PID" 2>/dev/null || true; fi
  rm -rf -- "$CASE"
}
trap cleanup EXIT
mkdir -p "$CASE/skills/a-fixture/templates" "$CASE/skills/a-fixture/empty"
cat > "$CASE/skills/a-fixture/SKILL.md" <<'YAML'
---
name: a-fixture
updated: 2026-11-14
sequence: 012
metadata:
  purpose: synthetic-conformance-only
  nested: {values: [007, 2026-11-14, true]}
description: |
  Exercise Skills directory and frontmatter conformance.
---
# Synthetic fixture
YAML
echo example > "$CASE/skills/a-fixture/templates/example.md"
for i in $(seq -w 0 99); do
  printf -v n 's%03d' "$((10#$i))"
  mkdir -p "$CASE/skills/$n"
  printf -- '---\nname: %s\ndescription: Synthetic sibling\n---\n# Fixture\n' "$n" > "$CASE/skills/$n/SKILL.md"
  printf -v part 'part-%03d.txt' "$((10#$i))"
  echo part > "$CASE/skills/a-fixture/$part"
done
printf 'source: %s/skills\ntargets: {}\n' "$CASE" > "$CASE/config.yaml"
export SKILLSHARE_CONFIG="$CASE/config.yaml"
unset SKILLSHARE_MCP_TOKEN MCPGODEBUG CODEX_HOME CLAUDE_CONFIG_DIR PI_CODING_AGENT_DIR
cd "$ROOT"
go build -o "$CASE/skillshare" ./cmd/skillshare
# Port 0 keeps concurrent test runs independent; mcp serve reports the bound address.
"$CASE/skillshare" mcp serve -g --http 127.0.0.1:0 > "$REPORT/server.stdout" 2> "$REPORT/server.stderr" &
PID=$!
URL=
for _ in $(seq 1 100); do
  URL=$(sed -n 's/.*http:\/\/\(127\.0\.0\.1:[0-9]*\).*/http:\/\/\1\//p' "$REPORT/server.stderr" | head -1)
  [[ -n "$URL" ]] && break
  kill -0 "$PID"
  sleep 0.1
done
test -n "$URL"
for scenario in sep-2640-skills-enumeration sep-2640-skills-manifest sep-2640-skills-directory; do
  # Extension scenarios are outside the spec timeline; --force runs them rather
  # than skipping them. It does not suppress any conformance checks.
  node "$REF/dist/index.js" server --url "$URL" --scenario "$scenario" \
    --spec-version 2026-07-28 --force --output-dir "$REPORT/$scenario" --verbose
done
python3 - "$REPORT" <<'PY'
import json, pathlib, sys
root = pathlib.Path(sys.argv[1])
checks = []
def collect(value):
    if isinstance(value, dict):
        if 'status' in value and ('id' in value or 'checkId' in value):
            checks.append(value)
        for child in value.values(): collect(child)
    elif isinstance(value, list):
        for child in value: collect(child)
for report in root.rglob('checks.json'):
    collect(json.loads(report.read_text()))
assert checks, 'no conformance checks found'
bad = [c for c in checks if c['status'] != 'SUCCESS']
assert not bad, json.dumps(bad, indent=2)
ids = {c.get('id', c.get('checkId')) for c in checks}
required = {
    'sep-2640-capability-directory-read-flag', 'sep-2640-directory-read-method-registered',
    'sep-2640-directory-read-result-resources-shape', 'sep-2640-directory-read-subdir-mimetype',
    'sep-2640-directory-read-invalid-params', 'sep-2640-directory-read-pagination',
    'sep-2640-skillmd-metadata-name', 'sep-2640-skillmd-metadata-description',
    'sep-2640-entry-frontmatter-identical'
}
assert required <= ids, f'missing checks: {required - ids}'
print(f'{len(checks)} official checks SUCCESS; all required directory/metadata/fidelity checks ran')
PY
