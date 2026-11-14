# Security Audit

Scan skills for prompt injection, data exfiltration, credential access, destructive commands, obfuscation, suspicious URLs, and broken local links. Use `audit rules --no-tui` for the installed rule set.

Local references in Markdown links/images, code paths with a file extension (prefixes `./`, `../`, `scripts/`, `references/`, or `assets/`), explicit `path:` declarations, and `skillshare-paths` fences are checked within the skill directory. Markdown links/images are document-relative. Code paths, `path:` declarations and path fences use the containing document for leading `./` or `../`; other relative code paths use the skill root, even in nested Markdown files. A nested skill (a subdirectory with its own `SKILL.md`) is its own root, also when its parent is audited. Absolute in-bundle paths accept equivalent spellings derived from the root and its symlinked ancestors. Missing targets remain LOW `dangling-link`; explicit reference escapes are HIGH `reference-escapes-root`; implicit inline-code paths outside the bundle are ignored and trailing `:line` suffixes are stripped; unreadable, unsupported or bounded-out checks are LOW `reference-check-incomplete`. Implicit glob/template and extensionless examples, URLs, anchors, ordinary identifiers and unmarked absolute code examples are excluded. Use `path:` or a path fence for required directories and extensionless files. Disable individual IDs with `enabled: false` in `audit-rules.yaml`. `doctor` uses the same checker and reports escapes as errors, missing/incomplete checks as warnings, each finding once.

## Usage

```bash
skillshare audit                   # Scan all skills
skillshare audit <name>            # Scan specific skill
skillshare audit a b c             # Scan multiple skills
skillshare audit --group frontend  # Scan all skills in a group
skillshare audit <path>            # Scan file or directory path
skillshare audit -T h              # Block on HIGH+ findings
skillshare audit -p                # Scan project skills
skillshare audit --profile strict  # Use strict profile (block HIGH+)
skillshare audit --dedupe global   # Full composite-key deduplication
skillshare audit --analyzer static # Run only the static analyzer
```

## Flags

| Flag | Description |
|------|-------------|
| `-G, --group <name>` | Scan all skills in a group (repeatable) |
| `-p, --project` | Scan project-level skills |
| `-g, --global` | Scan global skills |
| `--threshold <t>`, `-T <t>` | Block threshold: `critical\|high\|medium\|low\|info` (shorthand: `c\|h\|m\|l\|i`) |
| `--min-severity <level>` | Display only findings at/above this level; JSON/SARIF, scores and gates stay unchanged |
| `--profile <p>` | Audit profile preset: `default`, `strict`, `permissive` |
| `--dedupe <mode>` | Dedup mode: `legacy`, `global` (default) |
| `--analyzer <id>` | Only run specified analyzer (repeatable): `static`, `dataflow`, `tier`, `integrity`, `metadata`, `structure`, `cross-skill` |
| `--format <f>` | Output format: `text` (default), `json`, `sarif`, `markdown` |
| `--json` | Same as `--format json` (deprecated) |
| `--quiet, -q` | Only show skills with findings + summary |
| `--no-tui` | Disable interactive TUI, print plain text |
| `--respect-accepted` | Do not fail on findings accepted earlier with `--force`; JSON marks them `acknowledged`, SARIF adds `suppressions` |
| `--yes, -y` | Skip large-scan confirmation prompt |
| `--init-rules` | Create a starter `audit-rules.yaml` |
| `-h, --help` | Show help |

## Profiles

| Profile | Threshold | Dedupe | Use case |
|---------|-----------|--------|----------|
| `default` | `CRITICAL` | `global` | Standard — block only critical threats |
| `strict` | `HIGH` | `global` | Security-conscious teams |
| `permissive` | `CRITICAL` | `legacy` | Legacy deduplication; still blocks CRITICAL |

Explicit flags always override profile defaults.

## Severity Levels

| Level | Meaning | Default install behavior |
|-------|---------|--------------------------|
| **CRITICAL** | Prompt injection, data exfil, credential theft | **Blocked** (use `--force` to override) |
| **HIGH** | Destructive commands, hidden unicode, obfuscation | Warning shown |
| **MEDIUM** | Suspicious URLs, system path writes | Warning shown |
| **LOW** | Minor concerns, uncommon patterns | Warning shown |
| **INFO** | Informational observations | Warning shown |

### Markdown Context and Disclosure

Markdown is still scanned, including fenced code blocks. A shared Markdown parser supplies code-block context; it does not establish trust. Fenced shell samples in raw HTML blocks and comments also receive command-tier and dataflow analysis, with taint isolated per block. Raw HTML blocks do not qualify for SDK parameter downgrades.

- `prompt-injection-1`: SDK-style `system:` / `System:` parameters inside a fenced code block are downgraded from CRITICAL to HIGH. Recognized values include quoted strings, arrays, function calls, comma-terminated variables, YAML block scalars, and a quoted string or array on the next line. Uppercase `SYSTEM:` directives, bare role labels, and prose outside fences do not qualify. Other rules still scan the parameter contents.
- `prompt-injection-4`: explicit concealment of actions, changes, or instructions, and requests to hide content or remove conversation history, remain CRITICAL.
- `prompt-injection-5`: generic disclosure restrictions using `don't` or `do not` followed by `tell the user` are HIGH in all file types. These can be advice or concealment; no framework, repository, or schema wording is treated as proof of safety. A separate explicit-concealment match on the same line still blocks at CRITICAL.

HIGH findings warn at the default CRITICAL threshold and block at a HIGH threshold (the `strict` profile default). Some malicious wording can also match only the generic HIGH rule; use strict when that ambiguity must block. SDK parameter context is only a syntax heuristic. Credential access, data exfiltration, and other injection rules retain their configured severity.

Explicit global or project severity overrides, including CRITICAL, take precedence over SDK parameter downgrades. Generic disclosure restrictions now use `prompt-injection-5`; overrides for `prompt-injection-4` apply only to explicit concealment. Built-in rule 5 excludes explicit-concealment phrases, not separate generic restrictions on the same line. Full custom regex replacements retain whole-line exclusion semantics. To make generic restrictions CRITICAL, override `prompt-injection-5`. For reviewed false positives, findings accepted with `--force` remain visible and are remembered for that resource; a different rule, file, matched text, or detector-specific evidence requires fresh acceptance. For `whitespace-padding`, density findings use all eligible content in the file as evidence. Identical occurrences have separate acceptance keys; previously accepted padding findings require acceptance again.

### Python Comments and Docstrings

In `.py` files, built-in CRITICAL `credential-access` matches on comment-only lines, or inside a string that is a whole statement such as a docstring, are reported as HIGH. f-strings, assigned or argument strings, and code before a trailing comment keep CRITICAL, as does every line of a file that references `__doc__` or `getdoc` or uses bare CR line breaks, and every line after an f-string replacement field that holds a quote or `#` or does not close. Explicit severity overrides and full custom regex replacements keep their configured severity. Markdown, other file types, and other rules are unchanged. Text output, the TUIs and the dashboard mark such findings as lowered from CRITICAL (`severityContext: python-inert` in JSON and SARIF), and `audit rules` shows the context on each affected rule.

### Whitespace Padding

The `static` analyzer reports `whitespace-padding`: 20+ consecutive blank lines
followed by nonblank text (MEDIUM; 40+ HIGH), 80+ leading horizontal Unicode whitespace characters before text,
more than 4096 eligible bytes at 95%+ whitespace, or 100+ identical nonblank
lines immediately followed by different text (MEDIUM). Fenced and indented code and parsed
Markdown table rows are exempt from padding accounting in Markdown (`.md`) files.
Nonblank code/table lines still trigger a preceding blank run outside the block. Locations retain original LF-based line numbers,
including when Unicode separators, vertical tab or form feed separate visual
lines. An unclosed fence exempts content to EOF. Interior column alignment does
not trigger the horizontal check. Adjacent horizontally padded lines produce one finding. This indicates hidden-content risk, not proof
of malicious intent; density and repetition can also flag generated data.
Findings affect risk scores, and HIGH findings block at a HIGH audit threshold.
Disable or override severity/confidence by rule ID or pattern in global/project
`audit-rules.yaml`; explicit severity applies to every form. Leave `regex` and
`exclude` unset for this file-level detector.

## Bundled hooks and settings

The `structure` analyzer inspects `hooks.json`, `hooks/hooks.json`, `.claude/settings.json`, and `.claude/settings.local.json` inside a skill, with confined reads capped at 1,000,000 bytes per file. It reports lifecycle commands at MEDIUM and recognized network commands, remote POST, broad permissions, and command-valued helpers at HIGH. Findings describe potential behavior if activated. Malformed, unsupported, oversized, or unreadable declarations produce `bundled-config-unparsed` at INFO; they do not establish a clean surface. Arguments and URLs are redacted before reporting. The six `bundled-*` IDs are listed by `audit rules` and honor disabling/severity overrides. Command-valued settings inspection is limited to `apiKeyHelper` and `statusLine`; other command-valued settings are not inspected. No declaration is executed and no live settings are read.

Broad grants include `*`, bare tools, `Bash(*)`, `Bash(:*)`, and MCP server-wide grants (`mcp__server__*`). Narrow prefixes such as `Bash(git diff:*)` and scoped paths such as `Read(./docs/**)` and MCP tool prefixes such as `mcp__github__get_*` stay clean. POST evidence must come from the network command's own segment. Hidden settings receive declaration analysis only and remain excluded from text-rule scans and analyzability bytes. Existing text findings in the two visible hook files are redacted, changing their fingerprints/acceptance keys and requiring prior acceptances to be renewed.

## Config

```yaml
# config.yaml
audit:
  block_threshold: HIGH                         # Blocking severity gate
  profile: strict                               # Profile preset
  dedupe_mode: global                           # Dedup mode (global/legacy)
  enabled_analyzers: [static, dataflow, tier]   # Limit to specific analyzers
```

CLI flags override config values. Precedence: CLI > project config > global config > profile defaults.

## Install Integration

`skillshare install` auto-scans after download:

- **Findings at/above threshold → install blocked.** Inspect findings; explicit `--force` overrides the gate.
- **Findings below threshold → warning displayed** after successful install.
- **`--skip-audit`** skips security scanning entirely for a single install.

```bash
skillshare install user/repo              # Auto-audit, block on threshold
skillshare install user/repo --force      # Override block
skillshare install user/repo --skip-audit # Skip audit entirely
```

`install --json` retains the audit gate and enables noninteractive selection;
it does not permit overwriting existing items without `--force`. Explicit `--force` and `--skip-audit` have different security effects.

## Managing Rules

```bash
skillshare audit rules                          # Interactive TUI rule browser
skillshare audit rules --no-tui                 # Plain text table
skillshare audit rules disable <id>             # Disable single rule
skillshare audit rules disable --pattern <p>    # Disable entire pattern group
skillshare audit rules enable <id>              # Re-enable rule
skillshare audit rules severity <id> <level>    # Override severity
skillshare audit rules reset                    # Restore built-in defaults
skillshare audit rules init                     # Create starter audit-rules.yaml
```

Audit exits `0` when findings are below the threshold, `1` for threshold findings, and `2` for execution, usage, configuration, or scan errors. Scan errors take precedence over findings. With `--respect-accepted`, findings accepted earlier with `--force` do not count toward `1`. Declining the large-scan confirmation exits `0` without scanning. Audits launched from `list` use the same exit codes.

## Custom Audit Rules

```bash
skillshare audit --init-rules       # Global: ~/.config/skillshare/audit-rules.yaml
skillshare audit --init-rules -p    # Project: .skillshare/audit-rules.yaml
```

Three-layer merge (later overrides earlier): built-in → global → project. A rules file that cannot be read, parsed, or compiled makes every scan using it a scan error that names the file. Set `confidence` from 0 to 1 by rule ID or pattern; rules without a value default to 0.95. The HIGH prose patterns `anti-refusal`, `prompt-leakage`, `memory-poisoning`, `agent-snooping`, and `persistence` detect narrowly phrased risky directives.

```yaml
rules:
  - id: flag-todo
    severity: MEDIUM
    pattern: todo-comment
    message: "TODO comment found"
    regex: '(?i)\bTODO\b'

  - id: insecure-http-0
    enabled: false              # Disable a built-in rule

  - pattern: destructive-commands
    severity: MEDIUM            # Downgrade entire pattern group
```

## Output

Summary includes severity breakdown (`c/h/m/l/i`) and threat category breakdown (`inj`, `exfil`, `cred`, `obfusc`, `priv`, `integ`, `struct`, `risk`).

`Failed` counts skills at/above threshold. `Warning` counts skills with findings below threshold.

When a followed source link cannot be read, its skills go unscanned: JSON adds `warnings` and `incomplete: true`, Markdown lists the links under **Source links not followed**, the TUI summary line counts them, and the dashboard marks the scan **Scan incomplete**.

## Logging

Audit results are logged to `audit.log` (JSONL). View with:

```bash
skillshare log --audit
```

## File headers and coverage

Before text extension/size filters, the static analyzer checks the first 512 header
bytes per regular file, with a bounded extra read at a PE header offset when needed, including hidden paths and `__pycache__`. Disguised native
executables/WASM and shebang scripts are HIGH; ordinary executable artifacts are
MEDIUM; recognized CPython 3.6–3.14 bytecode is HIGH, or MEDIUM when a matching regular Python source exists. ZIP/JAR signatures and uncertain
headers are INFO. This does not analyze runtime behavior or archive members.
`.git`, `.skillshare`, internal/OS metadata, symlinks and special files are excluded
from header reads; the existing source-link policy and six-level depth limit stay
in effect. There is no file-count cap; directory scans count header read failures as skipped
and continue. An unreadable direct-file target still returns an error. Hidden
directories inside a skill receive header classification only. A whole-repository
scan (`install --track`, tracked updates) text-scans skills found in hidden
directories such as `.agents/skills/` or `.curated/`, but not target copies.
Links to non-special files keep existing text scanning. Special targets are
skipped; direct audits of them return an error. Hidden names alone do not imply disguise. HIGH requires a document,
image or configuration extension or a `.cache`/`__pycache__` parent directory;
other native binaries, including `.venv/bin/python`, are MEDIUM.
JSON results and summaries expose `binaryFiles`, `skippedFiles`, `oversizedFiles`;
CLI and dashboard summaries show the same counts; the dashboard, Markdown and SARIF also report per-resource counts. Skipped includes read failures, binary,
unsupported, oversized and regular files below hidden directories inside a skill, so the counters
overlap. Special files and links to special targets outside hidden directories count as skipped; other links are not counted just for skipping header reads. Header
bytes are not auditable text. The rules `disguised-executable`, `python-bytecode`,
`executable-artifact`, `archive-artifact`, `uncertain-header` can be disabled by ID
with `enabled: false` in `audit-rules.yaml`.

Direct-file disguise classification uses the filename, not ancestor directories.

## Sensitive Destinations and Deserializers

Metadata hosts are detected in every admitted text format. Prose, configuration data and text fences are HIGH; recognized source files, shebang scripts and the code fences listed below give other metadata mentions CRITICAL context. A recognized request is CRITICAL only when its parsed URL hostname matches a metadata host; a public URL with a metadata address only in its path or query stays HIGH. HIGH warns at the default CRITICAL threshold and blocks at a HIGH threshold. Explicit severity overrides take precedence; full custom regex replacements use their configured severity. Internal literal request targets are MEDIUM; direct request-input targets or interpolated hosts are MEDIUM with confidence 0.6. Language-scoped unsafe Python/PHP/Ruby/JavaScript deserialization is HIGH. Python YAML SafeLoader/CSafeLoader and torch `weights_only=True` are recognized only in the matching call. These are static signatures, not runtime SSRF or data-provenance guarantees.

Metadata code context recognizes `.sh`, `.bash`, `.zsh`, `.fish`, `.py`, `.js`, `.ts`, `.rb`, `.php`, `.php3`, `.php4`, `.php5`, `.phtml`, `.go`, `.rs`, `.swift`, `Makefile` and `Dockerfile` (case-insensitive). Markdown fences qualify when unlabelled or labelled with one of those extension names, or `python`, `javascript`, `typescript`, `ruby`, `shell`, `console`, `golang`, `rust`, `makefile`, `dockerfile`. Other fence labels (including `http`, `powershell`, `pwsh`, `node`, `jsx`, `tsx`, `java`) retain HIGH unless a recognized literal request targets metadata. Terminal sentence periods after IPv4 metadata addresses or the recognized metadata hostname are accepted; hostname suffixes such as `.example.com` do not match. Extensionless notes without a first-line shebang remain HIGH unless a recognized request targets metadata; a shebang after an initial BOM is recognized.

Custom rules accept `files: ['*.py', 'scripts/*.php']`: basename globs match any depth; slash-containing Go `path.Match` globs match relative paths. Matching is case-insensitive; `**`, escapes, absolute and parent paths are rejected. Omitting `files` in a partial override inherits the existing filter; `files: []` clears it. A full regex replacement replaces the whole rule, so omitted `files` removes the built-in scope and built-in call/context inspection does not apply. Inspect effective applicability with `audit rules --format json`.

## Display severity floor

Use `skillshare audit --min-severity high` to display HIGH and CRITICAL findings in terminal/TUI or Markdown output. Levels: critical, high, medium, low, info (case-insensitive; aliases such as h work). Reports show visible/hidden counts. This changes only display: full scores, blocking, exit codes, scan errors and completeness warnings remain. JSON/SARIF output is unchanged, without display metadata; use existing `summary.maxSeverity` in JSON. Dashboard severity counts support exact or minimum severity selection, while LOW/INFO remain collapsible when no severity is selected.
