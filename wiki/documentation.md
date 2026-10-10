# Documentation

Use when updating `README*`, `website/docs/`, command/reference/how-to/troubleshooting pages, the built-in `skills/skillshare` skill, or any code/documentation drift.

## Source First

Documentation must describe implemented behavior:

- Commands, flags, usage, and output: verify `cmd/skillshare/`.
- Targets: verify `internal/config/targets.yaml`.
- Audit rules: verify `internal/audit/rules.yaml` and analyzer code.
- Configuration fields: verify `internal/config/` and `schemas/`.
- UI behavior: verify `internal/server/`, `ui/src/api/`, and component code.

Every `--flag` claim must exist in current source or actual `--help` output. Never document planned behavior or preserve stale paths.

## Documentation Ownership

| Change | Inspect |
|---|---|
| CLI command, flag, or output | `website/docs/reference/commands/<command>.md` |
| Configuration, targets, or formats | `website/docs/reference/` and `schemas/` |
| User workflow | `website/docs/how-to/` or `getting-started/` |
| Concepts or architecture | `website/docs/understand/` |
| Error or recovery behavior | `website/docs/troubleshooting/` |
| New tool integration | `website/docs/learn/` |
| Headline feature or quickstart | `README.md` and required translations |
| Agent-facing CLI guidance | `skills/skillshare/SKILL.md` and its references |

Follow neighboring command pages for frontmatter, usage, flag tables, and examples. Use relative Markdown links. Put screenshots in `website/static/img/` using the existing `<feature>-demo.png` naming pattern.

## Built-in Skill

When public CLI behavior changes, inspect `skills/skillshare/SKILL.md` and its applicable `references/*.md`. Its frontmatter description has a consumer size limit; measure it after editing rather than assuming it still fits. The built-in skill is end-user guidance and must not contain internal development rules.

## README and Translations

`README.md` is the English source. This topic also loads the applicable `CONTRIBUTING.md` section for translation-link order and filename conventions. When English copy changes, explicitly determine whether translations must also change. Command names, versions, and installation instructions must not contradict one another across languages.

All root instructions and wiki topics are English. Preserve historical documents in their original language.

## Verification

1. Search for every changed flag, path, configuration key, and command name in source.
2. Check relative links, frontmatter, and sidebar placement. A page that has no translated copies is served as a fallback in every locale: give it absolute `/docs/...` links, and put an explicit `{#id}` on any heading it anchors into, in the English page and each `website/i18n/*` copy.
3. Build the website inside the devcontainer to validate types and broken links.
4. For public behavior changes, inspect README, built-in skill, and changelog impact. Update release history only when explicitly requested.
5. After wiki/router changes, run `python3 scripts/ai-context.py check`.
