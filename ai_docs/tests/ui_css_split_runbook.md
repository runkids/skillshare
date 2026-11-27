# Dashboard CSS Split Runbook

Verify that cutting or regrouping the dashboard stylesheets does not change what the dashboard looks like.

## Scope

- Cut (files split, no rule moved): the unminified build has the same layers and the same rules in the same order.
- Regroup (rules moved): every pair whose order flipped is listed and judged.
- The production build keeps both `backdrop-filter` and `-webkit-backdrop-filter`. lightningcss once kept only the last of such a pair, so production differed from the dev server.
- Screenshots of the old and new production builds match in Clean, Playful and Custom, light and dark.

## Environment

Run the steps inside the Linux devcontainer or a task container made from its image, with the candidate checkout at `/workspace` and Linux `ui/node_modules` installed. The baseline is `origin/main`; fetch it first, and set `CSS_BASE` to compare with another commit. The baseline build reuses the candidate's `node_modules`, so it is only valid while `ui/package.json` and `ui/pnpm-lock.yaml` are unchanged. Step 1 checks this. Output goes to `/tmp/css-split`.

## Steps

### 1. Test the checker and the token export

```bash
cd /workspace/scripts/design
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest css_rules_test tokens_test 2>&1
git -C /workspace diff --quiet "${CSS_BASE:-origin/main}" -- ui/package.json ui/pnpm-lock.yaml && echo "deps_unchanged=yes"
```

**Expected**:
- exit_code: 0
- regex: Ran \d+ tests
- deps_unchanged=yes

### 2. Build the baseline and the candidate

```bash
set -e
OUT=/tmp/css-split
rm -rf "$OUT"
mkdir -p "$OUT/base"
git -C /workspace archive "${CSS_BASE:-origin/main}" ui | tar -x -C "$OUT/base"
ln -s /workspace/ui/node_modules "$OUT/base/ui/node_modules"
for side in old new; do
  dir=/workspace/ui
  if [ "$side" = old ]; then dir="$OUT/base/ui"; fi
  cd "$dir"
  # --base / lets `vite preview` serve deep links for the screenshots.
  ./node_modules/.bin/vite build --base / --minify false --outDir "$OUT/$side-unmin" --emptyOutDir > "$OUT/$side-unmin.log" 2>&1
  ./node_modules/.bin/vite build --base / --outDir "$OUT/$side-prod" --emptyOutDir > "$OUT/$side-prod.log" 2>&1
  echo "$side css_files=$(ls "$OUT/$side-unmin"/assets/*.css | wc -l) unmin_lines=$(cat "$OUT/$side-unmin"/assets/*.css | wc -l)"
done
```

**Expected**:
- exit_code: 0
- regex: old css_files=1 unmin_lines=\d{4,}
- regex: new css_files=1 unmin_lines=\d{4,}

`--minify false` also leaves the CSS unminified (Vite 8.3), which the line count shows. More than one CSS file means a component or lazy route imports CSS, so the load order depends on which page opens first. Treat that as a failure.

### 3. Cut only: the rules are unchanged

```bash
cd /workspace
python3 scripts/design/css_rules.py --same /tmp/css-split/old-unmin/assets/*.css /tmp/css-split/new-unmin/assets/*.css
```

**Expected**:
- exit_code: 0
- regex: same: \d+ rules

A change that moves rules on purpose fails this step. Record that, and judge it with step 4.

### 4. Regroup: list the flipped pairs

```bash
cd /workspace
status=0
python3 scripts/design/css_rules.py --flips /tmp/css-split/old-unmin/assets/*.css /tmp/css-split/new-unmin/assets/*.css > /tmp/css-split/flips.txt || status=$?
cat /tmp/css-split/flips.txt
[ "$status" -le 1 ]
```

**Expected**:
- exit_code: 0
- regex: \d+ flipped pairs, 0 unmatched rules

A `layer order changed` line is a failure: it changes which layer wins. For each pair, decide whether both selectors can match one element in the same state; the tool does not know this. A pair that cannot is safe. A pair that can changes the page: move the rules back, or confirm the new result is intended. An unmatched rule was added, removed, edited, or moved to another layer or at-rule, so its order was not checked; account for each one. Write the verdicts in the pull request.

### 5. Production keeps the paired declarations

```bash
cd /tmp/css-split
for side in old new; do
  grep -ohE '(-webkit-)?backdrop-filter:' $side-prod/assets/*.css | sort | uniq -c > $side-backdrop.txt
done
diff old-backdrop.txt new-backdrop.txt && cat new-backdrop.txt
```

**Expected**:
- exit_code: 0
- regex: \d+ -webkit-backdrop-filter:
- regex: \d+ backdrop-filter:

A change that adds or removes a frosted surface changes the counts; both lines must move together.

## Visual Verification

Do this after the steps pass. mdproof does not run this section.

### Servers

Serve both production builds with `vite preview`, which uses the `/api` proxy from `ui/vite.config.ts`. In the container, run each from `/workspace/ui` with its own free port:

- `./node_modules/.bin/vite preview --base / --outDir /tmp/css-split/old-prod --host 0.0.0.0 --port <old-port> --strictPort`
- `./node_modules/.bin/vite preview --base / --outDir /tmp/css-split/new-prod --host 0.0.0.0 --port <new-port> --strictPort`

The proxy targets `localhost:${SKILLSHARE_API_PORT:-19420}`. The devcontainer already points it at its dashboard API. A task container needs its published ports and a TCP forwarder from its `localhost:19420` to `host.docker.internal:49420`; do not edit `vite.config.ts`.

Both builds read the same API. Its data may be someone's own test data, so **only look**: do not click anything that saves, syncs, installs, deletes, toggles or runs a scan. Typing into a dialog is fine when it is closed without saving.

### Screenshots

Take them on the host with a browser session of your own (`agent-browser --session <name>`), viewport 1440x900. Each origin has its own `localStorage`, so set the same state on both. The URL sets the theme and stores it:

| Theme | Query |
|---|---|
| Clean light / dark | `?style=clean&mode=light` / `?style=clean&mode=dark` |
| Playful light / dark | `?style=playful&mode=light` / `?style=playful&mode=dark` |
| Custom light / dark | `?style=custom&mode=light` / `?style=custom&mode=dark` |

With no saved custom theme, Custom is the Pine preset on the Clean base. Add `&` to pages that already have a query.

Take every page in all six themes. Choose names for `:name` from the pages' own lists, and use the same ones on both builds.

| Page | Path or setup |
|---|---|
| Dashboard | `/` |
| Skills, list / tree / tiles | `/skills` with `localStorage` `skillshare:skills-view` = `list`, `tree`, `cards`, then reload |
| Skill detail | `/skills/:name` |
| Target detail | `/targets/:name` |
| Sync | `/sync` |
| MCP | `/mcp` |
| Hooks | `/hooks` |
| Extras | `/extras` |
| Plugins | `/plugins` |
| Appearance with the custom theme editor | `/appearance` (the editor shows with a `custom` query) |
| Audit | `/audit` |
| Install dialog open | `/skills?install=url` |
| A menu open | `/skills` in list view, right-click a skill row; choose no item |
| A disabled switch | `/extras`, **Add extra**, set the target's mode to `symlink` so **flatten** is disabled; close without saving |

Also check what a still screenshot misses: hover a primary button, an icon button and a list row, and move a segmented control that only changes the view (the Skills view switch).

Compare each old and new pair, with a pixel diff where one is available, and look at the images. Dynamic content (times, loading states, animation frames) may differ; confirm any other difference is not styling by taking both again.

## Pass Criteria

- Steps 1, 2 and 5 pass.
- A cut passes step 3. A regroup passes step 4 with a written verdict for every pair.
- Old and new screenshots match for every page in all six themes, or each difference is explained and intended.
- Report what was not captured, and why.
