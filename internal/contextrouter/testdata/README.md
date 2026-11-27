# contextrouter test data

Generated; do not edit by hand.

- `cases.json`: router folders and the byte sizes that `ai-context.py check`
  prints for them.
- `compact.json`: Markdown bodies with the link spans and the
  `compactPathLinks` output.

Both come from `runkids/agents-context-router` at commit `19c94f9`
(`skills/agents-context-router/scripts/ai-context.py`). To regenerate, with a
checkout of that repository:

```sh
python3 scripts/testing/context_router_goldens.py \
  --router-repo <path to agents-context-router> \
  --out internal/contextrouter/testdata
```

Add or change a case in the script, never in the JSON. Following a newer
router means changing the commit there and in `ReferenceCommit`, regenerating,
and making the Go port pass again.

The fixtures are JSON rather than folders because this repository's own
`scripts/ai-context.py check` would flag loose `AGENTS.md` and `wiki/` files
here.
