// What the panel shows, built from the skillshare CLI's JSON and git's porcelain output.
// No Herdr imports, so `node --test herdr/tests` runs it.

export const KINDS = [["skills", "Skills"], ["agents", "Agents"], ["mcp", "MCP"], ["plugins", "Plugins"], ["hooks", "Hooks"]];
export const SCOPES = [["all", "All"], ["project", "Project"], ["global", "Global"]];

// Long work runs detached, its output kept in a temp folder (out, err, code, pid) that the
// panel polls. Herdr ends an exec's whole process group when the exec returns, so the job
// gets its own: setsid where it exists (Linux), or sh job control (macOS, whose sh is bash;
// dash turns job control off without a terminal, but Linux has setsid). Never both: a job
// that already leads its group makes setsid fork and exit. The job writes its own pid once
// detached, and the starter waits for it (up to 5 s) so it is not ended before then. Herdr also
// hangs up programs that carry the exec's HC_SCOPE once the exec returns (git and gh, which
// skillshare runs, died with exit 129), so the job drops it.
const DETACH = 'unset HC_SCOPE; if command -v setsid >/dev/null 2>&1; then bg() { exec setsid "$@"; }; else set -m; bg() { exec "$@"; }; fi';
// Named so SWEEP can tell the panel's job folders from anything else in the temp folder.
const TEMP = 'd=$(mktemp -d "${TMPDIR:-/tmp}/skillshare-panel.XXXXXX")';
const READY = 'i=0; while [ ! -s "$d/pid" ] && [ $i -lt 50 ]; do sleep 0.1; i=$((i+1)); done; printf %s "$d"';
// sh -c START <name> <dir> <args...>: runs skillshare <args> in <dir>; prints the temp folder.
export const START = `${DETACH}; ${TEMP}; dir=$1; shift
bg sh -c 'echo $$ >"$2/pid"; cd "$1" 2>/dev/null || cd; d=$2; shift 2; skillshare "$@" >"$d/out" 2>"$d/err" </dev/null; echo $? >"$d/code"' job "$dir" "$d" "$@" >/dev/null 2>&1 </dev/null &
${READY}`;
// The same for a script of ours: sh -c START_SCRIPT <name> <dir> <script> <args...>.
export const START_SCRIPT = `${DETACH}; ${TEMP}; dir=$1; script=$2; shift 2
bg sh -c 'echo $$ >"$1/pid"; d=$1; script=$2; shift 2; sh -c "$script" snapshot "$@" >"$d/out" 2>"$d/err" </dev/null; echo $? >"$d/code"' job "$d" "$script" "$dir" "$@" >/dev/null 2>&1 </dev/null &
${READY}`;
// sh -c ALIVE <name> <temp folder>: fails when the job stopped without writing its exit code.
// A killed job nobody reaps stays a zombie (a container without init) that kill -0 still finds,
// so a zombie counts as stopped. Where ps has no -p (busybox), only kill -0 decides.
export const ALIVE = '[ -s "$1/code" ] || { p=$(cat "$1/pid" 2>/dev/null); kill -0 "$p" 2>/dev/null && ! ps -o stat= -p "$p" 2>/dev/null | grep -q Z; } || [ -s "$1/code" ]';
// The longest the panel waits for one job (jobs.js), and so how old a job folder must be before
// SWEEP may remove it.
export const JOB_MINUTES = 15;
// sh -c SWEEP: removes job folders nobody will read: a panel that closed mid-job never collects
// its result. Keeps a job that still runs and any folder changed in the last JOB_MINUTES.
export const SWEEP = `for d in "\${TMPDIR:-/tmp}"/skillshare-panel.*; do
  [ -d "$d" ] && [ -n "$(find "$d" -prune -mmin +${JOB_MINUTES})" ] || continue
  [ ! -s "$d/code" ] && sh -c '${ALIVE}' alive "$d" && continue
  rm -rf "$d"
done; exit 0`;

// sh -c GIT_STATUS <name> <source dir> [fetch]: the repo holding the source, its branch line and
// changed files. Prints "norepo" when the source is not in a git repository.
export const GIT_STATUS = `cd "$1" 2>/dev/null || { echo norepo; exit 0; }
top=$(git rev-parse --show-toplevel 2>/dev/null) || { echo norepo; exit 0; }
cd "$top"
[ "$2" = fetch ] && git fetch -q 2>/dev/null
echo "top $top"
git status --porcelain=v1 --branch --untracked-files=all`;

// sh -c SNAPSHOT <name> <dir> <-p|-g>: every read for one scope in one trip, each section headed
// "@@<name> <seconds>" so a slow command shows.
export const SNAPSHOT = `cd "$1" 2>/dev/null || cd; f=$2
run() { n=$1; shift; s=$(date +%s); out=$(skillshare "$@" "$f" 2>/dev/null); printf '@@%s %s\\n%s\\n' "$n" "$(( $(date +%s) - s ))" "$out"; }
run list list --all --json
run diff diff --json
run mcp mcp list --json
run plugins plugin list --json
run hooks hooks list --json`;

// Sections of SNAPSHOT's output, parsed, plus how long each took.
export function parseSnapshot(text) {
  const out = { took: {} };
  for (const part of String(text ?? "").split(/^@@/m).slice(1)) {
    const nl = part.indexOf("\n");
    const [name, secs] = part.slice(0, nl).split(" ");
    out[name] = parseJSON(part.slice(nl + 1));
    out.took[name] = Number(secs);
  }
  return out;
}

// sh -c PROJECT <name> <folder>: prints the folder when skillshare treats it as a project.
export const PROJECT = '[ -f "$1/.skillshare/config.yaml" ] || [ -f "$1/skillshare/config.yaml" ] && printf "%s" "$1"';

// sh -c DISCOVER <name> <git url> [branch]: "@dir <temp folder>" (the clone is kept in <temp>/r for the
// audit step; the caller removes it), then the skill folders, one per line ("." for a skill at
// the root). The CLI's install dry-run does not list them.
export const DISCOVER = `d=$(mktemp -d)
GIT_TERMINAL_PROMPT=0 git clone -q --depth 1 \${2:+-b "$2"} "$1" "$d/r" >/dev/null 2>"$d/err" || { tail -n 1 "$d/err" >&2; rm -rf "$d"; exit 1; }
echo "@dir $d"
cd "$d/r" || exit 1
find . -name SKILL.md -not -path './.git/*' -not -path '*/node_modules/*' | sed 's|/SKILL.md$||; s|^\\./||' | sort`;

// Through START_SCRIPT: AUDIT <cwd> <temp folder> <-g|-p> <skill folders...>. One
// "@@ <folder>" line, then that folder's `skillshare audit <path> --json`, per folder.
export const AUDIT = `cd "$1" 2>/dev/null || cd; dir=$2; fl=$3; shift 3
for p; do echo "@@ $p"; skillshare audit "$dir/r/$p" --json "$fl" 2>/dev/null; echo; done`;

// sh -c PREVIEW <name> <repo> <files...>: each conflicting file as this computer committed it and
// as the remote has it ("@@gone" when that side deleted it), capped at 2000 lines.
export const PREVIEW = `cd "$1" || exit 1; shift
for p; do
  for side in local remote; do
    if [ $side = local ]; then rev=HEAD; else rev='@{u}'; fi
    printf '@@%s %s\\n' "$side" "$p"
    if git cat-file -e "$rev:$p" 2>/dev/null; then git show "$rev:$p" | head -n 2000; else echo "@@gone"; fi
  done
done`;

// sh -c CONFLICTS <name> <repo>: replays the merge that push --pull undid and lists the files
// both sides changed. The merge stays in progress until RESOLVE or ABORT.
export const CONFLICTS = `cd "$1" || exit 1
git merge --abort 2>/dev/null || true
git merge --no-ff --no-commit @{u} >/dev/null 2>&1 || true
git -c core.quotePath=false diff --name-only --diff-filter=U`;

// sh -c RESOLVE <name> <repo> <path=local|remote>...: takes one side of each file and commits the
// merge. .metadata.json is merged per entry first (mergeMetadata, METADATA_*); any other file
// still unmerged stops it, so nothing is resolved without a choice.
export const RESOLVE = `set -e
cd "$1"; shift
for c in "$@"; do
  p=\${c%=*}; side=\${c##*=}
  if [ "$side" = remote ]; then flag=--theirs; else flag=--ours; fi
  git checkout "$flag" -- "$p" 2>/dev/null && git add -- "$p" || git rm -q -- "$p"
done
if [ -n "$(git diff --name-only --diff-filter=U)" ]; then echo "unchosen conflicts remain" >&2; exit 1; fi
git commit -q --no-edit`;

// sh -c METADATA_STAGES <name> <repo> <path>: the file at the merge base, here and on the remote,
// each after an "@@stage N" line (empty when that side lacks it).
export const METADATA_STAGES = `cd "$1" || exit 1
for n in 1 2 3; do echo "@@stage $n"; git show ":$n:$2" 2>/dev/null; done`;

// sh -c METADATA_WRITE <name> <repo> <path> <content>: writes the merged file and stages it.
export const METADATA_WRITE = `cd "$1" && printf '%s' "$3" > "$2" && git add -- "$2"`;

export const ABORT = 'cd "$1" && git merge --abort 2>/dev/null; true';

// ---- Parsing ----

// The same per-entry merge skillshare's own pull does (internal/install/metadata_merge.go): an
// entry changed on one side takes that side; changed on both, the later installed_at wins (the
// remote on a tie) and an edit beats a delete; a local delete of an entry the remote left alone sticks.
export function mergeMetadata(base, ours, theirs) {
  const parse = (text) => { const j = String(text ?? "").trim() ? JSON.parse(text) : {}; j.entries ??= {}; return j; };
  const b = parse(base), o = parse(ours), th = parse(theirs);
  const canon = (v) => JSON.stringify(v, (_k, x) => (x && typeof x === "object" && !Array.isArray(x) ? Object.fromEntries(Object.entries(x).sort()) : x));
  const same = (x, y) => canon(x ?? null) === canon(y ?? null);
  const at = (e) => Date.parse(e?.installed_at ?? "") || 0;
  const merged = th;
  for (const [name, mine] of Object.entries(o.entries)) {
    const old = b.entries[name], remote = th.entries[name];
    if (same(mine, old)) continue;
    if (same(remote, old) || remote == null || at(mine) > at(remote)) merged.entries[name] = mine;
  }
  for (const name of Object.keys(th.entries)) {
    if (!(name in o.entries) && b.entries[name] != null && same(th.entries[name], b.entries[name])) delete merged.entries[name];
  }
  return JSON.stringify(merged, null, 2) + "\n";
}

export function parseStages(text) {
  const out = ["", "", ""];
  let cur = -1;
  for (const line of String(text ?? "").split("\n")) {
    const m = line.match(/^@@stage ([123])$/);
    if (m) { cur = Number(m[1]) - 1; continue; }
    if (cur >= 0) out[cur] += `${line}\n`;
  }
  return out;
}

// The CLI flag for a scope.
export const flag = (scope) => (scope === "project" ? "-p" : "-g");

export function parseJSON(text, fallback = null) {
  try { return JSON.parse(text); } catch { return fallback; }
}

// The last useful line of a command's output, for a notice.
export function lastLine(text) {
  return String(text ?? "").replace(/\x1b\[[0-9;]*m/g, "").trim().split("\n").filter((l) => l.trim()).pop()?.trim() ?? "";
}

// An error JSON ({"error": "..."}) or the last line of stderr.
export function failure(r, what) {
  const j = parseJSON(r?.stdout ?? "");
  return j?.error || lastLine(r?.stderr) || lastLine(r?.stdout) || `${what} exited with ${r?.code}`;
}

// "## main...origin/main [ahead 1, behind 2]" and the changed files.
export function parseGitStatus(text) {
  const lines = String(text ?? "").split("\n");
  if (lines[0]?.trim() === "norepo") return { repo: false };
  if (!lines.some((l) => l.startsWith("## "))) return null; // the read failed: unknown, not clean
  const out = { repo: true, top: null, branch: null, upstream: null, ahead: 0, behind: 0, files: [] };
  for (const line of lines) {
    if (line.startsWith("top ")) out.top = line.slice(4);
    else if (line.startsWith("## ")) {
      const m = line.slice(3).match(/^(?:No commits yet on )?(\S+?)(?:\.\.\.(\S+))?(?: \[(.*)\])?$/);
      if (m) {
        out.branch = m[1]; out.upstream = m[2] ?? null;
        const a = m[3]?.match(/ahead (\d+)/), b = m[3]?.match(/behind (\d+)/);
        out.ahead = a ? Number(a[1]) : 0; out.behind = b ? Number(b[1]) : 0;
      }
    } else if (line.length > 3) out.files.push({ code: line.slice(0, 2).trim() || "?", path: line.slice(3) });
  }
  return out;
}

// The source line in the footer, and whether it needs attention.
export function sourceLine(git) {
  if (!git) return { text: "Source · checking…", warn: false };
  if (!git.repo) return { text: "Source · not a git repository", warn: false };
  const parts = [];
  if (git.files.length) parts.push(`${git.files.length} uncommitted`);
  if (git.ahead) parts.push(`↑${git.ahead}`);
  if (git.behind) parts.push(`↓${git.behind}`);
  if (!git.upstream) parts.push("no remote");
  return { text: `Source · ${parts.length ? parts.join(" · ") : "up to date"}`, warn: Boolean(git.files.length || git.behind) };
}

// Resource names the targets still need, per kind, from `diff --json`. Items a sync would not
// touch (is_sync false: local-only in a target) are left out.
export function driftFrom(diff) {
  const out = new Map();
  for (const t of diff?.targets ?? []) {
    for (const item of t.items ?? []) {
      if (!item.is_sync) continue;
      const kind = item.kind === "agent" ? "agents" : "skills";
      const key = `${kind}:${item.name.replace(/\.md$/, "")}`;
      const was = out.get(key);
      out.set(key, { action: was?.action === "modify" ? "modify" : item.action, reason: was?.reason ?? item.reason, targets: [...(was?.targets ?? []), t.name] });
    }
  }
  return out;
}

// What each target still needs, most first: [{ name, n }]; n is 0 for a target in sync.
export function targetDrift(diff) {
  return (diff?.targets ?? []).map((t) => ({ name: t.name, n: (t.items ?? []).filter((i) => i.is_sync).length }))
    .sort((a, b) => b.n - a.n || a.name.localeCompare(b.name));
}

// Every target of the shown scopes and what still waits on it, most waiting first:
// { known, list: [{ name, n }], scopes }. raw: { [scope]: { diff, targets } }; diff leaves out targets
// that share another's folder, so targets adds the configured names. diff counts skills and agents only, so a waiting MCP
// server, plugin or hook in items adds one to each target it waits on. known: a diff answered;
// scopes: the ones with a waiting target.
export function targetList(raw, scope = "all", items = []) {
  const by = new Map();
  let known = false;
  const scopes = [];
  for (const s of Object.keys(raw ?? {})) {
    if (scope !== "all" && s !== scope) continue;
    if (raw[s]?.diff?.targets) known = true;
    for (const x of targetDrift(raw[s]?.diff)) { by.set(x.name, (by.get(x.name) ?? 0) + x.n); if (x.n && !scopes.includes(s)) scopes.push(s); }
    for (const name of raw[s]?.targets ?? []) if (!by.has(name)) by.set(name, 0);
  }
  for (const i of items) {
    if (!(scope === "all" || i.scope === scope) || i.notice || !waitsForSync(i) || !["mcp", "plugins", "hooks"].includes(i.kind)) continue;
    for (const r of previewTargets(i)) if (r.warn && by.has(r.name)) by.set(r.name, by.get(r.name) + 1);
  }
  return { known, scopes, list: [...by].map(([name, n]) => ({ name, n })).sort((a, b) => b.n - a.n || a.name.localeCompare(b.name)) };
}

// A row Sync would act on: a waiting resource, or a hook notice with native changes.
export const waitsForSync = (i) => (i.notice ? Boolean(i.pending) : i.state === "sync" || i.state === "missing");

// The scopes Sync runs in for the shown scope: every one with a waiting row or a waiting target
// (a target can wait with no row left, as after an uninstall); with nothing waiting, the shown ones.
export function syncScopes(items, raw, scope = "all") {
  const shown = (s) => scope === "all" || s === scope;
  const out = new Set(items.filter((i) => shown(i.scope) && waitsForSync(i)).map((i) => i.scope));
  for (const s of targetList(raw, scope).scopes) out.add(s);
  if (!out.size) for (const s of Object.keys(raw ?? {})) if (shown(s)) out.add(s);
  return [...out].sort();
}

// The map's lines, max at most: waiting targets first, then the ones in sync. What does not fit
// becomes a count: [{ name, n, wait } | { more, wait }].
export function mapLanes(targets, max = 6) {
  const pending = targets.filter((x) => x.n), fine = targets.filter((x) => !x.n);
  const slots = fine.length ? max - 1 : max;
  const rows = (pending.length <= slots ? pending : pending.slice(0, slots - 1)).map((x) => ({ name: x.name, n: x.n, wait: true }));
  if (pending.length > rows.length) rows.push({ more: pending.length - rows.length, wait: true });
  const room = max - rows.length, named = fine.length <= room ? fine : fine.slice(0, room - 1);
  rows.push(...named.map((x) => ({ name: x.name, n: 0, wait: false })));
  if (fine.length > named.length) rows.push({ more: fine.length - named.length, wait: false });
  return rows;
}

const plural = (n, word) => `${n} ${word}${n === 1 ? "" : "s"}`;
const andMore = (list) => (list.length <= 2 ? list.map((x) => x.name).join(" and ") : `${list[0].name}, ${list[1].name} and ${list.length - 2} more`);

// Overview's answer in two lines: { kind, headline, sub }, kind one of loading, failed, moving,
// waiting, ok. pending: rows Sync would act on; lag: targets with something waiting.
export function overviewAnswer({ loading, failed, error, moving, pending = 0, lag = [], total = 0, targets = 0 }) {
  if (loading) return { kind: "loading", headline: "Reading skillshare", sub: "one moment" };
  if (failed) return { kind: "failed", headline: "skillshare could not be read", sub: error ?? "" };
  if (moving) return { kind: "moving", headline: lag.length ? `Syncing ${plural(lag.length, "target")}` : "Syncing", sub: lag.length ? andMore(lag) : "every target" };
  if (pending || lag.length) {
    return { kind: "waiting", headline: lag.length ? `${plural(lag.length, "target")} need${lag.length === 1 ? "s" : ""} a sync` : `${plural(pending, "resource")} to sync`,
      sub: pending ? `${plural(pending, "resource")} waiting${lag.length ? ` for ${andMore(lag)}` : ""}` : `waiting on ${andMore(lag)}` };
  }
  return { kind: "ok", headline: "Everything is in sync", sub: `${plural(total, "resource")} · ${plural(targets, "target")}` };
}

// A machine card's remote line, and the targets that wait on it by name (max at most).
export function remoteLine(git) {
  const dirty = git?.files?.length ?? 0;
  return [git?.behind ? `↓${git.behind} behind` : null, dirty ? `${dirty} uncommitted` : null, git?.ahead ? `↑${git.ahead} to push` : null].filter(Boolean).join(" · ") || "up to date";
}
export const waitingNames = (waits, max = 3) => waits.slice(0, max).map((x) => `${x.name} ${x.n}`).join(" · ") + (waits.length > max ? ` · +${waits.length - max} more` : "");

// A machine with uncommitted or unpushed work and an upstream: a plain pull would refuse, so it syncs both ways.
export const needsBothWays = (git) => Boolean((git?.files?.length || git?.ahead) && git?.upstream);

const shortSource = (s) => String(s ?? "").replace(/^https?:\/\//, "").replace(/\.git(\/|$)/, "$1").replace(/\/$/, "");

// One scope's rows for every kind. `raw` holds each command's parsed JSON (missing ones are null).
export function buildItems(scope, raw) {
  const items = [];
  const drift = driftFrom(raw.diff);
  const checks = new Map();
  for (const s of raw.check?.skills ?? []) checks.set(`skills:${s.name}`, s);
  for (const s of Array.isArray(raw.checkAgents) ? raw.checkAgents : []) checks.set(`agents:${s.name}`, s);
  const behind = new Map((raw.check?.tracked_repos ?? []).map((r) => [r.name, r]));

  for (const s of Array.isArray(raw.list) ? raw.list : []) {
    const kind = s.kind === "agent" ? "agents" : "skills";
    const flat = kind === "agents" ? s.name.replace(/\.md$/, "") : s.name;
    const rel = String(s.relPath ?? s.name).replace(/\.md$/, "");
    const check = checks.get(`${kind}:${rel}`) ?? checks.get(`${kind}:${flat}`);
    const repo = s.repoName ? behind.get(s.repoName) : null;
    const d = drift.get(`${kind}:${flat}`);
    const item = {
      key: `${scope}:${kind}:${flat}`, kind, scope, name: flat, label: rel.split("/").pop(), rel, path: String(s.relPath ?? s.name),
      src: shortSource(check?.source ?? s.source) || (s.repoName ? `tracked repo ${s.repoName}` : rel.includes("/") ? rel : "local"),
      disabled: Boolean(s.disabled), version: check?.version ?? null, state: "ok", repo: s.repoName || null,
    };
    if (item.disabled) item.state = "off";
    else if (check?.status === "update_available" || repo?.status === "behind") {
      item.state = "update"; item.behind = repo?.behind ?? 0; item.update = repo ? repo.name : rel;
    } else if (d) { item.state = d.action === "add" ? "missing" : "sync"; item.reason = d.reason; item.targets = d.targets; }
    items.push(item);
  }

  const mcp = new Map();
  for (const c of raw.mcp?.changes ?? []) {
    const m = mcp.get(c.name) ?? { targets: [], pending: [] };
    m.targets.push(c.target);
    if (c.action && c.action !== "unchanged") m.pending.push(`${c.target}: ${c.action}`);
    mcp.set(c.name, m);
  }
  for (const [name, m] of mcp) {
    items.push({ key: `${scope}:mcp:${name}`, kind: "mcp", scope, name, label: name, src: "MCP server",
      state: m.pending.length ? "sync" : "ok", reason: m.pending.join(", "), targets: m.targets });
  }

  for (const [name, p] of Object.entries(raw.plugins?.packages ?? {})) {
    const bindings = Object.values(p.bindings ?? {});
    const pending = Object.entries(p.bindings ?? {}).filter(([, b]) => b.pending || b.sync === false).map(([t]) => t);
    items.push({ key: `${scope}:plugins:${name}`, kind: "plugins", scope, name, label: name,
      src: shortSource(p.source ?? bindings[0]?.id ?? "") || "plugin", version: p.version ?? bindings[0]?.version ?? null,
      state: pending.length ? "sync" : bindings.length ? "ok" : "missing", reason: pending.length ? `pending in ${pending.join(", ")}` : "", targets: Object.keys(p.bindings ?? {}) });
  }
  const hooks = new Map();
  const status = (id, root = "") => {
    const item = { key: `${scope}:hooks:status:${id}`, kind: "hooks", scope, root, name: "Hook status", label: "Hook status", targets: [], changes: [], declared: false, notice: true, pending: false };
    hooks.set(item.key, item);
    return item;
  };
  const hook = (root, name) => {
    const key = `${scope}:hooks:${root ? root + "/" : ""}${name}`;
    if (!hooks.has(key)) hooks.set(key, { key, kind: "hooks", scope, root, name, label: name, targets: [], changes: [], declared: false });
    return hooks.get(key);
  };
  for (const [root, source] of [["", raw.hooks?.source], ...Object.entries(raw.hooks?.source?.projects ?? {})]) {
    for (const [name, entry] of Object.entries(source?.entries ?? {})) {
      Object.assign(hook(root, name), { declared: true, disabled: entry.enabled === false, description: entry.description, targets: Object.keys(entry.bindings ?? {}) });
    }
  }
  for (const [index, c] of (raw.hooks?.plan?.changes ?? []).entries()) {
    const item = c.name ? hook(c.root ?? "", c.name) : status(index, c.root ?? "");
    if (c.target && !item.targets.includes(c.target)) item.targets.push(c.target);
    item.changes.push(c);
  }
  for (const c of raw.hooks?.plan?.changes ?? []) {
    if (!c.name && !c.target && c.action === "conflict") {
      for (const row of hooks.values()) if (!row.notice && row.root === (c.root ?? "")) row.changes.push(c);
    }
  }
  const error = raw.hooks?.error || raw.hooks?.previewError;
  if (error && Array.isArray(raw.list)) status("error");
  for (const [index, warning] of (raw.hooks?.plan?.warnings ?? []).entries()) status(`warning:${index}`).warnings = [warning];
  const changeLine = (c) => [[c.target, c.action].filter(Boolean).join(": "),
    ...new Set([c.message, c.inactiveReason].filter(Boolean))].join(" · ");
  for (const item of hooks.values()) {
    item.src = item.targets.join(", ") || "no agents";
    const pending = item.changes.filter((c) => !["unchanged", "inactive"].includes(c.action));
    item.lines = error ? (item.notice ? [error] : []) : [...item.changes.map(changeLine), ...(item.warnings ?? [])];
    item.diagnostic = Boolean(item.changes.some((c) => c.action === "inactive" || c.inactiveReason));
    const summary = pending.length ? item.changes.filter((c) => c.action !== "unchanged" || c.inactiveReason) : item.changes;
    item.reason = item.notice ? item.lines.join("; ") : summary.map(changeLine).join("; ");
    if (item.notice) {
      item.pending = item.changes.some((c) => ["add", "update", "remove", "adopt", "release"].includes(c.action));
      item.blocker = Boolean(error || item.changes.some((c) => c.action === "conflict"));
    }
    item.state = !raw.hooks?.plan ? "unknown" : pending.length ? "sync" : item.disabled ? "off" : "ok";
    items.push(item);
  }
  return items;
}

// Only top-level declared hooks can be changed with the panel's scoped CLI actions.
export function hookArgs(item, action) {
  if (item.kind !== "hooks" || !item.declared || item.root || !["enable", "disable", "remove"].includes(action)) return null;
  if ((action === "enable" && !item.disabled) || (action === "disable" && item.disabled)) return null;
  return ["hooks", action, item.name, item.scope === "project" ? "-p" : "-g"];
}

// Enabling or disabling a skill or agent → skillshare's args, or null when the row can't take
// that change. The pattern is the entry's own path (an agent's keeps its .md), as the dashboard
// writes it, so enable removes exactly what disable added.
export function toggleArgs(item, action) {
  if (!["skills", "agents"].includes(item.kind) || !item.path) return null;
  if (action !== (item.disabled ? "enable" : "disable")) return null;
  return [action, item.path, ...(item.kind === "agents" ? ["--kind", "agent"] : []), flag(item.scope)];
}

// The rows as they will be once a running request lands, so the list answers the click and not
// the read that follows it. req: what the panel was asked ({ op, item, action, updates, scopes,
// plugins, added, project }); project: the folder the rows are for, since project rows of
// another folder are not these rows. What cannot fail quietly is shown done: a disable, an
// enable, a removal. An update and a sync can be stopped (an audit, a conflict), so their rows
// only say they are running (`doing`). The read after the request replaces all of it; a request
// that fails is simply dropped, and the rows are back as they were.
export function optimistic(items, req, project = req.project) {
  const ours = (i) => i.scope !== "project" || (req.project ?? null) === (project ?? null);
  const one = (change) => items.flatMap((i) => (i.key === req.item?.key && ours(i) ? change(i) : [i]));
  const doing = (text, hit) => items.map((i) => (ours(i) && hit(i) ? { ...i, doing: text } : i));
  const able = (i) => [{ ...i, disabled: req.action === "disable", state: req.action === "disable" ? "off" : "ok" }];
  if (req.op === "toggle") return one(able);
  if (req.op === "hook") return one(req.action === "remove" ? () => [] : able);
  if (req.op === "uninstall") return one(() => []);
  if (req.op === "update") return doing("updating", (i) => i.state === "update" && (req.updates ?? []).some((u) => u.kind === i.kind && u.scope === i.scope && (u.all || u.name === i.update)));
  if (req.op === "sync") return doing("syncing", (i) => !i.notice && waitsForSync(i) && (req.scopes ?? []).includes(i.scope) && (i.kind !== "plugins" || req.plugins));
  if (req.op === "refresh" && req.added?.length) {
    const fresh = req.added.filter((a) => ours(a) && !items.some((i) => i.kind === "skills" && i.scope === a.scope && i.label === a.name));
    return [...items, ...fresh.map((a) => ({ key: `${a.scope}:skills:${a.name}`, kind: "skills", scope: a.scope, name: a.name, label: a.name, rel: a.name, path: a.name, src: shortSource(a.source) || "local", disabled: false, version: null, state: "ok", repo: null }))];
  }
  return items;
}

// A row's one short state; everything longer is in its preview.
export function rowState(item, showScope) {
  const pre = showScope ? `${item.scope} · ` : "";
  if (item.notice) return item.scope;
  if (item.doing) return `${pre}${item.doing}…`;
  const state = { update: "update available", missing: "new", sync: "changed", unknown: "sync status unknown" }[item.state];
  if (state) return pre + state;
  if (item.state === "off") return `${pre}disabled`;
  if (item.diagnostic) return `${pre}inactive on an agent`;
  if (item.kind === "mcp" || item.kind === "hooks") { const n = item.targets?.length ?? 0; return `${pre}${n} ${item.kind === "mcp" ? "target" : "agent"}${n === 1 ? "" : "s"}`; }
  const src = sourceLabel(item).label;
  return `${pre}${src}${item.version && item.kind === "plugins" ? ` · ${item.version}` : ""}`;
}

// The targets a skill or an agent can go to in its scope, by name. one: raw[scope], whose
// skillTargets and agentTargets come from status; a target the item waits on is always one.
// diff's targets stand in only when status named none: diff also lists targets that take agents alone.
export function eligibleTargets(item, one) {
  const base = item.kind === "agents" ? one?.agentTargets ?? [] : one?.skillTargets?.length ? one.skillTargets : (one?.diff?.targets ?? []).map((x) => x.name);
  return [...new Set([...base, ...(item.state === "off" ? [] : item.targets ?? [])])].sort();
}

// Who gets an item and what each still needs, waiting first: [{ name, text, warn }].
// names: every target of the item's scope, for skills and agents (their rows name only the waiting ones).
export function previewTargets(item, names = []) {
  let rows;
  if (item.kind === "hooks") {
    const by = new Map((item.targets ?? []).map((n) => [n, { name: n, text: "in sync", warn: false }]));
    for (const c of item.changes ?? []) if (c.target) by.set(c.target, { name: c.target, text: c.action === "unchanged" ? "in sync" : c.action, warn: !["unchanged", "inactive"].includes(c.action) });
    rows = [...by.values()];
  } else if (item.kind === "mcp" || item.kind === "plugins") {
    const waits = new Map(String(item.reason ?? "").replace(/^pending in /, "").split(", ").filter(Boolean).map((x) => { const [n, a] = x.split(": "); return [n, a ?? "pending"]; }));
    rows = (item.targets ?? []).map((n) => ({ name: n, text: waits.get(n) ?? "in sync", warn: waits.has(n) }));
  } else {
    const waits = new Set(item.targets ?? []);
    rows = names.map((n) => ({ name: n, text: item.state === "off" ? "disabled" : waits.has(n) ? "needs sync" : "in sync", warn: item.state !== "off" && waits.has(n) }));
  }
  return rows.sort((a, b) => Number(b.warn) - Number(a.warn) || a.name.localeCompare(b.name));
}

// sh -c READ_SKILL <name> <dir>: its files (40 at most), "@@", then SKILL.md's first 60 kB.
export const READ_SKILL = `cd "$1" 2>/dev/null || exit 3
find . -type f -not -path './.git/*' | sed 's|^\\./||' | sort | head -40
echo @@
head -c 60000 SKILL.md 2>/dev/null`;
// sh -c READ_FILE <name> <path>: a file's first 60 kB.
export const READ_FILE = 'head -c 60000 "$1" 2>/dev/null';

// A Markdown file's frontmatter as rows, and the text after it: { meta: [{ key, value }], body }.
export function splitFrontmatter(text) {
  const m = /^---\r?\n([\s\S]*?)\r?\n---[ \t]*(?:\r?\n|$)/.exec(String(text ?? ""));
  if (!m) return { meta: [], body: String(text ?? "") };
  const meta = [];
  for (const line of m[1].split(/\r?\n/)) {
    const top = /^([A-Za-z0-9_-]+):\s*(.*)$/.exec(line);
    if (top) meta.push({ key: top[1], value: /^[>|][+-]?$/.test(top[2]) ? "" : top[2].replace(/^(["'])(.*)\1$/, "$2") });
    else if (meta.length && /^\s+\S/.test(line)) { const last = meta[meta.length - 1]; last.value = `${last.value} ${line.trim().replace(/^- /, "")}`.trim(); }
  }
  return { meta, body: String(text ?? "").slice(m[0].length) };
}

// One server's settings out of the MCP source file, as rows. Values under env, headers and
// bearerToken are never returned: only their names.
const SECRET = /^(env|headers|bearerToken)$/, MASK = "••••••••";
export function mcpServer(text, name) {
  const lines = String(text ?? "").split(/\r?\n/).filter((l) => l.trim() && !l.trim().startsWith("#"));
  const depth = (l) => l.match(/^ */)[0].length, bare = (k) => k.trim().replace(/^(["'])(.*)\1$/, "$2");
  // The file can hold several server lists (the global one, and one per project root): the first that names it wins.
  let start = -1, child = null;
  for (let at = 0; at < lines.length && start < 0; at++) {
    if (!/^\s*servers:\s*$/.test(lines[at])) continue;
    child = null;
    for (let j = at + 1; j < lines.length && depth(lines[j]) > depth(lines[at]); j++) {
      child ??= depth(lines[j]);
      if (depth(lines[j]) === child && bare(lines[j].replace(/:\s*$/, "")) === name) { start = j; break; }
    }
  }
  if (start < 0) return null;
  const rows = [];
  let field = null;
  for (let j = start + 1; j < lines.length && depth(lines[j]) > child; j++) {
    field ??= depth(lines[j]);
    const s = lines[j].trim(), last = rows[rows.length - 1];
    if (depth(lines[j]) === field) {
      const m = /^([^:]+):\s*(.*)$/.exec(s);
      if (m) rows.push({ key: bare(m[1]), secret: SECRET.test(bare(m[1])), parts: m[2] ? [SECRET.test(bare(m[1])) ? MASK : m[2].replace(/^\[(.*)\]$/, "$1")] : [], inner: null });
    } else if (last) {
      last.inner ??= depth(lines[j]);
      if (depth(lines[j]) !== last.inner) continue;
      last.parts.push(last.secret ? `${bare(s.replace(/^- /, "").split(":")[0])} = ${MASK}` : bare(s.replace(/^- /, "")));
    }
  }
  return rows.map((r) => ({ key: r.key, value: r.parts.join(r.secret ? ", " : " ") }));
}

export function filterItems(items, { scope, kind, query }) {
  const q = String(query ?? "").trim().toLowerCase();
  return items.filter((i) => (scope === "all" || i.scope === scope) && (!kind || i.kind === kind)
    && (!q || i.name.toLowerCase().includes(q) || i.src.toLowerCase().includes(q)
      || (i.kind === "hooks" && `${i.root ?? ""} ${i.reason ?? ""}`.toLowerCase().includes(q))));
}

export function groupItems(items) {
  const by = (s) => items.filter((i) => !i.notice && s.includes(i.state)).sort((a, b) => a.label.localeCompare(b.label));
  return { update: by(["update"]), sync: by(["sync", "missing"]), ok: by(["ok", "off"]) };
}

// The repository a row came from: a tracked repo by name, a URL source by host/owner/repo
// (github.com left out), anything else by its source text.
function sourceLabel(item) {
  if (item.repo) return { label: item.repo, repo: true };
  const m = /^([^/\s]+\.[^/\s]+)\/([^/]+)\/([^/]+)/.exec(item.src);
  if (!m) return { label: item.src, repo: false };
  return { label: m[1] === "github.com" ? `${m[2]}/${m[3]}` : `${m[1]}/${m[2]}/${m[3]}`, repo: true };
}

// Sources with two or more rows become groups; the rest stay plain rows. Both sorted by label.
// `repo` is false for a group of local rows.
export function sourceGroups(items) {
  const by = new Map();
  for (const i of items) {
    const { label, repo } = sourceLabel(i);
    if (!by.has(label)) by.set(label, { repo, list: [] });
    by.get(label).list.push(i);
  }
  const byLabel = (a, b) => a.label.localeCompare(b.label);
  const groups = [], rows = [];
  for (const [label, { repo, list }] of by) {
    if (list.length > 1) groups.push({ key: `source:${label}`, label, repo, items: list.sort(byLabel) });
    else rows.push(list[0]);
  }
  return { groups: groups.sort(byLabel), rows: rows.sort(byLabel) };
}

export function counts(items) {
  const c = { all: 0, project: 0, global: 0, skills: 0, agents: 0, mcp: 0, plugins: 0, hooks: 0 };
  for (const i of items) { if (!i.notice) { c.all++; c[i.scope]++; c[i.kind]++; } }
  return c;
}

// "owner/repo", "github.com/owner/repo" or a URL → something git can clone, and whether the
// source names one skill inside the repo (then there is nothing to pick).
export function cloneUrl(source) {
  const s = String(source ?? "").trim().replace(/\/$/, "");
  if (!s) return null;
  if (/^(git@|ssh:\/\/|file:\/\/|\/|~)/.test(s)) return { url: s, single: false };
  let rest = s.replace(/^https?:\/\//, "");
  if (/^[\w.-]+\/[\w.-]+$/.test(rest)) rest = `github.com/${rest}`;
  const parts = rest.split("/");
  if (parts.length < 3) return null;
  const repo = parts.slice(0, 3).join("/").replace(/\.git$/, "");
  const single = parts.length > 3 && !(parts[3] === "tree" && parts.length <= 5);
  const onBranch = ["tree", "blob"].includes(parts[3]);
  const path = !single ? "" : (onBranch ? parts.slice(5) : parts.slice(3)).join("/");
  return { url: `https://${repo}.git`, single, path, branch: onBranch && parts[4] ? parts[4] : "" };
}

// Skill folders → what `install --skill` takes (the folder's own name).
export function skillChoices(paths) {
  return paths.filter((p) => p && p !== ".").map((p) => ({ path: p, name: p.split("/").pop() }));
}

export function newerVersion(a, b) {
  const n = (v) => String(v ?? "").replace(/^v/, "").split(".").map((x) => parseInt(x, 10) || 0);
  const x = n(a), y = n(b);
  for (let i = 0; i < 3; i++) if ((x[i] ?? 0) !== (y[i] ?? 0)) return (x[i] ?? 0) > (y[i] ?? 0) ? a : b;
  return a;
}

// ---- The panel's own update ----

const REPO = "https://github.com/runkids/skillshare";
// sh -c LATEST_RELEASE: prints where GitHub's "latest release" link lands (…/releases/tag/vX.Y.Z).
// The link instead of the API: it has no rate limit, and it names only a published release, never
// the tag of a draft.
export const LATEST_RELEASE = `curl -fsSLI --max-time 15 -o /dev/null -w '%{url_effective}' ${REPO}/releases/latest`;
export const releaseTag = (url) => /\/releases\/tag\/(v\d+\.\d+\.\d+)\s*$/.exec(String(url ?? ""))?.[1] ?? null;
// The release to offer: `latest` when it is newer than this panel, else null.
export const panelUpdate = (version, latest) => (latest && newerVersion(version, latest) !== version ? latest : null);

// sh -c PANEL_UPDATE <name> <dir> <tag> <from> [repo]: replaces the panel's files in Herdr's home
// on this computer with the ones release <tag> ships. It first checks the folder holds the
// running panel (version <from>): a renamed panel leaves another mod under this name. The files it
// replaces are copied once to .bak-update-<from> inside the folder (hidden, so Herdr does not load
// it), so a retry keeps the first copy. Herdr reloads the panel on every file it saves, so
// main.js goes after the modules it imports and version.js last: the build that reads the new
// version has every new file.
export const PANEL_UPDATE = `set -e
tag=$2; from=$3; repo=\${4:-${REPO}.git}; dest="\${HERDR_APP_HOME:-$HOME/.config/herdr-app}/mods/skillshare"
grep -qsF '"'"$from"'"' "$dest/version.js" || { echo "The panel is not in $dest; update it by hand" >&2; exit 1; }
t=$(mktemp -d "\${TMPDIR:-/tmp}/skillshare-panel.XXXXXX"); trap 'rm -rf "$t"' EXIT
export GIT_TERMINAL_PROMPT=0
{ git clone -q --depth 1 --filter=blob:none --sparse --branch "$tag" "$repo" "$t/repo" && git -C "$t/repo" sparse-checkout set herdr/skillshare/mods/skillshare; } >/dev/null 2>&1 || {
  rm -rf "$t/repo"; git clone -q --depth 1 --branch "$tag" "$repo" "$t/repo"; }
src="$t/repo/herdr/skillshare/mods/skillshare"
[ -f "$src/main.js" ] && [ -f "$src/version.js" ] || { echo "Release $tag has no panel update" >&2; exit 1; }
bak="$dest/.bak-update-$from"
[ -d "$bak" ] || { rm -rf "$bak.tmp"; mkdir "$bak.tmp"; cp -R "$dest"/* "$bak.tmp/"; mv "$bak.tmp" "$bak"; }
for f in "$src"/*; do case "\${f##*/}" in main.js|version.js) ;; *) cp -R "$f" "$dest/" ;; esac; done
cp "$src/main.js" "$dest/main.js"; cp "$src/version.js" "$dest/version.js"`;

// What a panel update left in shared for the build that loads after it ({ to, at }): "done" when
// this build is the one it installed, "stale" when the update never finished, else null (it is
// still copying files).
export function updateState(record, version, now = Date.now()) {
  if (!record?.to) return null;
  if (String(record.to).replace(/^v/, "") === version) return "done";
  return now - (record.at ?? 0) > JOB_MINUTES * 60 * 1000 ? "stale" : null;
}

export function ago(ms) {
  const s = Math.max(0, Math.round(ms / 1000));
  if (s < 60) return `${s}s ago`;
  if (s < 3600) return `${Math.round(s / 60)}m ago`;
  return `${Math.round(s / 3600)}h ago`;
}

// The message an agent gets when asked to audit a resource.
export function auditMessage(item) {
  const flag = item.scope === "project" ? " -p" : " -g";
  const kind = item.kind === "agents" ? "agents " : "";
  return `Audit the ${item.kind === "agents" ? "agent" : "skill"} "${item.name}": run \`skillshare audit ${kind}${item.name}${flag} --json\` and \`skillshare diff${flag}\`. Summarize what changed and any findings, then wait for me.`;
}

// DISCOVER's output → the kept clone folder and the skill folders in it.
export function parseDiscover(text) {
  const lines = String(text ?? "").split("\n").map((l) => l.trim()).filter(Boolean);
  const dir = lines[0]?.startsWith("@dir ") ? lines.shift().slice(5) : null;
  return { dir, paths: lines };
}

const SEVERITY = ["CRITICAL", "HIGH", "MEDIUM", "LOW", "INFO"];

// AUDIT's output → whether the audit would stop the install, and the findings that stop it.
// No readable result (an older CLI) is "unknown": install still audits on its own.
export function parseAudit(text, dir) {
  const prefix = dir ? `${dir}/r/` : null;
  const findings = [], blocked = [];
  let read = 0;
  for (const part of String(text ?? "").split(/^@@ /m).slice(1)) {
    const nl = part.indexOf("\n");
    const folder = part.slice(0, nl).trim();
    const j = parseJSON(part.slice(nl + 1));
    if (!j?.results) continue;
    read++;
    for (const r of j.results) {
      if (!r.isBlocked) continue;
      const skill = folder === "." ? "." : folder.split("/").pop(); // "." is the repository root; the caller names it
      blocked.push(skill);
      for (const f of r.findings ?? []) {
        const file = prefix && String(f.file ?? "").startsWith(prefix) ? f.file.slice(prefix.length) : f.file;
        findings.push({ skill, severity: f.severity, pattern: f.pattern, file, line: f.line, snippet: f.snippet ?? f.message ?? "" });
      }
    }
  }
  findings.sort((a, b) => SEVERITY.indexOf(a.severity) - SEVERITY.indexOf(b.severity));
  return { unknown: read === 0, blocked, findings };
}

// PREVIEW's output → { path: { local, remote } }, null for a side that deleted the file.
export function parsePreview(text) {
  const out = {};
  let cur = null;
  for (const line of String(text ?? "").split("\n")) {
    const m = line.match(/^@@(local|remote) (.+)$/);
    if (m) { cur = { path: m[2], side: m[1] }; (out[m[2]] ??= { local: [], remote: [] })[m[1]] = []; continue; }
    if (!cur) continue;
    const side = out[cur.path];
    if (line === "@@gone") side[cur.side] = null;
    else if (side[cur.side]) side[cur.side].push(line);
  }
  for (const v of Object.values(out)) for (const k of ["local", "remote"]) if (v[k]) v[k] = v[k].join("\n").replace(/\n$/, "");
  return out;
}

// Two versions of a file → each side's lines with the ones the other lacks marked, unchanged
// runs folded to `context` lines around each change.
export function diffSides(a, b, context = 2) {
  const x = a == null ? [] : String(a).split("\n"), y = b == null ? [] : String(b).split("\n");
  const n = x.length, m = y.length;
  const L = Array.from({ length: n + 1 }, () => new Uint16Array(m + 1));
  for (let i = n - 1; i >= 0; i--) for (let j = m - 1; j >= 0; j--) L[i][j] = x[i] === y[j] ? L[i + 1][j + 1] + 1 : Math.max(L[i + 1][j], L[i][j + 1]);
  const markX = new Array(n).fill(true), markY = new Array(m).fill(true);
  for (let i = 0, j = 0; i < n && j < m;) {
    if (x[i] === y[j]) { markX[i++] = false; markY[j++] = false; } else if (L[i + 1][j] >= L[i][j + 1]) i++; else j++;
  }
  const fold = (lines, marks) => {
    const keep = marks.map((_, i) => marks.slice(Math.max(0, i - context), i + context + 1).some(Boolean));
    const out = [];
    lines.forEach((text, i) => {
      if (keep[i]) out.push({ text, mark: marks[i] });
      else if (out.at(-1)?.gap !== true) out.push({ text: "…", gap: true });
    });
    return out;
  };
  const differ = Math.max(markX.filter(Boolean).length, markY.filter(Boolean).length);
  return { local: a == null ? null : fold(x, markX), remote: b == null ? null : fold(y, markY), differ };
}
