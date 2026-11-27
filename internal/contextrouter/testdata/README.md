# contextrouter test data

Generated; do not edit by hand.

- `cases.json`: router folders with the byte sizes that `ai-context.py check`
  prints for them, the descriptions `list` prints, and the sizes the older
  scripts v0.2.1 and v0.4.0 print for the same folders.
- `compact.json`: Markdown bodies with the link spans and the
  `compactPathLinks` output.

Both come from `runkids/agents-context-router` at tag `v0.6.0`
(`skills/agents-context-router/scripts/ai-context.py`). To regenerate, with a
checkout of that repository:

```sh
python3 scripts/testing/context_router_goldens.py \
  --router-repo <path to agents-context-router> \
  --out internal/contextrouter/testdata
```

Add or change a case in the script, never in the JSON. Following a newer
router means changing the tag there and `Reference` in `version.go`,
regenerating, and making the Go port pass again. A folder whose own script is
newer than the port knows is still measured and marked as unverified.

The fixtures are JSON rather than folders because this repository's own
`scripts/ai-context.py check` would flag loose `AGENTS.md` and `wiki/` files
here.
