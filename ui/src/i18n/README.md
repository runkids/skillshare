# Dashboard Translations

The dashboard uses one JSON file per locale in `ui/src/i18n/locales/`.

## Files

- `en.json` is the canonical source of keys.
- Every other locale must contain exactly the same keys.
- Run the UI test suite after editing translations; the parity test checks missing keys, extra keys, and placeholder mismatches.

## Key Checks

`t` takes any string, so a mistyped key would only show up as the key on screen. Two tests catch it instead:

- `keys.test.ts` reads the dashboard's source. Every dotted name written out in a namespace `en.json` has, and every string passed straight to `t()`, must be a key in `en.json`. A string passed straight to `plural()` must have both its forms there.
- `internal/plugin/i18n_keys_test.go` does the same for the keys the Go plugin package sends (`messageKey`, `errorKey`, `noteKey`, `reasonKey`, `problemKey`): each `"plugins."` string literal there must be in `en.json`.

A key built at run time cannot be checked whole. In the dashboard, `` `sync.edited.${mode}` `` is checked by its fixed head: some key must start with `sync.edited.`. In Go, a literal ending in a dot, such as `"plugins.problem."+agent`, must be listed in `dynamicTranslationKeys` with the function whose `switch` cases are its values.

A string that looks like a key but is not one (a file name, an API code, a config path) fails `keys.test.ts` until it is named in `notKeys` or its file in `notKeyFiles`, with a word on what it is. A file in `notKeyFiles` is skipped only by the checks of bare dotted names and built-key heads; what it passes to `t()` and `plural()` is still checked. Do not add a real key there.

## Plurals

A counted sentence has two keys, `<key>.one` and `<key>.other`, in every locale. Pick between them with `plural`, never by hand:

```ts
t(plural('sync.changes', n), { count: n });
```

`plural` returns `.one` for exactly 1 and `.other` for everything else, the same in every locale. Languages without a singular form repeat the sentence under both keys.

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
