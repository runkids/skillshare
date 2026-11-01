# Feature Proposal: Audit package-manager dependency source redirection

Issue: https://github.com/runkids/skillshare/issues/727

## Problem

A skill can change future package sources through commands, environment values
or shipped configuration. Existing install rules include a pip URL check, but
do not provide a complete registry-redirection policy or inspect all relevant
configuration basenames and hidden paths.

## Proposed Solution

Add a dependency-source-redirection family covering command text and explicitly
supported bundled configuration, with proposed HIGH severity for unexpected sources.

- Cover npm config/registry flags, pip index and extra-index flags/config, Poetry
  sources, Yarn configuration and direct git+http dependency sources.
- Recognize PIP_INDEX_URL, NPM_CONFIG_REGISTRY and Cargo registry-index environment
  settings. Resolve only simple literal variable assignments in a bounded local
  context; report unresolved destinations as unknown rather than inventing values.
- Inspect redirection and heredoc writes to `.npmrc`, `pip.conf`, `.yarnrc.yml`,
  `.cargo/config.toml` and `settings.xml`; include declared sources in
  `pyproject.toml`. Recognize the destination setting rather than any URL in a file.
- Explicitly collect these basenames and hidden locations under size/depth and
  source-boundary limits. `.conf`, XML and hidden `.cargo` cannot rely solely
  on current extension-based collection.
- Maintain package-manager default destinations and allow explicit trusted-source
  overrides. A private registry can be intentional; findings should explain the
  changed source and never label every non-default registry malicious.
- Redact URL userinfo, tokens and sensitive query/config values before snippets,
  errors, logs or report output. `internal/audit/credentials.go` generates
  credential-access patterns; it is not a reusable general redactor.
- Expose normal rule disabling, severity control and CLI/API/dashboard findings.
  Pair executable/config examples with prose and comment negatives.

Relevant source is `internal/audit/rules.yaml`,
`internal/audit/audit_scan_skill.go` and `internal/audit/credentials.go`.
No prerequisite proposal is required; bounded collection/redaction may be reused.

## Alternatives Considered

Command regexes alone miss shipped files. Flagging every registry URL creates
noise for legitimate private sources. A full shell interpreter or package-manager
resolver adds execution and scope that this static check does not require.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Estimate includes the complete command/config family, collection, destination
policy, bounded literal resolution, redaction, reporting, documentation and
fixtures. Cover defaults, private sources, comments, unresolved variables,
heredocs and secrets in every output format. No implementation is included.

## Open Questions

- Which package-manager versions and configuration shapes should be supported first?
- Should trusted-source policy be global, project-local or both, and how do they combine?
- Should unresolved command destinations be MEDIUM while concrete unexpected sources are HIGH?
- How should direct VCS dependencies differ from persistent registry configuration changes?
