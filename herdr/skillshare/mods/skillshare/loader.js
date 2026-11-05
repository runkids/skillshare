// What the panel reads from a machine: the Resources tab's data and one Machines card. No Herdr
// imports and no view state: main.js decides when to load and whether a result is still wanted,
// and the tests pass a fake machine.
//   io.job(args, { on, cwd, script }): skillshare (or a script of ours) run detached;
//   io.run(args, { on, cwd }): a short read; io.sh(script, args, on): one exec of a script.
import { SNAPSHOT, parseSnapshot, GIT_STATUS, PROJECT, parseJSON, failure, flag, parseGitStatus, driftFrom } from "./model.js";

const scopesOf = (project) => (project ? ["project", "global"] : ["global"]);
const at = (on, project, scope) => ({ on, cwd: scope === "project" ? project : null });

// The project skillshare uses for the first of folders that is one, or null.
export async function findProject({ sh }, on, folders) {
  const r = await sh(`for d in "$@"; do [ -n "$d" ] || continue; ${PROJECT.replace(/\$1/g, "$d")} && exit 0; done; true`, folders, on);
  return r.stdout.trim() || null;
}

// Every scope's resources, the source's git state and the CLI version, in one pass:
// { raw: { [scope]: { list, diff, mcp, plugins, hooks, took, error } }, git, version, error }.
// network also fetches the source's remote, so ahead/behind is current.
export async function loadResources(io, on, project, network) {
  const raw = {};
  const statusRead = io.run(["status", "--json", "-g"], { on });
  await Promise.all(scopesOf(project).map(async (scope) => {
    const r = await io.job([flag(scope)], { ...at(on, project, scope), script: SNAPSHOT });
    const snap = parseSnapshot(r.stdout);
    raw[scope] = { list: snap.list, diff: snap.diff, mcp: snap.mcp, plugins: snap.plugins, hooks: snap.hooks, took: snap.took, error: Array.isArray(snap.list) ? null : failure(r, "skillshare list") };
  }));
  const status = parseJSON((await statusRead).stdout);
  const git = status?.source?.path ? parseGitStatus((await io.sh(GIT_STATUS, [status.source.path, network ? "fetch" : ""], on)).stdout) : { repo: false };
  return { raw, git, version: status?.version ?? null, error: raw.global?.error ?? raw.project?.error ?? null };
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
  const st = parseJSON(status.stdout), d = parseJSON(doctor.stdout);
  const git = st?.source?.path ? parseGitStatus((await io.sh(GIT_STATUS, [st.source.path], on)).stdout) : null;
  return {
    on, label: on ?? (host.stdout.trim() || "this computer"), here: !on, missing: status.code === 127 || /not found/.test(status.stderr),
    version: d?.version?.current ?? st?.version ?? null, latest: d?.version?.latest ?? null, update: Boolean(d?.version?.update_available),
    doctor: d?.summary ?? null, targets: (st?.targets ?? []).filter((x) => x.skills_enabled !== false).length,
    drift: driftFrom(parseJSON(diff.stdout)).size, git,
  };
}
