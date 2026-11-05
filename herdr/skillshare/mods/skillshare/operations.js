// What each request the panel performs does on a machine: the skillshare commands in order, how
// their output reads, and what to tell the person. No Herdr imports and no view state: main.js
// takes the lock, shows progress and the result, and the tests pass a fake machine.
import {
  hookArgs, failure, parseJSON, lastLine, flag, CONFLICTS, ABORT, RESOLVE, PREVIEW, METADATA_STAGES, METADATA_WRITE,
  mergeMetadata, parseStages, parsePreview, diffSides,
} from "./model.js";

const isMetadata = (f) => f === ".metadata.json" || f.endsWith("/.metadata.json");
const lines = (r) => String(r.stdout ?? "").split("\n").map((s) => s.trim()).filter(Boolean);
const BOTH_WAYS = ["push", "--pull", "-m", "Update skills from Herdr"];
const done = (text) => ({ kind: "done", text });
const warn = (text) => ({ kind: "warn", text });
const error = (text) => ({ kind: "error", text });
const outcome = (r, ok, what) => (r.code === 0 ? done(ok) : error(failure(r, what)));

// operate(req, io) performs one request on machine req.on.
//   io.job(args, { on, cwd }): skillshare run detached; io.run(args, { on, cwd }): a short read;
//   io.sh(script, args, on): a script of ours; io.step(text): what it is doing now.
// Resolves to { kind, text } for the notice (kind null: none), and when they apply:
//   conflict: the stopped pull to ask about ({ on, repo, files, previews, push }), or null when
//     no stopped pull is left;
//   overlay: { title, text } to show; machines: true when the Machines tab should reload.
export async function operate(req, io) {
  const op = OPS[req.op];
  return op ? op(req, io, req.on ?? null) : { kind: null };
}

const at = (req, scope) => ({ on: req.on ?? null, cwd: scope === "project" ? req.project : null });

const OPS = {
  async sync(req, { job, step }) {
    for (const scope of req.scopes) {
      step(`Syncing ${scope}…`);
      const r = await job(["sync", "--all", "--json", flag(scope)], at(req, scope));
      if (r.code !== 0) return error(failure(r, "skillshare sync"));
      if (req.plugins) {
        const p = await job(["sync", "plugins", "--json", flag(scope)], at(req, scope));
        if (p.code !== 0) return error(failure(p, "skillshare sync plugins"));
      }
    }
    return done(req.scopes.length > 1 ? "Synced project and global" : `Synced ${req.scopes[0]}`);
  },

  async update(req, { job, step }) {
    let updated = 0;
    for (const u of req.updates) {
      step(u.all ? `Updating all ${u.kind === "agents" ? "agents" : "skills"} (${u.scope})…` : `Updating ${u.label}…`);
      const args = ["update", ...(u.kind === "agents" ? ["agents"] : []), ...(u.all ? ["--all"] : [u.name]), "--json", flag(u.scope)];
      const r = await job(args, at(req, u.scope));
      const j = parseJSON(r.stdout);
      const blocked = (j?.items ?? []).filter((i) => i.status === "security_blocked").map((i) => i.name);
      if (blocked.length) return error(`Audit blocked ${blocked.join(", ")}. Ask an agent to audit it before updating.`);
      if (r.code !== 0 && !j) return error(failure(r, "skillshare update"));
      const failed = [...(j?.items ?? []), ...(j?.agents ?? [])].filter((i) => i.status === "failed");
      if (failed.length) return error(`Could not update ${failed.map((i) => i.name).join(", ")}: ${failed[0].error ?? failed[0].message ?? "see skillshare update"}`);
      updated += j?.updated ?? (j?.agents ?? []).filter((a) => a.status === "updated").length ?? 0;
    }
    return done(updated ? `Updated ${updated}. Targets get it on the next sync.` : "Already up to date");
  },

  async hook(req, { job, step }) {
    const args = hookArgs(req.item, req.action);
    if (!args) return error("This hook action is unavailable.");
    step(`Saving ${req.item.label}…`);
    const r = await job(args, at(req, req.item.scope));
    return outcome(r, "Saved. Sync applies it.", "skillshare hooks");
  },

  async uninstall(req, { job, step }) {
    const it = req.item;
    step(`Uninstalling ${it.label}…`);
    const kind = it.kind === "agents" ? ["agents"] : [];
    const args = it.scope === "project" ? ["uninstall", ...kind, it.rel, "-p", "--force"] : ["uninstall", ...kind, it.rel, "--json", "-g"];
    const r = await job(args, at(req, it.scope));
    const j = parseJSON(r.stdout);
    if (r.code !== 0 || j?.failed?.length) return error(j?.failed?.length ? `Could not uninstall ${j.failed.join(", ")}` : failure(r, "skillshare uninstall"));
    return done(`Moved ${it.label} to the skillshare trash`);
  },

  async syncboth(req, io, on) {
    io.step("Syncing both ways…");
    const r = await io.job(BOTH_WAYS, { on });
    if (r.code === 0) return { ...done("Synced both ways: pushed, pulled and synced targets"), conflict: null };
    const stop = await stopOnConflict(r, req.repo, on, io, true);
    if (stop === "merged") {
      io.step("Pushing…");
      const p = await io.job(BOTH_WAYS, { on });
      return { ...outcome(p, "Merged skill records from both machines and synced both ways", "skillshare push"), ...(p.code === 0 ? { conflict: null } : {}) };
    }
    if (stop) return { ...warn("Pull stopped: both sides changed the same files. Nothing was pushed."), conflict: stop };
    return error(failure(r, "skillshare push --pull"));
  },

  async pull(req, io, on) {
    io.step("Pulling…");
    const r = await io.job(["pull"], { on });
    if (r.code === 0) return { ...done("Pulled and synced targets"), conflict: null };
    const stop = await stopOnConflict(r, req.repo, on, io, false);
    if (stop === "merged") {
      io.step("Syncing targets…");
      const sy = await io.job(["sync", "--all", "--json", "-g"], { on });
      return outcome(sy, "Pulled, merged skill records from both machines, and synced targets", "skillshare sync");
    }
    if (stop) return { ...warn("Pull stopped: both sides changed the same files. Nothing changed yet."), conflict: stop };
    return error(failure(r, "skillshare pull"));
  },

  async push(req, { job, step }, on) {
    step(req.commit ? "Committing and pushing…" : "Pushing…");
    return outcome(await job(["push", "-m", req.message], { on }), "Pushed to the remote", "skillshare push");
  },

  // Replays the stopped pull with the person's choice for each file.
  async resolve(req, io, on) {
    const { sh, job, step } = io;
    step("Applying your choices…");
    const now = lines(await sh(CONFLICTS, [req.repo], on)).filter((f) => !isMetadata(f));
    if (now.some((f) => !req.choices[f])) {
      // The remote moved since the preview: other files conflict now. Ask again instead of guessing.
      await sh(ABORT, [req.repo], on);
      const stop = await stopOnConflict({ stdout: "conflict", stderr: "" }, req.repo, on, io, req.push);
      if (stop && stop !== "merged") return { ...warn("The remote changed since you chose. Pick again."), conflict: stop };
      return { ...error("The remote changed since you chose; nothing changed"), conflict: null };
    }
    const meta = await mergeMetadataFiles(req.repo, on, sh);
    if (meta) { await sh(ABORT, [req.repo], on); return error(meta); }
    const r = await sh(RESOLVE, [req.repo, ...Object.entries(req.choices).map(([p, side]) => `${p}=${side}`)], on);
    if (r.code !== 0) { await sh(ABORT, [req.repo], on); return error(lastLine(r.stderr) || "Could not apply the choices; nothing changed"); }
    if (!req.push) {
      // Stopped by a plain pull: merge the choices and sync, but leave pushing to the person.
      step("Syncing targets…");
      const sy = await job(["sync", "--all", "--json", "-g"], { on });
      return { ...outcome(sy, "Merged your choices and synced targets. Push when you're ready.", "skillshare sync"), conflict: null };
    }
    step("Pushing…");
    const p = await job(BOTH_WAYS, { on });
    return { ...outcome(p, "Resolved and synced both ways", "skillshare push"), ...(p.code === 0 ? { conflict: null } : {}) };
  },

  async pullsync(req, { job, step }, on) {
    step(`Pulling on ${req.label}…`);
    return { ...outcome(await job(["pull"], { on }), `Pulled and synced on ${req.label}`, "skillshare pull"), machines: true };
  },

  async upgrade(req, { job, step }, on) {
    step(`Updating skillshare on ${req.label}…`);
    return { ...outcome(await job(["upgrade", "--cli", "--force"], { on }), `Updated skillshare on ${req.label}`, "skillshare upgrade"), machines: true };
  },

  async text(req, { run, step }, on) {
    step(req.busy);
    const r = await run(req.args, { on, cwd: req.cwd ?? null });
    return { kind: null, overlay: { title: req.title, text: String(r.stdout || r.stderr || "No output").replace(/\x1b\[[0-9;]*m/g, "") } };
  },
};

// A pull that both sides changed: read the files both ways, put the repository back as it was,
// and return what to ask about. Only .metadata.json changed on both sides: merge it per entry the
// way skillshare's pull does, commit, and return "merged". Not a conflict, or one that can't be
// read: false.
async function stopOnConflict(r, repo, on, { sh, step }, push) {
  if (!repo || !/both changed|merge was undone|conflict/i.test(`${r.stdout}\n${r.stderr}`)) return false;
  step("Reading conflicts…");
  const all = lines(await sh(CONFLICTS, [repo], on));
  const files = all.filter((f) => !isMetadata(f));
  if (!files.length && all.length) {
    step("Merging skill records…");
    let err = await mergeMetadataFiles(repo, on, sh);
    if (!err && (await sh(RESOLVE, [repo], on)).code !== 0) err = "Could not commit the merged skill records";
    if (err) { await sh(ABORT, [repo], on); return false; }
    return "merged";
  }
  await sh(ABORT, [repo], on);
  if (!files.length) return false;
  const shown = (l) => l && { lines: l.slice(0, 120), cut: l.length > 120 };
  const previews = Object.fromEntries(Object.entries(parsePreview((await sh(PREVIEW, [repo, ...files], on)).stdout)).map(([f, v]) => {
    const d = diffSides(v.local, v.remote);
    return [f, { differ: d.differ, local: shown(d.local), remote: shown(d.remote) }];
  }));
  return { on, repo, files, previews, push };
}

// Merges every conflicted .metadata.json in the merge in progress. Returns an error, or null.
async function mergeMetadataFiles(repo, on, sh) {
  const files = lines(await sh('cd "$1" && git -c core.quotePath=false diff --name-only --diff-filter=U', [repo], on)).filter(isMetadata);
  for (const f of files) {
    let merged;
    try { merged = mergeMetadata(...parseStages((await sh(METADATA_STAGES, [repo, f], on)).stdout)); } catch (e) { return `Could not merge ${f}: ${e.message ?? e}`; }
    const w = await sh(METADATA_WRITE, [repo, f, merged], on);
    if (w.code !== 0) return `Could not write ${f}: ${lastLine(w.stderr)}`;
  }
  return null;
}
