---
sidebar_position: 2
---

# Security-First Design

> AI skills are executable instructions. skillshare treats them as untrusted input.

## The Threat Model

When you install a skill from GitHub, you're giving an AI tool instructions that will influence code generation, file modifications, and potentially command execution. A malicious skill could:

- **Inject prompts** that override the AI's safety guidelines
- **Exfiltrate data** by instructing the AI to send file contents to external URLs
- **Execute destructive commands** through the AI's shell access
- **Steal credentials** by accessing environment variables or config files

This is not theoretical. Prompt injection is the #1 security concern in AI tooling.

## The Audit Engine

skillshare includes a built-in security scanner (`skillshare audit`) that checks every installed skill against 15+ detection patterns across 5 severity levels:

| Severity | Examples |
|----------|----------|
| CRITICAL | Prompt injection, system prompt override |
| HIGH | Data exfiltration URLs, credential access patterns |
| MEDIUM | Destructive commands (`rm -rf`, `DROP TABLE`), file system writes |
| LOW | Network requests, external tool invocation |
| INFO | Large file sizes, unusual formatting |

### How It Works

The audit engine scans SKILL.md content using pattern matching and heuristics:

```bash
# Scan all installed skills
skillshare audit

# JSON output for CI integration
skillshare audit --json

# Scan project skills only
skillshare audit -p
```

### Automatic Blocking

During `skillshare install`, the audit runs automatically. If a CRITICAL finding is detected, installation is blocked:

```
CRITICAL: Prompt injection detected in "malicious-skill"
  → Pattern: "ignore previous instructions"
  → Installation blocked. Use --force to override (not recommended).
```

## Defense in Depth

The audit engine is one layer. skillshare's security model includes:

1. **Audit at install time** — catch threats before they reach your AI tools
2. **Audit on demand** — re-scan existing skills as new patterns are added
3. **Symlink isolation** — skills are symlinked, not copied, so the source remains the authority
4. **Backup before changes** — `skillshare backup` snapshots your entire skill library
5. **Trash with TTL** — deleted skills go to trash first, not permanent deletion
6. **Operation logging** — every mutating operation is logged to `operations.log` (JSONL)

## Supply Chain Considerations

The AI skill ecosystem is young. There are no package registries with review processes, no code signing, no dependency resolution. Skills are Markdown files in git repositories.

skillshare's approach:
- **Scan everything** — even skills from trusted sources
- **Block by default** — CRITICAL findings prevent installation
- **Log everything** — audit results are stored for forensic review
- **Update patterns** — new detection patterns ship with each skillshare release

## Configuring Audit Behavior

Set the block threshold in `config.yaml` to control what severity blocks installation:

```yaml
# config.yaml
audit:
  block_threshold: HIGH   # Block on HIGH and CRITICAL (default: CRITICAL)
```

For per-rule customization, use a separate `audit-rules.yaml` file (initialized with `skillshare audit --init-rules`):

```yaml
# audit-rules.yaml
rules:
  - id: network-request-0
    enabled: false          # Disable this specific rule
  - id: my-custom-check
    severity: MEDIUM
    pattern: "TODO|FIXME"
    description: Policy violation — unresolved TODOs
```

## Related

- [`audit` command reference](/docs/reference/commands/audit)
- [Security guide](/docs/how-to/advanced/security)
- [CI/CD validation recipe](/docs/how-to/recipes/ci-cd-skill-validation)
