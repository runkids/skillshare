// Compares the Extensions tab's selection with Pi's own resolution, for the
// isolated fixture of scripts/pi/extensions-e2e-fixture.sh. Pi's
// DefaultPackageManager.resolve only lists files; no extension module is
// imported (the fixture extensions throw if they ever are).
//
//   PI_ROOT=/opt/agent-clis/lib/node_modules/@earendil-works/pi-coding-agent \
//     node scripts/pi/extensions-crosscheck.mjs <api-base> <target> <agentDir> [projectRoot]
//
// With projectRoot, Pi resolves as if it trusts the project, which is what the
// tab's configured selection describes.
//
// Both directions are checked: every row the tab shows must match Pi, and every
// extension Pi resolves must appear in the tab ("omitted" otherwise). Pi's
// built-in extensions are not part of the tab and are left out of that check.
import { dirname, join } from "node:path";
import { pathToFileURL } from "node:url";

const [api, target, agentDir, projectRoot] = process.argv.slice(2);
const piRoot = process.env.PI_ROOT;
if (!api || !target || !agentDir || !piRoot) {
  console.error("usage: PI_ROOT=... node extensions-crosscheck.mjs <api-base> <target> <agentDir> [projectRoot]");
  process.exit(2);
}
const load = (p) => import(pathToFileURL(join(piRoot, "dist", p)).href);
const { DefaultPackageManager } = await load("core/package-manager.js");
const { SettingsManager } = await load("core/settings-manager.js");

const cwd = projectRoot ?? join(agentDir, "..", "no-project");
const settingsManager = SettingsManager.create(cwd, agentDir, { projectTrusted: Boolean(projectRoot) });
const pm = new DefaultPackageManager({ cwd, agentDir, settingsManager });
const resolved = await pm.resolve(async () => "skip");
const pi = new Map(resolved.extensions.map((x) => [x.path, x.enabled ? "loads" : "skipped"]));
const builtin = new Set(resolved.extensions.filter((x) => x.metadata?.source === "builtin").map((x) => x.path));
const listed = new Set();
const listedDirs = [];

const view = await (await fetch(`${api}/targets/${encodeURIComponent(target)}/pi-extensions`)).json();
const installs = { local: (p) => p.identity.replace(/^local:/, ""), npm: (p) => join(agentDir, "npm", "node_modules", p.identity.replace(/^npm:/, "")) };
let compared = 0;
const mismatches = [];
for (const p of view.packages) {
  const root = installs[p.kind]?.(p);
  if (!root) continue;
  for (const r of p.rows) {
    const abs = join(root, r.path);
    listed.add(abs);
    if (r.file !== "present" || r.selection === "unknown") continue;
    compared++;
    if (pi.get(abs) !== r.selection) mismatches.push(`${abs}: skillshare ${r.selection}, pi ${pi.get(abs) ?? "absent"}`);
  }
}
for (const f of view.folders) {
  if (f.kind !== "folder") {
    // Paths listed in settings resolve against that file's folder; a directory covers its files.
    for (const r of f.rows) listedDirs.push(join(dirname(f.path), r.path));
    continue;
  }
  for (const r of f.rows) {
    const abs = join(f.path, "..", r.path);
    listed.add(abs);
    compared++;
    if (pi.get(abs) !== r.selection) mismatches.push(`${abs}: skillshare ${r.selection}, pi ${pi.get(abs) ?? "absent"}`);
  }
}
const omitted = [...pi.keys()].filter((abs) => !builtin.has(abs) && !listed.has(abs) && !listedDirs.some((d) => abs === d || abs.startsWith(d + "/")));
console.log(JSON.stringify({ target, scope: view.scope, resolved: pi.size - builtin.size, compared, mismatches, omitted }, null, 2));
process.exit(mismatches.length === 0 && omitted.length === 0 && compared > 0 ? 0 : 1);
