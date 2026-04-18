# Dashboard Translations

The dashboard uses one JSON file per locale in `ui/src/i18n/locales/`.

## Files

- `en.json` is the canonical source of keys.
- Every other locale must contain exactly the same keys.
- Run the UI test suite after editing translations; the parity test checks missing keys, extra keys, and placeholder mismatches.

## Key Style

- Use dot-separated keys grouped by area, for example `layout.nav.dashboard` or `api.error.not_found`.
- Keep keys stable. Rename a key only when updating every locale and all call sites.
- Prefer reusable labels under `common.*` only when the same text means the same thing in every context.

## Tone

- Keep translations natural and conversational, as if they are product UI copy a user sees while working.
- Be concise. Labels should fit buttons, nav items, badges, and dialogs without wrapping awkwardly.
- Avoid stiff literal translations, internal engineering phrasing, jokes, and slang.
- For errors, explain what happened in plain language. Keep raw technical details in placeholders or fallback text.

## Placeholders

- Use named placeholders with braces: `Updated {name}`.
- Placeholder names must match across every locale.
- Do not translate placeholder names.

## Do Not Translate

- Product and command names such as `skillshare`, `skillshare ui`, Git, and CodeMirror.
- User-owned data: file paths, skill names, repo names, branch names, commit messages, audit rule messages, and terminal output.
- API codes such as `target.not_found`; translate the text mapped to those codes instead.
