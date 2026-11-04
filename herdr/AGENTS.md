# AGENTS.md — herdr/

A Herdr mod and layout for skillshare. Herdr runs mods in QuickJS with its own UI kit; nothing here is part of the Go build.

- `skillshare/` is one share folder: `RECIPE.md`, `preview.png`, `layouts/herdr-skillshare.json` (+ its png) and `mods/skillshare/`.
- `mods/skillshare/model.js` has no Herdr imports, so `herdr/tests/*.test.mjs` cover it with `make test-herdr` (part of `make check` and CI). Put new parsing there and test it.
- `main.js` imports `herdr`; check it with `herdr-app check <mod folder>` on a machine with Herdr. QuickJS rejects `??` mixed with `||`/`&&` without parentheses, which node accepts.
- Use Herdr theme tokens only. Never keep a `cx` past its callback, call `setShared`/`notify` during render, or pass `null` to `.child`.
- `model.js` reads the CLI's `--json` output and mirrors `internal/install/metadata_merge.go`. Change both sides together.
- Keep `README.md`, `RECIPE.md` and the code in step. `RECIPE.md` describes only what the build does; `preview.png` is the design board's Main page, not a capture of private data.
