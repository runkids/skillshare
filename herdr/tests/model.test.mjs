import { test } from "node:test";
import assert from "node:assert/strict";
import { spawn, spawnSync } from "node:child_process";
import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, readdirSync, existsSync, rmSync, utimesSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { parseGitStatus, sourceLine, driftFrom, targetDrift, buildItems, hookArgs, counts, groupItems, filterItems, sourceGroups, cloneUrl, skillChoices, newerVersion, parseDiscover, parseAudit, parsePreview, diffSides, mergeMetadata, START, ALIVE, SWEEP, SNAPSHOT, parseSnapshot, rowState, previewTargets, splitFrontmatter, mcpServer, targetList, mapLanes, needsBothWays, READ_SKILL, eligibleTargets, syncScopes, overviewAnswer, remoteLine, waitingNames, releaseTag, panelUpdate, updateState, PANEL_UPDATE, toggleArgs } from "../skillshare/mods/skillshare/model.js";
import { VERSION } from "../skillshare/mods/skillshare/version.js";

test("git status reads branch, ahead/behind and files", () => {
  const g = parseGitStatus("top /src\n## main...origin/main [ahead 1, behind 2]\n M a.md\n?? b/SKILL.md\n");
  assert.deepEqual([g.top, g.branch, g.upstream, g.ahead, g.behind, g.files.length], ["/src", "main", "origin/main", 1, 2, 2]);
});

test("a source outside git says so", () => {
  assert.equal(parseGitStatus("norepo\n").repo, false);
});

test("the footer warns on uncommitted changes or commits behind", () => {
  assert.deepEqual(sourceLine({ repo: true, upstream: "origin/main", files: [{}, {}], ahead: 0, behind: 2 }), { text: "Source · 2 uncommitted · ↓2", warn: true });
});

test("drift keeps only what a sync would change", () => {
  const d = driftFrom({ targets: [{ name: "claude", items: [
    { action: "add", name: "x", kind: "skill", is_sync: true },
    { action: "remove", name: "local", kind: "skill", is_sync: false },
  ] }] });
  assert.deepEqual([...d.keys()], ["skills:x"]);
});

test("rows get update, sync and missing states", () => {
  const items = buildItems("global", {
    list: [{ name: "a", relPath: "a" }, { name: "b", relPath: "b" }, { name: "c", relPath: "c" }, { name: "d", relPath: "d", disabled: true }],
    diff: { targets: [{ name: "claude", items: [{ action: "modify", name: "b", kind: "skill", reason: "local copy", is_sync: true }, { action: "add", name: "c", kind: "skill", is_sync: true }] }] },
    check: { skills: [{ name: "a", status: "update_available", version: "abc1234", source: "https://github.com/o/r.git/a" }] },
  });
  assert.deepEqual(items.map((i) => i.state), ["update", "sync", "missing", "off"]);
});

test("MCP servers need a sync when a target change is pending", () => {
  const items = buildItems("global", { mcp: { changes: [{ name: "m", target: "claude", action: "add" }, { name: "m", target: "codex", action: "unchanged" }] } });
  assert.equal(items[0].state, "sync");
});

test("hooks combine declarations and planned changes, including disabled entries", () => {
  const items = buildItems("global", { hooks: { source: { entries: {
    clean: { bindings: { claude: {} } }, disabled: { enabled: false, bindings: {} },
    pending: { bindings: { codex: {} } },
    conflict: { bindings: { claude: {} } }, pruning: { enabled: false, bindings: { claude: {} } },
  } }, plan: { changes: [
    { name: "clean", target: "claude", action: "unchanged" },
    { name: "pending", target: "codex", action: "add" },
    { name: "conflict", target: "claude", action: "conflict", message: "import it or explicitly replace it" },
    { name: "pruning", target: "claude", action: "remove" },
    { name: "removed", target: "codex", action: "remove" },
  ] } } });
  const by = Object.fromEntries(items.map((i) => [i.name, i]));
  assert.deepEqual(items.map((i) => i.state), ["ok", "off", "sync", "sync", "sync", "sync"]);
  assert.equal(by.disabled.src, "no agents");
  assert.equal(by.removed.declared, false);
  assert.equal(by.conflict.reason, "claude: conflict · import it or explicitly replace it");
  assert.equal(by.clean.src, "claude");
  assert.equal(counts(items).hooks, 6);
  assert.deepEqual(Object.values(groupItems(items)).map((g) => g.length), [0, 4, 2]);
  assert.deepEqual(filterItems(items, { scope: "all", kind: "hooks", query: "codex" }).map((i) => i.name), ["pending", "removed"]);
});

test("source groups collect a repository's rows and leave single sources as rows", () => {
  const items = buildItems("global", { list: [
    { name: "feature-radar-ref", source: "https://github.com/runkids/feature-radar/skills/feature-radar-ref" },
    { name: "feature-radar", source: "github.com/runkids/feature-radar/skills/feature-radar" },
    { name: "code-review", repoName: "_mattpocock-skills", relPath: "_mattpocock-skills/code-review" },
    { name: "codebase-design", repoName: "_mattpocock-skills", relPath: "_mattpocock-skills/codebase-design" },
    { name: "archify", source: "github.com/tt-a1i/archify/archify" },
    { name: "mine" },
  ] });
  const { groups, rows } = sourceGroups(items);
  assert.deepEqual(groups.map((g) => [g.label, g.items.map((i) => i.name)]), [
    ["_mattpocock-skills", ["code-review", "codebase-design"]],
    ["runkids/feature-radar", ["feature-radar", "feature-radar-ref"]],
  ]);
  assert.deepEqual(rows.map((i) => i.name), ["archify", "mine"]);
});

test("local rows group under a non-repository source", () => {
  const { groups } = sourceGroups(buildItems("global", { list: [{ name: "a" }, { name: "b" }] }));
  assert.deepEqual(groups.map((g) => [g.label, g.repo]), [["local", false]]);
});

test("hook preview and inventory errors preserve panel read failures", () => {
  const [item] = buildItems("project", { hooks: { source: { entries: { fmt: { bindings: {} } } }, previewError: "preflight failed" } });
  assert.equal(item.state, "unknown");
  assert.equal(rowState(item, false), "sync status unknown");
  const [status] = buildItems("project", { list: [], hooks: { error: "bad hook config" } });
  assert.equal(status.notice, true);
  assert.equal(status.pending, false);
  assert.equal(status.reason, "bad hook config");
  assert.deepEqual(buildItems("project", { list: null, hooks: null }), []);
  assert.deepEqual(buildItems("global", { list: null, hooks: { error: "missing config" } }), []);
  const raw = { list: [], hooks: { source: { entries: {} }, previewError: "parse /fixture/settings.json: invalid JSON" } };
  const [cleanup] = buildItems("global", raw);
  assert.ok(cleanup, "cleanup failure must not become an empty Hooks tab");
  assert.equal(cleanup.notice, true);
  assert.equal(cleanup.pending, false);
  assert.deepEqual(cleanup.lines, [raw.hooks.previewError]);
  assert.equal(groupItems([cleanup]).sync.length, 0);
  assert.equal(counts([cleanup]).hooks, 0);
  assert.deepEqual(buildItems("global", { ...raw, list: null }), []);
});

test("hook notices stay filtered but never inflate named entry or group counts", () => {
  const items = buildItems("project", { list: [], hooks: { source: { entries: { fmt: { bindings: { git: {} } } } },
    plan: { changes: [], warnings: ["Git include is conditional; activation depends on its includeIf condition"] } } });
  assert.deepEqual([counts(items).all, counts(items).project, counts(items).hooks], [1, 1, 1]);
  assert.equal(Object.values(groupItems(items)).flat().length, 1);
  const notices = items.filter((i) => i.notice);
  assert.equal(notices.length, 1);
  assert.equal(notices[0].pending, false);
  assert.deepEqual(counts(notices), { all: 0, project: 0, global: 0, skills: 0, agents: 0, mcp: 0, plugins: 0, hooks: 0 });
  assert.deepEqual(filterItems(items, { scope: "project", kind: "hooks", query: "conditional" }), notices);
  assert.deepEqual(filterItems(items, { scope: "global", kind: "hooks", query: "conditional" }), []);
  const warnings = buildItems("global", { hooks: { plan: { changes: [], warnings: ["conditional include", "Git version too old"] } } });
  const matched = filterItems(warnings, { scope: "all", kind: "hooks", query: "version" });
  assert.equal(matched.length, 1);
  assert.deepEqual(matched[0].lines, ["Git version too old"]);
  const warning = "Git include is conditional; activation depends on its includeIf condition";
  assert.ok(notices[0].lines.includes(warning) && notices[0].reason.includes(warning));
  const warningsOnly = buildItems("global", { list: [], hooks: { source: { entries: {} }, plan: { changes: [], warnings: [warning] } } });
  assert.equal(warningsOnly.length, 1);
  assert.ok(warningsOnly[0].lines.includes(warning));
});

test("hook inactivity preserves its reason independently of pending native writes", () => {
  for (const [action, message, inactiveReason, state] of [
    ["inactive", "git not found; outputs skipped", "", "ok"],
    ["unchanged", "", "Git 2.53 cannot run config hooks; Git 2.54 or later is required", "ok"],
    ["add", "", "include target /fixture/gitconfig is not writable; add manually", "sync"],
  ]) {
    const [item] = buildItems("global", { hooks: { source: { entries: { fmt: { bindings: { git: {} } } } },
      plan: { changes: [{ target: "git", path: "/fixture/gitconfig", name: "fmt", action, message, inactiveReason }] } } });
    assert.equal(item.state, state);
    assert.equal(item.diagnostic, true);
    assert.ok(item.lines[0].includes(inactiveReason || message));
    assert.ok(item.reason.includes(inactiveReason || message));
  }
});

test("root-wide conflicts reach every affected entry while full roots distinguish names", () => {
  const root = "/a/repo", other = "/b/repo", message = "project has its own Skillshare config; manage its hooks there with -p";
  const items = buildItems("global", { hooks: { source: { entries: { fmt: { bindings: {} } }, projects: {
    [root]: { entries: { fmt: { bindings: { claude: {} } }, lint: { bindings: {} } } },
    [other]: { entries: { fmt: { bindings: {} } } },
  } }, plan: { changes: [{ target: "", name: "", path: root, root, action: "conflict", message },
    { target: "claude", name: "retired", path: "/fixture/settings.json", root, action: "remove" }] } } });
  assert.equal(counts(items).hooks, 5);
  assert.equal(new Set(items.map((i) => i.key)).size, 6);
  const affected = items.filter((i) => i.root === root && !i.notice);
  assert.equal(affected.length, 3);
  assert.ok(affected.every((i) => i.state === "sync" && i.reason.includes(message)));
  assert.equal(items.find((i) => i.root === other).state, "ok");
  assert.equal(filterItems(items, { scope: "all", kind: "hooks", query: other }).length, 1);
  const [notice] = items.filter((i) => i.notice);
  assert.equal(notice.pending, false);
  assert.equal(notice.root, root);
  {
    const items = buildItems("global", { hooks: { source: {
      entries: { fmt: { bindings: {} } }, projects: { "/fixture/repo": { entries: { fmt: { bindings: { claude: {} } } } }, "/fixture/owned": { entries: {} } },
    }, plan: { changes: [{ root: "/fixture/repo", name: "fmt", target: "claude", action: "add" },
      { path: "/fixture/owned", root: "/fixture/owned", name: "", target: "", action: "conflict", message: "project has its own Skillshare config; manage its hooks there with -p" }] } } });
    assert.equal(new Set(items.map((i) => i.key)).size, 3);
    assert.equal(items[1].key, "global:hooks:/fixture/repo/fmt");
    assert.equal(items[1].state, "sync");
    assert.equal(items[2].label, "Hook status");
    assert.equal(items[2].reason, "conflict · project has its own Skillshare config; manage its hooks there with -p");
    assert.equal(hookArgs(items[1], "disable"), null);
    assert.equal(hookArgs(items[2], "remove"), null);
  }
});

test("unnamed native includes carry pending scope without counting as hooks", () => {
  for (const [action, pending] of [["add", true], ["update", true], ["remove", true], ["adopt", true], ["release", true], ["unchanged", false], ["conflict", false]]) {
    const [notice] = buildItems("project", { hooks: { source: { entries: {} }, plan: { changes: [
      { target: "git", path: "/fixture/gitconfig", name: "", action, message: "update Skillshare include.path" },
    ] } } });
    assert.equal(notice.notice, true);
    assert.equal(notice.pending, pending);
    assert.equal(notice.scope, "project");
    assert.equal(counts([notice]).all, 0);
    assert.equal(Object.values(groupItems([notice])).flat().length, 0);
  }
});

test("snapshot reads hooks with the chosen scope from a fixture CLI", (t) => {
  const dir = mkdtempSync(join(tmpdir(), "ss-snapshot-"));
  t.after(() => rmSync(dir, { recursive: true, force: true }));
  writeFileSync(join(dir, "skillshare"), `#!/bin/sh
printf '%s\\n' "$*" >>"$HOME/calls"
printf '%s\\n' '{"source":{"entries":{}},"plan":{"changes":[]}}'
`, { mode: 0o755 });
  const env = { ...process.env, HOME: dir, PATH: `${dir}:${process.env.PATH}` };
  for (const key of ["CODEX_HOME", "CLAUDE_CONFIG_DIR", "PI_CODING_AGENT_DIR", "OPENCODE_CONFIG_DIR", "COPILOT_HOME", "GEMINI_CLI_HOME", "XDG_CONFIG_HOME"]) delete env[key];
  for (const flag of ["-g", "-p"]) {
    const result = spawnSync("sh", ["-c", SNAPSHOT, "test", dir, flag], { env, encoding: "utf8" });
    assert.equal(result.status, 0, result.stderr);
    assert.deepEqual(parseSnapshot(result.stdout).hooks.source.entries, {});
    assert.ok(readFileSync(join(dir, "calls"), "utf8").split("\n").includes(`hooks list --json ${flag}`));
  }
});

test("preview failures show unknown entry status and a single scope error without pending sync", () => {
  const error = "parse /fixture/settings.json: invalid JSON";
  const items = buildItems("global", { list: [], hooks: { source: { entries: {
    fmt: { bindings: { claude: {} } }, lint: { bindings: {} }, disabled: { enabled: false, bindings: {} },
  } }, previewError: error } });
  const rows = items.filter((i) => !i.notice), notices = items.filter((i) => i.notice);
  assert.equal(rows.length, 3);
  assert.ok(rows.every((i) => i.state === "unknown" && !i.lines.includes(error)));
  assert.equal(notices.length, 1);
  assert.deepEqual(notices[0].lines, [error]);
  assert.equal(notices[0].pending, false);
  assert.equal(Object.values(groupItems(items)).flat().length, 0);
  assert.equal(counts(items).hooks, 3);
  assert.equal(rowState(rows[0], false), "sync status unknown");
  assert.equal(rowState(rows[2], false), "sync status unknown");
});

test("disabled hook cleanup keeps disabled alongside runtime diagnostics", () => {
  const reason = "Git 2.53.0 cannot run config hooks; Git 2.54 or later is required";
  const [item] = buildItems("global", { hooks: { source: { entries: {
    fmt: { enabled: false, bindings: { git: {} } }, live: { bindings: { git: {} } },
  } }, plan: { changes: [{ target: "git", path: "/fixture/gitconfig", name: "fmt", action: "remove", inactiveReason: reason }] } } });
  assert.equal(item.state, "sync");
  assert.equal(item.disabled, true);
  assert.equal(item.reason, `git: remove · ${reason}`);
});

test("pending hook summary omits unchanged targets but details retain them and inactivity", () => {
  const [item] = buildItems("global", { hooks: { source: { entries: { fmt: { bindings: { claude: {}, codex: {}, git: {} } } } }, plan: { changes: [
    { target: "claude", name: "fmt", action: "unchanged" }, { target: "codex", name: "fmt", action: "add" },
    { target: "git", name: "fmt", action: "unchanged", inactiveReason: "include target not writable; add manually" },
  ] } } });
  assert.equal(item.state, "sync");
  assert.ok(!item.reason.includes("claude: unchanged"));
  assert.ok(item.reason.includes("codex: add"));
  assert.ok(item.reason.includes("add manually"));
  assert.ok(item.lines.includes("claude: unchanged"));
});

test("hook actions use scoped argv without JSON and reject unsupported actions", () => {
  for (const scope of ["global", "project"]) {
    const item = { kind: "hooks", scope, name: "fmt", declared: true, disabled: false, root: "" };
    const flag = scope === "project" ? "-p" : "-g";
    assert.deepEqual(hookArgs(item, "disable"), ["hooks", "disable", "fmt", flag]);
    assert.deepEqual(hookArgs(item, "remove"), ["hooks", "remove", "fmt", flag]);
    assert.equal(hookArgs(item, "enable"), null);
    assert.deepEqual(hookArgs({ ...item, disabled: true }, "enable"), ["hooks", "enable", "fmt", flag]);
    assert.equal(hookArgs({ ...item, disabled: true }, "disable"), null);
    assert.equal(hookArgs({ ...item, declared: false }, "remove"), null);
    assert.equal(hookArgs({ ...item, root: "/fixture" }, "remove"), null);
    assert.equal(hookArgs(item, "replace"), null);
    assert.equal(hookArgs({ ...item, kind: "skills" }, "remove"), null);
  }
});

test("groups split updates, sync work and the rest", () => {
  const g = groupItems([{ state: "ok", label: "b" }, { state: "update", label: "a" }, { state: "missing", label: "c" }]);
  assert.deepEqual([g.update.length, g.sync.length, g.ok.length], [1, 1, 1]);
});

test("the scope filter keeps one scope", () => {
  const items = [{ scope: "project", kind: "skills", name: "a", src: "" }, { scope: "global", kind: "skills", name: "b", src: "" }];
  assert.deepEqual(filterItems(items, { scope: "project", kind: "skills", query: "" }).map((i) => i.name), ["a"]);
});

test("owner/repo becomes a GitHub clone URL", () => {
  assert.deepEqual(cloneUrl("runkids/feature-radar"), { url: "https://github.com/runkids/feature-radar.git", single: false, path: "", branch: "" });
});

test("a path inside a repo names a single skill", () => {
  assert.equal(cloneUrl("github.com/o/r/skills/x").single, true);
});

test("skill folders become pickable names", () => {
  assert.deepEqual(skillChoices([".", "skills/a"]), [{ path: "skills/a", name: "a" }]);
});

test("versions compare numerically", () => {
  assert.equal(newerVersion("0.28.10", "0.28.9"), "0.28.10");
});

test("the row line names the scope only when both show", () => {
  assert.equal(rowState({ state: "missing", scope: "global" }, true), "global · new");
  assert.equal(rowState({ state: "missing", scope: "global" }, false), "new");
});

test("a branch name with dots keeps its upstream", () => {
  const g = parseGitStatus("top /src\n## release-1.2...origin/release-1.2 [behind 1]\n");
  assert.deepEqual([g.branch, g.upstream, g.behind], ["release-1.2", "origin/release-1.2", 1]);
});

test("git output without a branch line is unknown, not a clean repo", () => {
  assert.equal(parseGitStatus(""), null);
});

test("a tree URL names the skill folder inside the repo", () => {
  const c = cloneUrl("https://github.com/o/r/tree/dev/skills/x");
  assert.deepEqual([c.path, c.branch], ["skills/x", "dev"]);
});

test("discovery keeps the clone folder apart from the skill folders", () => {
  assert.deepEqual(parseDiscover("@dir /tmp/x\nskills/a\n"), { dir: "/tmp/x", paths: ["skills/a"] });
});

test("a blocked audit lists its findings with paths inside the repo", () => {
  const json = JSON.stringify({ results: [{ skillName: "a", isBlocked: true, findings: [
    { severity: "MEDIUM", pattern: "p2", file: "/tmp/x/r/skills/a/x.md", line: 3 },
    { severity: "HIGH", pattern: "prompt-injection", file: "/tmp/x/r/skills/a/SKILL.md", line: 14, snippet: "Ignore previous" }] }] });
  const a = parseAudit(`@@ skills/a\n${json}\n`, "/tmp/x");
  assert.deepEqual([a.blocked, a.findings[0].severity, a.findings[0].file], [["a"], "HIGH", "skills/a/SKILL.md"]);
});

test("an audit with no readable result is unknown, not clean", () => {
  assert.equal(parseAudit("@@ skills/a\nunknown flag\n", "/tmp/x").unknown, true);
});

test("the preview reads both sides and a deleted side", () => {
  assert.deepEqual(parsePreview("@@local a.md\none\ntwo\n@@remote a.md\n@@gone\n"), { "a.md": { local: "one\ntwo", remote: null } });
});

test("side by side marks the lines the other side lacks", () => {
  const d = diffSides("a\nb\nc", "a\nB\nc");
  assert.deepEqual([d.local.map((l) => l.mark), d.differ], [[false, true, false], 1]);
});

test("unchanged runs fold around the change", () => {
  const same = Array.from({ length: 10 }, (_, i) => `l${i}`);
  const d = diffSides([...same, "x"].join("\n"), [...same, "y"].join("\n"), 1);
  assert.deepEqual(d.local.map((l) => l.text), ["…", "l9", "x"]);
});

test("metadata merges per entry: a local uninstall sticks, a local edit beats a remote delete", () => {
  const base = '{"version":1,"entries":{"uninstalled-here":{"source":"a"},"deleted-remotely":{"source":"b"}}}';
  const ours = '{"version":1,"entries":{"deleted-remotely":{"source":"b2"}}}';
  const theirs = '{"version":1,"entries":{"uninstalled-here":{"source":"a"}}}';
  assert.deepEqual(JSON.parse(mergeMetadata(base, ours, theirs)).entries, { "deleted-remotely": { source: "b2" } });
});

test("metadata keeps skills installed on both machines, the later install winning a clash", () => {
  const base = '{"version":1,"entries":{"x":{"source":"o","installed_at":"2026-01-01T00:00:00Z"}}}';
  const ours = '{"version":1,"entries":{"x":{"source":"mine","installed_at":"2026-03-01T00:00:00Z"},"a":{"source":"a"}}}';
  const theirs = '{"version":1,"entries":{"x":{"source":"theirs","installed_at":"2026-02-01T00:00:00Z"},"b":{"source":"b"}}}';
  const e = JSON.parse(mergeMetadata(base, ours, theirs)).entries;
  assert.deepEqual([Object.keys(e).sort(), e.x.source], [["a", "b", "x"], "mine"]);
});

test("MCP and plugin rows name no targets", () => {
  const items = buildItems("global", {
    mcp: { changes: [{ name: "github", target: "claude", action: "add" }, { name: "github", target: "codex", action: "update" }] },
    plugins: { packages: { lint: { bindings: { claude: { pending: true } } } } },
  });
  assert.deepEqual(items.map((i) => rowState(i, false)), ["changed", "changed"]);
});

// ---- Detached jobs (START, ALIVE) ----

// Starts a fake `skillshare` (body) the way the panel does, then ends the starting shell's
// process group, as Herdr does when an exec returns. Resolves to the job's temp folder.
// After the test, the job is ended and both temp folders removed.
function startJob(t, body) {
  const bin = mkdtempSync(join(tmpdir(), "ss-bin-"));
  t.after(() => rmSync(bin, { recursive: true, force: true }));
  writeFileSync(join(bin, "skillshare"), `#!/bin/sh\n${body}\n`, { mode: 0o755 });
  const env = { ...process.env, PATH: `${bin}:${process.env.PATH}`, HC_SCOPE: "exec-1" };
  return new Promise((resolve) => {
    const sh = spawn("sh", ["-c", START, "test", tmpdir(), "hello"], { env, detached: true });
    let out = "";
    sh.stdout.on("data", (b) => { out += b; });
    sh.on("exit", () => {
      try { process.kill(-sh.pid, "SIGKILL"); } catch { /* group already gone */ }
      if (out) t.after(() => {
        const pid = jobPid(out);
        if (pid > 0) try { process.kill(-pid, "SIGKILL"); } catch { /* job already gone */ }
        rmSync(out, { recursive: true, force: true });
      });
      resolve(out);
    });
  });
}

const until = async (ok, ms = 5000) => {
  for (const end = Date.now() + ms; Date.now() < end; await new Promise((r) => setTimeout(r, 50))) if (ok()) return true;
  return false;
};
const alive = (d) => spawnSync("sh", ["-c", ALIVE, "test", d]).status === 0;
const jobPid = (d) => Number(existsSync(join(d, "pid")) ? readFileSync(join(d, "pid"), "utf8").trim() : 0);

test("a detached job outlives the process group that started it", async (t) => {
  const d = await startJob(t, 'sleep 1; echo "$@"');
  assert.ok(await until(() => existsSync(join(d, "code"))), "the job never wrote its exit code");
  assert.deepEqual([readFileSync(join(d, "code"), "utf8").trim(), readFileSync(join(d, "out"), "utf8").trim()], ["0", "hello"]);
});

test("a detached job drops the exec's HC_SCOPE, so Herdr does not hang it up", async (t) => {
  const d = await startJob(t, 'echo "${HC_SCOPE-unset}"');
  assert.ok(await until(() => existsSync(join(d, "code"))), "the job never wrote its exit code");
  assert.equal(readFileSync(join(d, "out"), "utf8").trim(), "unset");
});

test("a job killed before it finishes reads as stopped", async (t) => {
  const d = await startJob(t, "sleep 30");
  assert.ok(await until(() => jobPid(d) > 0), "the job never wrote its pid");
  assert.ok(alive(d), "a running job should read as alive");
  process.kill(-jobPid(d), "SIGKILL");
  assert.ok(await until(() => !alive(d)), "a killed job should read as stopped");
});

test("SWEEP removes only old job folders whose job no longer runs", (t) => {
  const tmp = mkdtempSync(join(tmpdir(), "ss-sweep-"));
  const job = spawn("sleep", ["30"]);
  t.after(() => { job.kill("SIGKILL"); rmSync(tmp, { recursive: true, force: true }); });
  const gone = spawnSync("sh", ["-c", "echo $$"], { encoding: "utf8" }).stdout.trim();
  const folder = (name, files, old) => {
    const d = join(tmp, name);
    mkdirSync(d);
    for (const [f, v] of Object.entries(files)) writeFileSync(join(d, f), v);
    if (old) utimesSync(d, new Date(Date.now() - 20 * 60 * 1000), new Date(Date.now() - 20 * 60 * 1000));
  };
  folder("skillshare-panel.done", { pid: gone, code: "0\n" }, true);
  folder("skillshare-panel.killed", { pid: gone }, true);
  folder("skillshare-panel.running", { pid: String(job.pid) }, true);
  folder("skillshare-panel.recent", { pid: gone, code: "0\n" }, false);
  folder("tmp.other", {}, true);
  spawnSync("sh", ["-c", SWEEP], { env: { ...process.env, TMPDIR: tmp } });
  assert.deepEqual(readdirSync(tmp).sort(), ["skillshare-panel.recent", "skillshare-panel.running", "tmp.other"]);
});

test("targetDrift counts what each target still needs, most first", () => {
  const d = targetDrift({ targets: [
    { name: "claude", items: [] },
    { name: "codex", items: [{ action: "add", name: "a", kind: "skill", is_sync: true }, { action: "remove", name: "local", kind: "skill", is_sync: false }] },
  ] });
  assert.deepEqual(d, [{ name: "codex", n: 1 }, { name: "claude", n: 0 }]);
});

test("a row says one short state and leaves the detail to its preview", () => {
  const base = { kind: "skills", scope: "global", src: "github.com/runkids/archify", state: "ok" };
  assert.equal(rowState(base, true), "global · runkids/archify");
  assert.equal(rowState({ ...base, state: "sync", reason: "content changed in 3 files" }, false), "changed");
  assert.equal(rowState({ ...base, kind: "mcp", targets: ["claude", "codex"] }, false), "2 targets");
});

test("a preview lists every target, the waiting ones first", () => {
  const skill = { kind: "skills", state: "sync", targets: ["codex"] };
  assert.deepEqual(previewTargets(skill, ["claude", "codex"]), [{ name: "codex", text: "needs sync", warn: true }, { name: "claude", text: "in sync", warn: false }]);
  const mcp = { kind: "mcp", state: "sync", targets: ["claude", "gemini"], reason: "gemini: create" };
  assert.deepEqual(previewTargets(mcp).map((r) => `${r.name} ${r.text}`), ["gemini create", "claude in sync"]);
});

test("frontmatter becomes rows and the rest is the document", () => {
  const { meta, body } = splitFrontmatter("---\nname: archify\ndescription: >\n  Diagrams\n  as HTML\n---\n# Archify\n");
  assert.deepEqual(meta, [{ key: "name", value: "archify" }, { key: "description", value: "Diagrams as HTML" }]);
  assert.equal(body, "# Archify\n");
});

test("an MCP server's secret values never leave the source file", () => {
  const yaml = "mcp:\n  servers:\n    other:\n      command: x\n    context7:\n      command: npx\n      args:\n        - -y\n        - '@upstash/context7-mcp'\n      env:\n        API_KEY: sk-live-123\n      bearerToken: abc\n";
  const rows = mcpServer(yaml, "context7");
  assert.deepEqual(rows, [{ key: "command", value: "npx" }, { key: "args", value: "-y @upstash/context7-mcp" }, { key: "env", value: "API_KEY = ••••••••" }, { key: "bearerToken", value: "••••••••" }]);
  assert.equal(JSON.stringify(rows).includes("sk-live"), false);
});

test("the map lists configured targets diff leaves out, in the shown scope only", () => {
  const raw = {
    global: { diff: { targets: [{ name: "claude", items: [{ is_sync: true }, { is_sync: true }] }, { name: "cursor", items: [] }] }, targets: ["claude", "codex", "cursor"] },
    project: { diff: { targets: [{ name: "claude", items: [{ is_sync: true }] }] }, targets: ["pi"] },
  };
  assert.deepEqual(targetList(raw).list, [{ name: "claude", n: 3 }, { name: "codex", n: 0 }, { name: "cursor", n: 0 }, { name: "pi", n: 0 }]);
  assert.deepEqual(targetList(raw, "project"), { known: true, scopes: ["project"], list: [{ name: "claude", n: 1 }, { name: "pi", n: 0 }] });
  assert.deepEqual(targetList(raw).scopes, ["global", "project"]);
  assert.equal(targetList({ global: { diff: null, targets: ["claude"] } }).known, false);
});

test("the map never draws more lines than it has room for", () => {
  const t = (name, n = 0) => ({ name, n });
  const six = ["a", "b", "c", "d", "e", "f"].map((x) => t(x));
  assert.deepEqual(mapLanes(six).map((r) => r.name), ["a", "b", "c", "d", "e", "f"]);
  assert.deepEqual(mapLanes([...six, t("g")]).map((r) => r.name ?? r.more), ["a", "b", "c", "d", "e", 2]);
  const waiting = ["a", "b", "c", "d", "e", "f", "g"].map((x) => t(x, 1));
  assert.deepEqual(mapLanes(waiting).map((r) => r.name ?? r.more), ["a", "b", "c", "d", "e", 2]);
  const mixed = mapLanes([...waiting, t("x"), t("y")]);
  assert.deepEqual(mixed.map((r) => [r.name ?? r.more, r.wait]), [["a", true], ["b", true], ["c", true], ["d", true], [3, true], [2, false]]);
});

test("a machine with work of its own syncs both ways only when it has an upstream", () => {
  assert.equal(needsBothWays({ files: ["a"], upstream: "origin/main" }), true);
  assert.equal(needsBothWays({ files: [], ahead: 2, upstream: "origin/main" }), true);
  assert.equal(needsBothWays({ files: ["a"], upstream: null }), false);
  assert.equal(needsBothWays({ files: [], ahead: 0, behind: 3, upstream: "origin/main" }), false);
});

test("reading a skill lists its files, then SKILL.md, and fails on a missing folder", (t) => {
  const dir = mkdtempSync(join(tmpdir(), "ss-skill-"));
  t.after(() => rmSync(dir, { recursive: true, force: true }));
  mkdirSync(join(dir, "refs")); mkdirSync(join(dir, ".git"));
  writeFileSync(join(dir, "SKILL.md"), "---\nname: x\n---\nBody\n"); writeFileSync(join(dir, "refs", "a.md"), "a"); writeFileSync(join(dir, ".git", "HEAD"), "x");
  const read = (d) => spawnSync("sh", ["-c", READ_SKILL, "test", d], { encoding: "utf8" });
  assert.equal(read(dir).stdout, "SKILL.md\nrefs/a.md\n@@\n---\nname: x\n---\nBody\n");
  assert.equal(read(join(dir, "none")).status, 3);
});

test("an MCP server declared only under a project root is still found", () => {
  const yaml = "mcp:\n  servers:\n    other:\n      command: x\n  projects:\n    ~/work/app:\n      servers:\n        local:\n          command: node\n          env:\n            KEY: secret-1\n";
  assert.deepEqual(mcpServer(yaml, "local"), [{ key: "command", value: "node" }, { key: "env", value: "KEY = ••••••••" }]);
  assert.equal(mcpServer(yaml, "none"), null);
});

test("a skill new to one target waits only there", () => {
  const one = { diff: { targets: [{ name: "claude", items: [] }, { name: "cursor", items: [{ name: "x", kind: "skill", action: "add", is_sync: true }] }] }, skillTargets: ["claude", "codex", "cursor"] };
  const [item] = buildItems("global", { list: [{ name: "x", kind: "skill" }], diff: one.diff });
  assert.equal(item.state, "missing");
  assert.deepEqual(previewTargets(item, eligibleTargets(item, one)).map((r) => [r.name, r.text]), [["cursor", "needs sync"], ["claude", "in sync"], ["codex", "in sync"]]);
});

test("an agent's targets are the ones that take agents", () => {
  const one = { diff: { targets: [{ name: "claude", items: [] }, { name: "codex", items: [] }] }, skillTargets: ["claude", "codex"], agentTargets: ["claude"] };
  assert.deepEqual(eligibleTargets({ kind: "agents", state: "ok", targets: [] }, one), ["claude"]);
  assert.deepEqual(eligibleTargets({ kind: "agents", state: "sync", targets: ["cursor"] }, one), ["claude", "cursor"]);
  assert.deepEqual(eligibleTargets({ kind: "skills", state: "ok" }, one), ["claude", "codex"]);
  // diff also lists a target that takes agents alone; a skill does not go there
  const agentsOnly = { diff: { targets: [{ name: "claude", items: [] }, { name: "x", items: [] }] }, skillTargets: ["claude"], agentTargets: ["claude", "x"] };
  assert.deepEqual(eligibleTargets({ kind: "skills", state: "ok" }, agentsOnly), ["claude"]);
  assert.deepEqual(eligibleTargets({ kind: "skills", state: "ok" }, { diff: agentsOnly.diff }), ["claude", "x"]);
});

test("Sync runs in every scope with a waiting row or a waiting target", () => {
  const orphan = { targets: [{ name: "claude", items: [{ name: "gone", kind: "skill", action: "remove", is_sync: true }] }] };
  const raw = { global: { diff: { targets: [] } }, project: { diff: orphan } };
  const items = [{ scope: "global", kind: "skills", state: "missing" }, { scope: "project", kind: "skills", state: "ok" }];
  assert.deepEqual(syncScopes(items, raw), ["global", "project"]);
  assert.deepEqual(syncScopes(items, raw, "global"), ["global"]);
  assert.deepEqual(syncScopes([{ scope: "global", kind: "hooks", notice: true, pending: true }], { global: {}, project: {} }), ["global"]);
});

test("with nothing waiting, Sync runs in the shown scopes", () => {
  const raw = { global: { diff: { targets: [] } }, project: { diff: { targets: [] } } };
  assert.deepEqual(syncScopes([{ scope: "global", kind: "skills", state: "ok" }], raw), ["global", "project"]);
  assert.deepEqual(syncScopes([], raw, "project"), ["project"]);
});

test("a waiting MCP server, plugin or hook shows on the targets it waits on", () => {
  const raw = { global: { diff: { targets: [{ name: "claude", items: [] }, { name: "codex", items: [] }] }, targets: ["claude", "codex", "cursor"] } };
  const items = [
    { scope: "global", kind: "mcp", state: "sync", targets: ["claude", "codex"], reason: "pending in codex: create" },
    { scope: "global", kind: "hooks", state: "sync", targets: ["claude"], changes: [{ target: "claude", action: "add" }, { target: "git", action: "add" }] },
    { scope: "global", kind: "plugins", state: "ok", targets: ["cursor"], reason: "" },
  ];
  assert.deepEqual(targetList(raw, "all", items).list, [{ name: "claude", n: 1 }, { name: "codex", n: 1 }, { name: "cursor", n: 0 }]);
  assert.deepEqual(targetList(raw, "project", items).list, []);
});

test("Overview's answer agrees with what waits", () => {
  const lag = [{ name: "claude", n: 2 }, { name: "codex", n: 1 }, { name: "pi", n: 1 }];
  assert.deepEqual(overviewAnswer({ loading: true }), { kind: "loading", headline: "Reading skillshare", sub: "one moment" });
  assert.deepEqual(overviewAnswer({ failed: true, error: "config not found" }), { kind: "failed", headline: "skillshare could not be read", sub: "config not found" });
  assert.deepEqual(overviewAnswer({ pending: 4, lag }), { kind: "waiting", headline: "3 targets need a sync", sub: "4 resources waiting for claude, codex and 1 more" });
  assert.deepEqual(overviewAnswer({ pending: 0, lag: lag.slice(0, 1) }), { kind: "waiting", headline: "1 target needs a sync", sub: "waiting on claude" });
  assert.deepEqual(overviewAnswer({ pending: 1 }), { kind: "waiting", headline: "1 resource to sync", sub: "1 resource waiting" });
  assert.deepEqual(overviewAnswer({ moving: true, pending: 2, lag: lag.slice(0, 2) }), { kind: "moving", headline: "Syncing 2 targets", sub: "claude and codex" });
  assert.deepEqual(overviewAnswer({ total: 88, targets: 6 }), { kind: "ok", headline: "Everything is in sync", sub: "88 resources · 6 targets" });
});

test("a machine card says what its remote and its targets wait for", () => {
  assert.equal(remoteLine({ behind: 2, files: ["a"], ahead: 1 }), "↓2 behind · 1 uncommitted · ↑1 to push");
  assert.equal(remoteLine({ files: [] }), "up to date");
  const waits = ["a", "b", "c", "d"].map((name, i) => ({ name, n: i + 1 }));
  assert.equal(waitingNames(waits), "a 1 · b 2 · c 3 · +1 more");
  assert.equal(waitingNames(waits.slice(0, 2)), "a 1 · b 2");
});

test("the latest release link gives its tag, and any other link none", () => {
  assert.deepEqual([releaseTag("https://github.com/runkids/skillshare/releases/tag/v0.31.0"), releaseTag("https://github.com/runkids/skillshare/releases")], ["v0.31.0", null]);
});

test("a panel update is offered only for a newer release", () => {
  assert.deepEqual([panelUpdate("0.30.3", "v0.31.0"), panelUpdate("0.31.0", "v0.31.0"), panelUpdate("0.31.0", "v0.30.3"), panelUpdate("0.31.0", null)], ["v0.31.0", null, null, null]);
});

test("the build an update installed reports it, and an unfinished update goes stale", () => {
  const at = 1000, min = 60 * 1000;
  assert.deepEqual([updateState({ to: "v0.31.0", at }, "0.31.0", at), updateState({ to: "v0.31.0", at }, "0.30.3", at + min), updateState({ to: "v0.31.0", at }, "0.30.3", at + 16 * min), updateState(null, "0.30.3")],
    ["done", null, "stale", null]);
});

test("the panel's version is the release Release Please last wrote", () => {
  const manifest = JSON.parse(readFileSync(new URL("../../.github/release-please-manifest.json", import.meta.url), "utf8"));
  assert.equal(VERSION, manifest["."]);
});

// Runs PANEL_UPDATE from 0.0.1 to a local release v9.9.9 holding `shipped`, over a Herdr home
// whose mods/skillshare holds `installed`. Returns the result, the folder, and a reader.
function panelUpdate999(t, shipped, installed) {
  const root = mkdtempSync(join(tmpdir(), "ss-panel-"));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const repo = join(root, "repo"), dest = join(root, "home/mods/skillshare");
  const put = (dir, files) => { for (const [f, text] of Object.entries(files)) { mkdirSync(join(dir, f, ".."), { recursive: true }); writeFileSync(join(dir, f), text); } };
  put(repo, { "README.md": "repo", ...shipped }); put(dest, installed);
  const git = (...a) => spawnSync("git", ["-C", repo, "-c", "user.name=t", "-c", "user.email=t@example.com", ...a]);
  git("init", "-q"); git("add", "."); git("commit", "-qm", "release"); git("tag", "v9.9.9");
  const r = spawnSync("sh", ["-c", PANEL_UPDATE, "test", "", "v9.9.9", "0.0.1", repo], { env: { ...process.env, HERDR_APP_HOME: join(root, "home"), TMPDIR: root }, encoding: "utf8" });
  return { r, root, dest, read: (f) => readFileSync(join(dest, f), "utf8"), said: r.stderr.trim().split("\n").pop() };
}
const MOD = "herdr/skillshare/mods/skillshare";
const SHIPPED = { [`${MOD}/main.js`]: "new", [`${MOD}/version.js`]: 'VERSION = "9.9.9"', [`${MOD}/icons/a.svg`]: "icon" };
const INSTALLED = { "main.js": "old", "version.js": 'VERSION = "0.0.1"' };

test("PANEL_UPDATE installs the release's panel and keeps the files it replaced", (t) => {
  const { r, root, read } = panelUpdate999(t, SHIPPED, INSTALLED);
  assert.deepEqual([r.status, read("main.js"), read("version.js"), read("icons/a.svg"), read(".bak-update-0.0.1/main.js"), readdirSync(root).sort()],
    [0, "new", 'VERSION = "9.9.9"', "icon", "old", ["home", "repo"]]);
});

test("PANEL_UPDATE keeps the copy an earlier attempt made", (t) => {
  const { r, read } = panelUpdate999(t, SHIPPED, { ...INSTALLED, "main.js": "half", ".bak-update-0.0.1/main.js": "old" });
  assert.deepEqual([r.status, read(".bak-update-0.0.1/main.js")], [0, "old"]);
});

test("PANEL_UPDATE leaves the panel alone when the release has none", (t) => {
  const { r, said, dest } = panelUpdate999(t, {}, INSTALLED);
  assert.deepEqual([r.status, said, readdirSync(dest).sort()], [1, "Release v9.9.9 has no panel update", ["main.js", "version.js"]]);
});

test("PANEL_UPDATE leaves another mod under the panel's name alone", (t) => {
  const { r, said, dest, read } = panelUpdate999(t, SHIPPED, { "main.js": "someone else's" });
  assert.deepEqual([r.status, said, read("main.js")], [1, `The panel is not in ${dest}; update it by hand`, "someone else's"]);
});

test("a collapsed map line keeps the targets it stands for", () => {
  const t = (name, n = 0) => ({ name, n });
  const rows = mapLanes([...["a", "b", "c", "d", "e", "f", "g"].map((x) => t(x, 2)), t("x"), t("y")]);
  assert.deepEqual(rows.filter((r) => r.more).map((r) => r.rest), [[t("e", 2), t("f", 2), t("g", 2)], [t("x"), t("y")]]);
});

test("a skill or agent is disabled and enabled by its own path, in its scope", () => {
  const items = buildItems("project", { list: [
    { name: "react-tips", relPath: "frontend/react-tips", kind: "skill" },
    { name: "reviewer.md", relPath: "reviewer.md", kind: "agent", disabled: true },
  ] });
  assert.deepEqual([toggleArgs(items[0], "disable"), toggleArgs(items[1], "enable")],
    [["disable", "frontend/react-tips", "-p"], ["enable", "reviewer.md", "--kind", "agent", "-p"]]);
});

test("a row offers only the change it can take", () => {
  const [skill] = buildItems("global", { list: [{ name: "archify", relPath: "archify", kind: "skill" }] });
  assert.deepEqual([toggleArgs(skill, "enable"), toggleArgs({ kind: "mcp", name: "m", scope: "global" }, "disable")], [null, null]);
});
