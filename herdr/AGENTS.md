# AGENTS.md — herdr/

A Herdr mod and layout for skillshare. Herdr runs mods in QuickJS with its own UI kit; nothing here is part of the Go build.

- `skillshare/` is one share folder: `RECIPE.md`, `preview.png`, `layouts/herdr-skillshare.json` (+ its png) and `mods/skillshare/`.
- Only `main.js` imports Herdr. `model.js` (parsing, shell scripts), `jobs.js` (detached jobs), `channel.js` (requests, per-machine locks, notices), `operations.js` (what each request runs) and `loader.js` (what the panel reads) take Herdr's functions as arguments, so `herdr/tests/*.test.mjs` cover them with `make test-herdr` (part of `make check` and CI). Put new logic in one of them and test it; keep `main.js` to views.
- `main.js` imports `herdr`; check it with `herdr-app check <mod folder>` on a machine with Herdr. QuickJS rejects `??` mixed with `||`/`&&` without parentheses, which node accepts.
- Use Herdr theme tokens only. Never keep a `cx` past its callback, call `setShared`/`notify` during render, or pass `null` to `.child`.
- `model.js` parses the CLI's `--json` output (`loader.js` reads a few `status` and `doctor` fields) and mirrors `internal/install/metadata_merge.go`. Change both sides together.
- Keep `README.md`, `RECIPE.md` and the code in step. `RECIPE.md` describes only what the build does; `preview.png` is the design board's Main page, not a capture of private data. Redraw `panels.png` and the site copy too.
