import { test } from "node:test";
import assert from "node:assert/strict";
import { parseGitStatus, sourceLine, driftFrom, buildItems, groupItems, filterItems, cloneUrl, skillChoices, newerVersion, rowSub, parseDiscover, parseAudit, parsePreview, diffSides, mergeMetadata } from "../skillshare/mods/skillshare/model.js";

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
  assert.equal(rowSub({ state: "missing", scope: "global" }, true), "global · not synced yet");
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
  const subs = items.map((i) => rowSub(i, false));
  assert.deepEqual(subs, ["changed since last sync", "changed since last sync"]);
});
