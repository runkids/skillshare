# CLI E2E Runbook: Disguised Executable Headers

Verify bounded header classification, hidden/cache traversal, benign twins and
separate coverage counts. Fixtures contain synthetic bytes and are never executed.

## Scope

- Renamed ELF and cached Python bytecode are HIGH, including oversized payloads.
- ZIP/JAR archives and uncertain PE headers are INFO, without runtime claims.
- Ordinary scripts and images do not produce executable findings.

## Environment

Run in the devcontainer using the skillshare-cli-e2e-test workflow. Build
`/workspace/bin/skillshare` first. mdproof supplies isolated HOME and TMPDIR.
Override directory-wide hooks to avoid shared temporary paths:

`mdproof --report json --build true --setup true --step-setup true --step-teardown true --teardown true --isolation per-runbook --workdir '$HOME' /workspace/ai_docs/tests/audit_disguised_executables_runbook.md`

## Steps

### 1. Classify hidden, cached and oversized payloads

```bash
set -eu
python3 - <<'PY'
import os, struct, zipfile
from pathlib import Path
p = Path(os.environ['TMPDIR']) / 'header-payloads'
(p / '.cache').mkdir(parents=True, exist_ok=True)
(p / '__pycache__').mkdir(exist_ok=True)
(p / 'SKILL.md').write_text('# Synthetic header fixtures\n')
elf = bytearray(64)
elf[:7] = b'\x7fELF\x02\x01\x01'
struct.pack_into('<HHI', elf, 16, 2, 62, 1)
struct.pack_into('<H', elf, 52, 64)
(p / '.cache' / 'guide.png').write_bytes(elf)
(p / '__pycache__' / 'module.pyc').write_bytes(b'\xcb\x0d\r\n' + bytes(12))
(p / 'large.png').write_bytes(bytes(elf) + bytes(1_000_000))
with zipfile.ZipFile(p / 'assets.zip', 'w') as archive:
    archive.writestr('payload.txt', 'synthetic archive fixture\n')
cfg = Path.home() / '.config' / 'skillshare' / 'config.yaml'
cfg.parent.mkdir(parents=True, exist_ok=True)
cfg.write_text(f'source: {p}\ntargets: {{}}\n')
PY
status=0
/workspace/bin/skillshare audit -g "$TMPDIR/header-payloads" --format json -T high > "$TMPDIR/header-payloads.json" || status=$?
test "$status" -eq 1
cat "$TMPDIR/header-payloads.json"
```

Expected:
- exit_code: 0
- jq: .summary.binaryFiles == 4 and .summary.skippedFiles == 4 and .summary.oversizedFiles == 1
- jq: [.results[].findings[] | select(.pattern == "disguised-executable" and .severity == "HIGH")] | length == 2
- jq: [.results[].findings[] | select(.pattern == "python-bytecode" and .severity == "HIGH")] | length == 1
- jq: [.results[].findings[] | select(.pattern == "archive-artifact" and .severity == "INFO")] | length == 1

### 2. Keep benign twins and uncertain formats below HIGH

```bash
set -eu
python3 - <<'PY'
import os, struct, zipfile
from pathlib import Path
p = Path(os.environ['TMPDIR']) / 'header-benign'
p.mkdir(exist_ok=True)
(p / 'SKILL.md').write_text('# Benign header fixtures\n')
(p / 'run.sh').write_text('#!/bin/sh\necho hello\n')
(p / 'guide.md').write_text('# Guide\nA shebang sample: #!/bin/sh\n')
(p / 'logo.png').write_bytes(b'\x89PNG\r\n\x1a\n' + bytes(16))
with zipfile.ZipFile(p / 'assets.jar', 'w') as archive:
    archive.writestr('payload.txt', 'synthetic archive fixture\n')
pe = bytearray(1024)
pe[:2] = b'MZ'
struct.pack_into('<I', pe, 60, 600)
pe[600:604] = b'PE\0\0'
(p / 'uncertain.png').write_bytes(pe)
cfg = Path.home() / '.config' / 'skillshare' / 'config.yaml'
cfg.parent.mkdir(parents=True, exist_ok=True)
cfg.write_text(f'source: {p}\ntargets: {{}}\n')
PY
/workspace/bin/skillshare audit -g "$TMPDIR/header-benign" --format json -T high
```

Expected:
- exit_code: 0
- jq: .summary.failed == 0 and .summary.binaryFiles == 3 and .summary.skippedFiles == 3
- jq: [.results[].findings[] | select(.pattern == "disguised-executable" or .pattern == "python-bytecode" or .pattern == "executable-artifact")] | length == 0
- jq: [.results[].findings[] | select(.pattern == "archive-artifact" and .severity == "INFO")] | length == 1
- jq: [.results[].findings[] | select(.pattern == "uncertain-header" and .severity == "INFO")] | length == 1

## Pass Criteria

- Both steps pass all assertions.
- No fixture content is executed and all fixture state stays inside mdproof's isolated directories.
