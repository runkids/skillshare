// Phase 0 contract check for Pi package-resource filters (issue #342).
//
// Runs Pi's own DefaultPackageManager / SettingsManager against throwaway
// fixture directories and asserts the results. It only resolves paths: no
// package is installed (onMissing => "skip", PI_OFFLINE=1) and no extension
// module is imported (fixture extensions throw if they ever are).
// Trust decisions written here are test inputs inside the fixture root only.
//
// Run inside the devcontainer only:
//   PI_ROOT=/home/developer/.local/agent-clis/lib/node_modules/@earendil-works/pi-coding-agent \
//   node scripts/pi/phase0-contract.mjs
// Exit 0 means every assertion passed.

import assert from "node:assert/strict";
import { existsSync, mkdtempSync, mkdirSync, writeFileSync, readFileSync, rmSync, symlinkSync, utimesSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, relative } from "node:path";
import { pathToFileURL } from "node:url";

const piRoot = process.env.PI_ROOT;
if (!piRoot) throw new Error("PI_ROOT is required");
process.env.PI_OFFLINE = "1";

const load = (p) => import(pathToFileURL(join(piRoot, "dist", p)).href);
// PI_ENTRY=bundle runs the classes from the bundle the pi CLI executes
// (dist/bundle/index.js, which only re-exports and starts nothing); the default
// "core" uses dist/core. The bundle does not export resolveProjectTrusted, so
// that one function comes from dist/core either way.
const entry = process.env.PI_ENTRY ?? "core";
if (entry !== "core" && entry !== "bundle") throw new Error(`PI_ENTRY must be core or bundle, not ${entry}`);
const api = entry === "bundle"
  ? await load("bundle/index.js")
  : { ...(await load("core/package-manager.js")), ...(await load("core/settings-manager.js")), ...(await load("core/trust-manager.js")) };
const { DefaultPackageManager, SettingsManager, hasTrustRequiringProjectResources, ProjectTrustStore } = api;
for (const [name, value] of Object.entries({ DefaultPackageManager, SettingsManager, hasTrustRequiringProjectResources, ProjectTrustStore })) {
  if (typeof value !== "function") throw new Error(`${entry} entry does not export ${name}`);
}
const { resolveProjectTrusted } = await load("core/project-trust.js");
const version = JSON.parse(readFileSync(join(piRoot, "package.json"), "utf8")).version;

let scenarios = 0;
let assertions = 0;
const eq = (actual, expected, message) => {
  assert.deepEqual(actual, expected, message);
  assertions++;
};
async function scenario(name, fn) {
  await fn();
  scenarios++;
  console.log(`ok ${scenarios} - ${name}`);
}

const root = mkdtempSync(join(tmpdir(), "pi-phase0-"));
try {
  process.env.HOME = join(root, "home");
  mkdirSync(process.env.HOME, { recursive: true });

  // One local package with three extensions, one skill, one prompt.
  const pkg = join(root, "pkgs", "tools");
  for (const d of ["extensions", "skills/review", "prompts"]) mkdirSync(join(pkg, d), { recursive: true });
  writeFileSync(join(pkg, "package.json"), JSON.stringify({ name: "tools" }));
  for (const x of ["a", "b", "c"]) writeFileSync(join(pkg, "extensions", `${x}.ts`), "throw new Error('fixture must never be imported');\n");
  writeFileSync(join(pkg, "skills/review/SKILL.md"), "---\nname: review\ndescription: fixture\n---\n");
  writeFileSync(join(pkg, "prompts/commit.md"), "fixture\n");
  const src = pkg;
  const ALL = ["extensions/a.ts:on", "extensions/b.ts:on", "extensions/c.ts:on"];
  const SKILL_ON = ["skills/review/SKILL.md:on"];
  const PROMPT_ON = ["prompts/commit.md:on"];

  let n = 0;
  // Pi decides trust from a trust.json inside the fixture agentDir:
  // "saved-true" / "saved-false" write a decision; "none" leaves it unresolved (no UI => untrusted).
  async function run({ global, project, trust = "saved-true", base = pkg }) {
    const agentDir = join(root, `case${++n}`, "agent");
    const cwd = join(root, `case${n}`, "proj");
    mkdirSync(agentDir, { recursive: true });
    mkdirSync(cwd, { recursive: true });
    if (global) writeFileSync(join(agentDir, "settings.json"), JSON.stringify(global, null, 2));
    if (project) {
      mkdirSync(join(cwd, ".pi"), { recursive: true });
      writeFileSync(join(cwd, ".pi", "settings.json"), JSON.stringify(project, null, 2));
    }
    const trustStore = new ProjectTrustStore(agentDir);
    if (trust !== "none") trustStore.set(cwd, trust === "saved-true");
    const trusted = await resolveProjectTrusted({ cwd, trustStore, defaultProjectTrust: "ask", projectTrustContext: { hasUI: false } });
    const settingsManager = SettingsManager.create(cwd, agentDir, { projectTrusted: trusted });
    const pm = new DefaultPackageManager({ cwd, agentDir, settingsManager });
    const missing = [];
    const r = await pm.resolve(async (s) => { missing.push(s); return "skip"; });
    const pick = (list) => list.filter((x) => x.path.startsWith(base))
      .map((x) => `${relative(base, x.path)}:${x.enabled ? "on" : "off"}`).sort();
    return { trusted, extensions: pick(r.extensions), skills: pick(r.skills), prompts: pick(r.prompts), themes: pick(r.themes), missing, settingsManager, cwd, agentDir };
  }

  console.log(`# pi ${version}, entry ${entry}, node ${process.version}`);

  await scenario("string entry loads everything", async () => {
    const r = await run({ global: { packages: [src] } });
    eq(r.extensions, ALL);
    eq(r.skills, SKILL_ON);
    eq(r.prompts, PROMPT_ON);
  });

  await scenario("-path excludes one exact path", async () => {
    const r = await run({ global: { packages: [{ source: src, extensions: ["-extensions/b.ts"] }] } });
    eq(r.extensions, ["extensions/a.ts:on", "extensions/b.ts:off", "extensions/c.ts:on"]);
  });

  await scenario("[] disables one type; omitted types keep defaults", async () => {
    const r = await run({ global: { packages: [{ source: src, extensions: [] }] } });
    eq(r.extensions, ["extensions/a.ts:off", "extensions/b.ts:off", "extensions/c.ts:off"]);
    eq(r.skills, SKILL_ON);
    eq(r.prompts, PROMPT_ON);
  });

  await scenario("include glob, then !glob excludes", async () => {
    const r = await run({ global: { packages: [{ source: src, extensions: ["extensions/*.ts", "!extensions/c.ts"] }] } });
    eq(r.extensions, ["extensions/a.ts:on", "extensions/b.ts:on", "extensions/c.ts:off"]);
  });

  await scenario("+path beats !glob; -path beats +path", async () => {
    const r = await run({ global: { packages: [{ source: src, extensions: ["!extensions/*.ts", "+extensions/a.ts", "+extensions/b.ts", "-extensions/b.ts"] }] } });
    eq(r.extensions, ["extensions/a.ts:on", "extensions/b.ts:off", "extensions/c.ts:off"]);
  });

  await scenario("project autoload:false delta overrides named paths and inherits the rest", async () => {
    const r = await run({
      global: { packages: [{ source: src, extensions: ["-extensions/c.ts"] }] },
      project: { packages: [{ source: src, autoload: false, extensions: ["+extensions/c.ts", "-extensions/a.ts"] }] },
    });
    eq(r.trusted, true);
    eq(r.extensions, ["extensions/a.ts:off", "extensions/b.ts:on", "extensions/c.ts:on"], "b is inherited from global");
    eq(r.skills, SKILL_ON, "skills inherited");
    eq(r.prompts, PROMPT_ON, "prompts inherited");
  });

  await scenario("project-only delta drops every unnamed resource", async () => {
    const r = await run({ project: { packages: [{ source: src, autoload: false, extensions: ["+extensions/a.ts"] }] } });
    eq(r.extensions, ["extensions/a.ts:on"]);
    eq(r.skills, [], "skill lost");
    eq(r.prompts, [], "prompt lost");
  });

  await scenario("project object entry without autoload replaces global completely", async () => {
    const r = await run({
      global: { packages: [{ source: src, extensions: ["-extensions/c.ts"] }] },
      project: { packages: [{ source: src, extensions: ["+extensions/c.ts"] }] },
    });
    eq(r.extensions, ALL, "global -c no longer applies");
  });

  await scenario("project string entry replaces global filter", async () => {
    const r = await run({
      global: { packages: [{ source: src, extensions: ["-extensions/c.ts"] }] },
      project: { packages: [src] },
    });
    eq(r.extensions, ALL);
  });

  await scenario("saved untrusted project is ignored; global decides", async () => {
    const r = await run({
      global: { packages: [{ source: src, extensions: ["-extensions/c.ts"] }] },
      project: { packages: [{ source: src, autoload: false, extensions: ["+extensions/c.ts"] }] },
      trust: "saved-false",
    });
    eq(r.trusted, false);
    eq(r.extensions, ["extensions/a.ts:on", "extensions/b.ts:on", "extensions/c.ts:off"]);
  });

  await scenario("offline: missing npm source is skipped without the install hook", async () => {
    const r = await run({ global: { packages: ["npm:@phase0/not-installed@1.0.0"] } });
    eq(r.missing, []);
  });

  await scenario("online: missing npm and git sources reach the install hook", async () => {
    delete process.env.PI_OFFLINE;
    try {
      const r = await run({ global: { packages: ["npm:@phase0/not-installed@1.0.0", "git:github.com/phase0/none@v1"] } });
      eq(r.missing, ["npm:@phase0/not-installed@1.0.0", "git:github.com/phase0/none@v1"]);
    } finally {
      process.env.PI_OFFLINE = "1";
    }
  });

  await scenario("identity ignores npm version and git ref, not symlinks; bare git@ is local", async () => {
    const agentDir = join(root, "identity", "agent");
    mkdirSync(agentDir, { recursive: true });
    const pm = new DefaultPackageManager({ cwd: join(root, "identity"), agentDir, settingsManager: SettingsManager.create(join(root, "identity"), agentDir) });
    const id = (s) => pm.getPackageIdentity(s, "user");
    const link = join(root, "pkgs", "tools-link");
    symlinkSync(pkg, link);
    eq(id("npm:@acme/tools@1.0.0"), id("npm:@acme/tools"));
    eq(id("git:github.com/acme/tools@v1"), "git:github.com/acme/tools");
    eq(id("https://github.com/acme/tools"), "git:github.com/acme/tools");
    assert.notEqual(id(pkg), id(link));
    assertions++;
    assert.match(id("git@github.com:acme/tools.git"), /^local:/);
    assertions++;
  });

  await scenario("query-bearing Git sources share identity and obey first-global/last-project precedence", async () => {
    const agentDir = join(root, "query-identity", "agent");
    mkdirSync(agentDir, { recursive: true });
    const pm = new DefaultPackageManager({ cwd: join(root, "query-identity"), agentDir, settingsManager: SettingsManager.create(join(root, "query-identity"), agentDir) });
    const clean = "https://github.com/acme/tools";
    for (const query of [
      `${clean}?token=dummy-token`,
      "git:https://dummy-user:dummy-pass@github.com/acme/tools?token=dummy-token",
      "git:github.com/acme/tools?token=dummy-token",
    ]) {
      eq(pm.getPackageIdentity(query, "user"), pm.getPackageIdentity(clean, "user"));
      const global = (pkg) => ({ pkg, scope: "user" });
      const project = (pkg) => ({ pkg, scope: "project" });
      eq(pm.dedupePackages([global(query), global(clean)]), [global(query)]);
      eq(pm.dedupePackages([global(clean), global(query)]), [global(clean)]);
      eq(pm.dedupePackages([project(clean), project(query)]), [project(query)]);
      eq(pm.dedupePackages([project(query), project(clean)]), [project(clean)]);
    }
  });

  await scenario("invalid UTF-8 source decodes to a valid later source and owns its global identity", async () => {
    const cwd = join(root, "bad-source-identity");
    const agentDir = join(cwd, "agent");
    mkdirSync(agentDir, { recursive: true });
    const source = join(cwd, "replacement-\uFFFD");
    const first = { source, extensions: ["-extensions/a.ts"] };
    const last = { source, extensions: ["+extensions/a.ts"] };
    const raw = Buffer.from(JSON.stringify({ packages: [first, last] }));
    const offset = raw.indexOf(Buffer.from("\uFFFD"));
    writeFileSync(join(agentDir, "settings.json"), Buffer.concat([raw.subarray(0, offset), Buffer.from([0xff]), raw.subarray(offset + 3)]));
    const settingsManager = SettingsManager.create(cwd, agentDir);
    const pm = new DefaultPackageManager({ cwd, agentDir, settingsManager });
    const entries = settingsManager.getGlobalSettings().packages;
    eq(entries[0].source, source);
    eq(pm.getPackageIdentity(entries[0].source, "user"), pm.getPackageIdentity(entries[1].source, "user"));
    eq(pm.dedupePackages(entries.map((pkg) => ({ pkg, scope: "user" }))), [{ pkg: first, scope: "user" }]);
  });

  await scenario("native write keeps other filters, [] and unknown keys", async () => {
    const entry = { source: src, extensions: ["-extensions/b.ts"], skills: [], prompts: ["!prompts/commit.md"], themes: [], "x-acme": { keep: true, nested: [1] } };
    const r = await run({ global: { "x-top": 1, packages: [entry] } });
    eq(r.skills, ["skills/review/SKILL.md:off"], "skills: [] in effect before write");
    eq(r.prompts, ["prompts/commit.md:off"], "prompt exclusion in effect before write");
    r.settingsManager.setPackages(r.settingsManager.getGlobalSettings().packages.map((p) => ({ ...p, extensions: ["-extensions/a.ts"] })));
    await r.settingsManager.flush();
    eq(JSON.parse(readFileSync(join(r.agentDir, "settings.json"), "utf8")), { "x-top": 1, packages: [{ ...entry, extensions: ["-extensions/a.ts"] }] });
  });

  await scenario("unresolved trust without UI => untrusted; native project write refused, file byte-identical", async () => {
    const r = await run({ project: { packages: [{ source: src, autoload: false, extensions: ["+extensions/a.ts"] }] }, trust: "none" });
    eq(r.trusted, false);
    eq(r.extensions, [], "project ignored");
    const file = join(r.cwd, ".pi", "settings.json");
    const before = readFileSync(file);
    assert.throws(() => r.settingsManager.setProjectPackages([]), /Project is not trusted; refusing to write project settings/);
    assertions++;
    await r.settingsManager.flush();
    eq(Buffer.compare(readFileSync(file), before), 0);
  });

  await scenario("folder without project resources is implicitly trusted by the native API", async () => {
    const cwd = join(root, "trustprobe");
    const agentDir = join(root, "trustprobe-agent");
    mkdirSync(cwd, { recursive: true });
    mkdirSync(agentDir, { recursive: true });
    const trustStore = new ProjectTrustStore(agentDir);
    const opts = { cwd, trustStore, defaultProjectTrust: "ask", projectTrustContext: { hasUI: false } };
    eq(hasTrustRequiringProjectResources(cwd), false);
    eq(await resolveProjectTrusted(opts), true, "no .pi => trusted without asking");
    mkdirSync(join(cwd, ".pi"));
    writeFileSync(join(cwd, ".pi", "settings.json"), "{}");
    eq(hasTrustRequiringProjectResources(cwd), true);
    eq(await resolveProjectTrusted(opts), false, "same folder, now unresolved => untrusted without UI");
  });

  await scenario("settings.json with a comment is rejected (strict JSON)", async () => {
    const agentDir = join(root, "jsonc", "agent");
    mkdirSync(agentDir, { recursive: true });
    writeFileSync(join(agentDir, "settings.json"), '{\n  // comment\n  "packages": []\n}\n');
    const sm = SettingsManager.create(join(root, "jsonc"), agentDir, { projectTrusted: true });
    eq(sm.drainErrors().map((e) => e.scope), ["global"]);
    eq(sm.getGlobalSettings().packages, undefined, "nothing loaded from the invalid file");
  });

  // A package with one extension, a skill and a prompt, and the given "pi" manifest
  // (none when undefined). Returns string-entry and object-entry resolutions.
  let pkgN = 0;
  async function convert(pi, { skills = true, themes = false } = {}) {
    const dir = join(root, "pkgs", `convert${++pkgN}`);
    const dirs = skills ? ["extensions", "skills/review", "prompts"] : ["extensions"];
    for (const d of dirs) mkdirSync(join(dir, d), { recursive: true });
    writeFileSync(join(dir, "package.json"), JSON.stringify(pi === undefined ? { name: "fixture" } : { name: "fixture", pi }));
    writeFileSync(join(dir, "extensions", "a.ts"), "throw new Error('fixture must never be imported');\n");
    if (themes) {
      mkdirSync(join(dir, "themes"), { recursive: true });
      writeFileSync(join(dir, "themes", "dark.json"), JSON.stringify({ name: "dark" }));
    }
    if (skills) {
      writeFileSync(join(dir, "skills/review/SKILL.md"), "---\nname: review\ndescription: fixture\n---\n");
      writeFileSync(join(dir, "prompts/commit.md"), "fixture\n");
    }
    const asString = await run({ global: { packages: [dir] }, base: dir });
    const asObject = await run({ global: { packages: [{ source: dir, extensions: ["-extensions/a.ts"] }] }, base: dir });
    return { asString, asObject };
  }

  await scenario("string -> object loads convention skills/prompts/themes a partial manifest leaves out", async () => {
    const { asString, asObject } = await convert({ extensions: ["extensions/a.ts"] }, { themes: true });
    eq([asString.extensions, asString.skills, asString.prompts, asString.themes], [["extensions/a.ts:on"], [], [], []]);
    eq([asObject.extensions, asObject.skills, asObject.prompts, asObject.themes], [["extensions/a.ts:off"], ["skills/review/SKILL.md:on"], ["prompts/commit.md:on"], ["themes/dark.json:on"]]);
  });

  await scenario("string -> object keeps skills/prompts: no manifest, full manifest, extension-only", async () => {
    for (const [pi, opts] of [[undefined, {}], [{ extensions: ["extensions/a.ts"], skills: [], prompts: ["prompts/commit.md"] }, {}], [{ extensions: ["extensions/a.ts"] }, { skills: false }]]) {
      const { asString, asObject } = await convert(pi, opts);
      eq(asObject.extensions, ["extensions/a.ts:off"]);
      eq([asObject.skills, asObject.prompts], [asString.skills, asString.prompts], JSON.stringify(pi));
    }
  });

  await scenario("a single-file local source ignores filters; any file is listed as an extension", async () => {
    const dir = join(root, "single");
    mkdirSync(dir, { recursive: true });
    writeFileSync(join(dir, "one.ts"), "throw new Error('fixture must never be imported');\n");
    writeFileSync(join(dir, "notes.md"), "fixture\n");
    const r = await run({ global: { packages: [{ source: join(dir, "one.ts"), extensions: ["-one.ts"] }, join(dir, "notes.md")] }, base: dir });
    eq(r.extensions, ["notes.md:on", "one.ts:on"]);
    eq([r.skills, r.prompts], [[], []]);
  });

  await scenario("an unpaired surrogate escape in a rule names no file; U+FFFD only matches U+FFFD", async () => {
    const dir = join(root, "pkgs", "encoding");
    mkdirSync(join(dir, "extensions"), { recursive: true });
    writeFileSync(join(dir, "package.json"), JSON.stringify({ name: "fixture" }));
    for (const x of ["a", "\uFFFD"]) writeFileSync(join(dir, "extensions", `${x}.ts`), "throw new Error('fixture must never be imported');\n");
    const unpaired = { global: { packages: [{ source: dir, extensions: ["*", "!extensions/\ud800.ts", "-extensions/a.ts"] }] }, base: dir };
    eq(JSON.stringify(unpaired.global).includes("\\ud800"), true, "the settings file holds the escape");
    eq((await run(unpaired)).extensions, ["extensions/a.ts:off", "extensions/\uFFFD.ts:on"]);
    const literal = await run({ global: { packages: [{ source: dir, extensions: ["*", "!extensions/\uFFFD.ts"] }] }, base: dir });
    eq(literal.extensions, ["extensions/a.ts:on", "extensions/\uFFFD.ts:off"]);
  });

  // Project editing (Skillshare writes only the project's .pi/settings.json). An
  // inherited global package is overridden the way pi config does it: a delta
  // entry {source, autoload: false, extensions} whose source is the global one,
  // as written when it is not local, or relative to the project's .pi when it is.
  const projectRef = (cwd, abs) => relative(join(cwd, ".pi"), abs) || ".";

  await scenario("override references: verbatim npm/git and native relative local match the global identity", async () => {
    const cwd = join(root, "refs", "proj");
    const agentDir = join(root, "refs", "agent");
    mkdirSync(join(cwd, ".pi"), { recursive: true });
    mkdirSync(agentDir, { recursive: true });
    const pm = new DefaultPackageManager({ cwd, agentDir, settingsManager: SettingsManager.create(cwd, agentDir) });
    const same = (globalSource, projectSource) => eq(pm.getPackageIdentity(projectSource, "project"), pm.getPackageIdentity(globalSource, "user"), `${globalSource} -> ${projectSource}`);
    same("npm:@acme/tools@1.2.3", "npm:@acme/tools@1.2.3");
    same("git:github.com/acme/tools@v1", "git:github.com/acme/tools@v1");
    same("https://github.com/acme/tools.git", "https://github.com/acme/tools.git");
    same(pkg, projectRef(cwd, pkg));
    same(relative(agentDir, pkg), projectRef(cwd, pkg));
    eq(projectRef(cwd, pkg).startsWith(".."), true, "the reference may leave the project");
  });

  await scenario("inherited delta with a relative local reference: one rule, the rest and other resources inherited", async () => {
    const global = { packages: [src] };
    const delta = (cwd) => ({ packages: [{ source: projectRef(cwd, src), autoload: false, extensions: ["-extensions/a.ts"] }] });
    const cwdOf = () => join(root, `case${n + 1}`, "proj");
    const trusted = await run({ global, project: delta(cwdOf()) });
    eq([trusted.trusted, trusted.extensions, trusted.skills, trusted.prompts], [true, ["extensions/a.ts:off", "extensions/b.ts:on", "extensions/c.ts:on"], SKILL_ON, PROMPT_ON]);
    const untrusted = await run({ global, project: delta(cwdOf()), trust: "saved-false" });
    eq([untrusted.trusted, untrusted.extensions, untrusted.skills, untrusted.prompts], [false, ALL, SKILL_ON, PROMPT_ON]);
  });

  await scenario("delta over a string base whose manifest omits skills/prompts/themes keeps them, trusted or not", async () => {
    const dir = join(root, "pkgs", "partial-delta");
    for (const d of ["extensions", "skills/review", "prompts", "themes"]) mkdirSync(join(dir, d), { recursive: true });
    writeFileSync(join(dir, "package.json"), JSON.stringify({ name: "fixture", pi: { extensions: ["extensions/a.ts", "extensions/b.ts"] } }));
    for (const x of ["a", "b"]) writeFileSync(join(dir, "extensions", `${x}.ts`), "throw new Error('fixture must never be imported');\n");
    writeFileSync(join(dir, "skills/review/SKILL.md"), "---\nname: review\ndescription: fixture\n---\n");
    writeFileSync(join(dir, "prompts/commit.md"), "fixture\n");
    writeFileSync(join(dir, "themes/dark.json"), JSON.stringify({ name: "dark" }));
    const others = (r) => [r.skills, r.prompts, r.themes];
    const base = await run({ global: { packages: [dir] }, base: dir });
    const project = { packages: [{ source: dir, autoload: false, extensions: ["-extensions/a.ts"] }] };
    const trusted = await run({ global: { packages: [dir] }, project, base: dir });
    eq(trusted.extensions, ["extensions/a.ts:off", "extensions/b.ts:on"]);
    eq(others(trusted), others(base));
    const untrusted = await run({ global: { packages: [dir] }, project, base: dir, trust: "saved-false" });
    eq([untrusted.extensions, others(untrusted)], [base.extensions, others(base)]);
  });

  await scenario("a delta with no rule left equals no project entry; a replacement uses the project base dir", async () => {
    const global = { packages: [{ source: src, extensions: ["-extensions/c.ts"] }] };
    const none = await run({ global });
    const empty = await run({ global, project: { packages: [{ source: src, autoload: false }] } });
    eq([empty.extensions, empty.skills, empty.prompts], [none.extensions, none.skills, none.prompts]);
    const cwd = join(root, `case${n + 1}`, "proj");
    const replaced = await run({ global, project: { packages: [{ source: projectRef(cwd, src), extensions: ["-extensions/b.ts"] }] } });
    eq(replaced.extensions, ["extensions/a.ts:on", "extensions/b.ts:off", "extensions/c.ts:on"], "global -c no longer applies");
  });

  await scenario("project-only delta: -path and unnamed paths stay unloaded, +path loads, other resources none", async () => {
    const r = await run({ project: { packages: [{ source: src, autoload: false, extensions: ["-extensions/a.ts", "+extensions/b.ts"] }] } });
    eq(r.extensions, ["extensions/a.ts:off", "extensions/b.ts:on"], "c is not loaded at all");
    eq([r.skills, r.prompts], [[], []]);
  });

  await scenario("native project write: creates .pi when missing; a held settings.json.lock refuses it", async () => {
    const cwd = join(root, "projwrite", "proj");
    const agentDir = join(root, "projwrite", "agent");
    mkdirSync(cwd, { recursive: true });
    mkdirSync(agentDir, { recursive: true });
    const sm = SettingsManager.create(cwd, agentDir, { projectTrusted: true });
    sm.setProjectPackages([{ source: "npm:@acme/tools", autoload: false, extensions: ["-extensions/a.ts"] }]);
    await sm.flush();
    const file = join(cwd, ".pi", "settings.json");
    eq(JSON.parse(readFileSync(file, "utf8")).packages[0].autoload, false);
    eq(existsSync(`${file}.lock`), false, "lock released");
    mkdirSync(`${file}.lock`);
    const before = readFileSync(file);
    const held = SettingsManager.create(cwd, agentDir, { projectTrusted: true });
    held.setProjectPackages([]);
    await held.flush();
    eq(held.drainErrors().map((e) => [e.scope, /lock/i.test(String(e.error?.message ?? e.error))]), [["project", true]]);
    eq(Buffer.compare(readFileSync(file), before), 0, "file unchanged while the lock is held");
  });

  await scenario("filtered registrations survive native persistence without rewriting opaque values", async () => {
    for (const local of [false, true]) {
      for (const source of ["npm:@acme/tools@1.2.3", "git:https://example.invalid/acme/tools.git#v1.2.3", src]) {
        const r = await run({ global: { packages: [] }, project: { packages: [] } });
        const pm = new DefaultPackageManager({ cwd: r.cwd, agentDir: r.agentDir, settingsManager: r.settingsManager });
        const scope = local ? "project" : "user";
        const normalized = pm.normalizePackageSourceForSettings(source, scope);
        const rawEntry = `{"source":${JSON.stringify(normalized)},"extensions":["-extensions/a.ts"],"skills":[],"prompts":[],"themes":[],"opaque":{"integer":9007199254740993,"escaped":"\\u0061"}}`;
        const file = local ? join(r.cwd, ".pi", "settings.json") : join(r.agentDir, "settings.json");
        writeFileSync(file, `{"packages":[${rawEntry}]}`);
        const sm = SettingsManager.create(r.cwd, r.agentDir, { projectTrusted: true });
        const native = new DefaultPackageManager({ cwd: r.cwd, agentDir: r.agentDir, settingsManager: sm });
        const before = readFileSync(file);
        // Remote installers are not invoked; this verifies registration persistence,
        // not network install breadth. Local install itself only checks existence.
        if (source === src) await native.installAndPersist(source, { local });
        else eq(native.addSourceToSettings(source, { local }), false);
        await sm.flush();
        eq(sm.drainErrors(), []);
        eq(Buffer.compare(readFileSync(file), before), 0, "existing object is byte-identical after native persistence");
        await native.update(source); // Offline: no fetch or module execution.
        eq(Buffer.compare(readFileSync(file), before), 0, "update retains the configured object");
        eq(native.removeSourceFromSettings(source, { local }), true);
        await sm.flush();
        eq(JSON.parse(readFileSync(file)).packages, []);
      }
    }
  });

  await scenario("a user npm managed-root miss may resolve through a legacy global root", async () => {
    const r = await run({ global: { packages: [] } });
    const native = new DefaultPackageManager({ cwd: r.cwd, agentDir: r.agentDir, settingsManager: r.settingsManager });
    // Substitute only root lookup; never inspect the real global package tree.
    native.getGlobalNpmRoot = () => join(root, "legacy-global");
    native.getPnpmGlobalPackagePath = () => undefined;
    const legacy = join(root, "legacy-global", "@fixture", "legacy");
    mkdirSync(legacy, { recursive: true });
    const parsed = native.parseSource("npm:@fixture/legacy@1.2.3");
    eq(existsSync(native.getManagedNpmInstallPath(parsed, "user")), false);
    eq(native.getNpmInstallPath(parsed, "user"), legacy);
    eq(native.getNpmInstallPath(parsed, "project"), join(r.cwd, ".pi", "npm", "node_modules", "@fixture", "legacy"));
  });

  await scenario("native proper-lockfile reclaims an empty stale settings lock", async () => {
    const r = await run({ global: { packages: [] } });
    const file = join(r.agentDir, "settings.json");
    mkdirSync(`${file}.lock`);
    const old = new Date(Date.now() - 60000);
    utimesSync(`${file}.lock`, old, old);
    r.settingsManager.setPackages(["npm:@acme/tools"]);
    await r.settingsManager.flush();
    eq(r.settingsManager.drainErrors(), []);
    eq(JSON.parse(readFileSync(file)).packages, ["npm:@acme/tools"]);
    eq(existsSync(`${file}.lock`), false);
  });

  console.log(`# ${scenarios} scenarios, ${assertions} assertions passed`);
} finally {
  rmSync(root, { recursive: true, force: true });
}
