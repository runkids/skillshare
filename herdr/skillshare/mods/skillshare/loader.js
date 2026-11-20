// What the panel reads from a machine: the Resources tab's data and one Machines card. No Herdr
// imports and no view state: main.js decides when to load and whether a result is still wanted,
// and the tests pass a fake machine.
//   io.job(args, { on, cwd, script }): skillshare (or a script of ours) run detached;
//   io.run(args, { on, cwd }): a short read; io.sh(script, args, on): one exec of a script.
import { GIT_STATUS, PROJECT, READ_SKILL, READ_EXTRA, READ_FILE, LATEST_RELEASE, releaseTag, parseJSON, failure, flag, parseGitStatus, driftFrom, targetDrift, splitFrontmatter, mcpServer } from "./model.js";

const scopesOf = (project) => (project ? ["project", "global"] : ["global"]);
const at = (on, project, scope) => ({ on, cwd: scope === "project" ? project : null });

// The project skillshare uses for the first of folders that is one, or null.
export async function findProject({ sh }, on, folders) {
  const r = await sh(`for d in "$@"; do [ -n "$d" ] || continue; ${PROJECT.replace(/\$1/g, "$d")} && exit 0; done; true`, folders, on);
  return r.stdout.trim() || null;
}

// Each read publishes independently. Keep the previous values during refresh, but track
// pending and failed sections separately so a stale value never claims a fresh sync verdict.
export async function loadResources(io, on, project, network, progress = () => {}, previous = {}) {
  const commands = { list: ["list", "--all"], diff: ["diff"], mcp: ["mcp", "list"], plugins: ["plugin", "list"], hooks: ["hooks", "list"], extras: ["extras", "list"], memory: ["extras", "memory", "list"], status: ["status"] };
  const raw = {};
  // root: the project folder, which a project's relative extras paths start from.
  for (const scope of scopesOf(project)) raw[scope] = { ...previous[scope], pending: Object.keys(commands), errors: {}, took: {}, error: null, root: scope === "project" ? project : null };
  let git = null, version = null;
  const snapshot = () => ({ raw: Object.fromEntries(Object.entries(raw).map(([scope, one]) => [scope, { ...one, pending: [...one.pending], errors: { ...one.errors }, took: { ...one.took } }])), git, version,
    error: Object.entries(raw).flatMap(([scope, one]) => Object.entries(one.errors).map(([name, message]) => `${scope} ${name}: ${message}`)).join("\n") || null });
  const publish = () => progress(snapshot());
  const names = (list) => (list ?? []).map((x) => x.name).filter(Boolean);
  publish();
  await Promise.all(scopesOf(project).flatMap((scope) => Object.entries(commands).map(async ([name, args]) => {
    const one = raw[scope], start = Date.now(), opt = at(on, project, scope);
    let status;
    try {
      const r = await (name === "status" ? io.run([...args, "--json", flag(scope)], opt) : io.job([...args, "--json", flag(scope)], opt));
      const value = parseJSON(r.stdout);
      if (r.code !== 0 || value == null || (name === "list" && !Array.isArray(value))) throw new Error(failure(r, `skillshare ${args.join(" ")}`));
      if (name === "status") {
        status = value;
        Object.assign(one, { targets: names(value.targets), targetPaths: (value.targets ?? []).map((x) => ({ name: x.name, path: x.path })), skillTargets: names((value.targets ?? []).filter((x) => x.skills_enabled !== false)), agentTargets: names(value.agents?.targets) });
        if (scope === "global") version = value.version ?? null;
      } else one[name] = value;
    } catch (e) {
      one.errors[name] = String(e?.message ?? e);
      if (name === "list") one.error = one.errors[name];
    } finally {
      one.pending = one.pending.filter((key) => key !== name);
      one.took[name] = (Date.now() - start) / 1000;
      publish();
    }
    // Source state does not wait for any resource inventory (especially native plugins).
    if (name === "status" && scope === "global") {
      try {
        if (one.errors.status) git = { error: one.errors.status };
        else if (!status?.source?.path) git = { repo: false };
        else {
          const r = await io.sh(GIT_STATUS, [status.source.path, network ? "fetch" : ""], on);
          git = r.code === 0 ? parseGitStatus(r.stdout) : { error: failure(r, "git status") };
        }
      } catch (e) { git = { error: String(e?.message ?? e) }; }
      publish();
    }
  })));
  return snapshot();
}

// What has an update upstream, per scope: { [scope]: { check, checkAgents } }. Slow: it asks
// every source's remote.
export async function loadChecks(io, on, project) {
  const out = {};
  for (const scope of scopesOf(project)) {
    const opt = at(on, project, scope), f = flag(scope);
    const [check, checkAgents] = await Promise.all([io.job(["check", "--json", f], opt), io.job(["check", "agents", "--json", f], opt)]);
    out[scope] = { check: parseJSON(check.stdout), checkAgents: parseJSON(checkAgents.stdout) };
  }
  return out;
}

// One Machines card: the CLI's version and health, drift, and the source's git state.
export async function loadMachine(io, on) {
  const [host, doctor, diff, status] = await Promise.all([
    on ? Promise.resolve({ stdout: on }) : io.sh("hostname -s"),
    io.job(["doctor", "--json", "-g"], { on }), io.run(["diff", "--json", "-g"], { on }), io.run(["status", "--json", "-g"], { on }),
  ]);
  const st = parseJSON(status.stdout), d = parseJSON(doctor.stdout), df = parseJSON(diff.stdout);
  const git = st?.source?.path ? parseGitStatus((await io.sh(GIT_STATUS, [st.source.path], on)).stdout) : null;
  return {
    on, label: on ?? (host.stdout.trim() || "this computer"), here: !on, missing: status.code === 127 || /not found/.test(status.stderr),
    version: d?.version?.current ?? st?.version ?? null, latest: d?.version?.latest ?? null, update: Boolean(d?.version?.update_available),
    doctor: d?.summary ?? null, targets: (st?.targets ?? []).filter((x) => x.skills_enabled !== false).length,
    drift: driftFrom(df).size, pending: targetDrift(df).filter((x) => x.n), git,
  };
}

// The latest published skillshare release ("v1.2.3"), asked from this computer; null when GitHub
// can't be reached.
export async function loadLatestRelease({ sh }) {
  return releaseTag((await sh(LATEST_RELEASE)).stdout);
}

// What a preview shows beyond the row: a skill's files and SKILL.md, an agent's file, an MCP
// server's settings (mcpPath is the MCP source file), or an extra's files.
// { files, dir, meta, body } | { server } | { gone } | { error }; an extra's also names the file shown.
export async function loadPreview(io, on, project, item, mcpPath) {
  if (item.kind === "mcp") return { server: mcpPath ? mcpServer((await io.sh(READ_FILE, [mcpPath], on)).stdout, item.name) : null };
  if (item.kind === "extras") {
    // A missing source has nothing to read, and reading again won't change that.
    if (item.sourceMissing) return { gone: `${item.file ? `${item.dir}/${item.file}` : item.dir} is missing. Create it, or remove the extra.` };
    if (item.extra !== "folder") return { files: [item.file], file: item.file, dir: item.dir, ...(await loadFile(io, on, item.dir, item.file)) };
    // A folder lists only the files extras sync sends, and shows its SKILL.md, which READ_EXTRA
    // already read, else its first file. A folder holds no SKILL.md as a rule, so READ_EXTRA
    // ending non-zero is not an error here.
    const r = await io.sh(READ_EXTRA, [item.dir], on), cut = r.stdout.indexOf("@@\n");
    if (cut < 0) return { error: `${item.dir} could not be read.` };
    const files = r.stdout.slice(0, cut).split("\n").filter(Boolean), text = r.stdout.slice(cut + 3);
    if (text && !files.includes("SKILL.md")) files.unshift("SKILL.md");
    if (!files.length) return { error: `${item.dir} has no files to show.` };
    if (files.includes("SKILL.md")) return { files, file: "SKILL.md", dir: item.dir, ...splitFrontmatter(text) };
    return { files, file: files[0], dir: item.dir, ...(await loadFile(io, on, item.dir, files[0])) };
  }
  const st = parseJSON((await io.run(["status", "--json", flag(item.scope)], at(on, project, item.scope))).stdout);
  const base = item.kind === "agents" ? st?.agents?.source : st?.source?.path;
  if (!base) return { error: "skillshare did not say where this source is." };
  if (item.kind === "agents") {
    const r = await io.sh(READ_FILE, [`${base}/${item.rel}.md`], on);
    return r.stdout ? { files: [`${item.label}.md`], dir: null, ...splitFrontmatter(r.stdout) } : { error: `${item.rel}.md could not be read.` };
  }
  const dir = `${base}/${item.rel}`, r = await io.sh(READ_SKILL, [dir], on);
  const cut = r.stdout.indexOf("@@\n");
  if (r.code !== 0 || cut < 0) return { error: `${dir} could not be read.` };
  const files = r.stdout.slice(0, cut).split("\n").filter(Boolean), text = r.stdout.slice(cut + 3);
  // The list stops at 40 names; the document it shows is always one of them.
  if (text && !files.includes("SKILL.md")) files.unshift("SKILL.md");
  return { files, dir, ...splitFrontmatter(text) };
}

// Another file of a previewed skill.
export async function loadFile(io, on, dir, file) {
  const text = (await io.sh(READ_FILE, [`${dir}/${file}`], on)).stdout;
  return /\.md$/i.test(file) ? splitFrontmatter(text) : { meta: [], body: text, plain: true };
}
