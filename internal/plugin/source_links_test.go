package plugin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func writeFile(t *testing.T, root, name, data string) {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestSourceInternalLinksSurviveSnapshot(t *testing.T) {
	root := fixture(t)
	writeFile(t, root, "CLAUDE.md", "instructions")
	for name, target := range map[string]string{"AGENTS.md": "CLAUDE.md", "instructions": "skills", "nested": "AGENTS.md"} {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "copy")
	if err := copyTree(root, dest); err != nil {
		t.Fatal(err)
	}
	if link, err := os.Readlink(filepath.Join(dest, "AGENTS.md")); err != nil || link != "CLAUDE.md" {
		t.Fatalf("link lost: %s %v", link, err)
	}
	if digest, err := treeDigest(dest); err != nil || digest != d.Digest {
		t.Fatalf("copy digest differs: %s %v", digest, err)
	}
	writeFile(t, root, "CLAUDE.md", "changed")
	if digest, _ := treeDigest(root); digest == d.Digest {
		t.Fatal("target edit did not change digest")
	}
}

func TestSourceLeavesOutUnsafeLinks(t *testing.T) {
	for _, target := range []string{"/etc/passwd", "../outside", "missing", "link", ".git/config", "."} {
		t.Run(target, func(t *testing.T) {
			root := fixture(t)
			writeFile(t, root, ".git/config", "private")
			want, err := treeDigest(root)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(root, "link")); err != nil {
				t.Fatal(err)
			}
			d, err := Discover(context.Background(), root)
			if err != nil || d.Digest != want || len(d.Warnings) != 1 {
				t.Fatalf("unsafe link not left out with a warning: %+v %v", d, err)
			}
			dest := filepath.Join(t.TempDir(), "copy")
			if err := copyTree(root, dest); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(filepath.Join(dest, "link")); !os.IsNotExist(err) {
				t.Fatalf("unsafe link copied: %v", err)
			}
		})
	}
}

func TestDiscoverAllCatalogsAndRelativeURL(t *testing.T) {
	root := fixture(t)
	writeFile(t, root, ".agents/plugins/marketplace.json", `{"name":"codex-market","plugins":[{"name":"demo","source":{"source":"url","url":"./"}}]}`)
	writeFile(t, root, ".claude-plugin/marketplace.json", `{"name":"claude-market","plugins":[{"name":"demo","source":"./"},{"name":"second","source":"./second"}]}`)
	writeFile(t, root, "second/.claude-plugin/plugin.json", `{"name":"second"}`)
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Candidates) != 2 || d.Candidates[0].Problem != "" || len(d.Candidates[0].Targets) != 6 {
		t.Fatalf("catalogs not merged: %+v", d)
	}
}

func TestDiscoverPrefersLocalPathOverExternalCatalogEntry(t *testing.T) {
	root := fixture(t)
	writeFile(t, root, ".agents/plugins/marketplace.json", `{"name":"codex-market","plugins":[{"name":"demo","source":{"source":"url","url":"https://example.com/demo.git"}}]}`)
	writeFile(t, root, ".claude-plugin/marketplace.json", `{"name":"claude-market","plugins":[{"name":"demo","source":"./"}]}`)
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Candidates) != 1 || d.Candidates[0].Path != "." || d.Candidates[0].Problem != "" {
		t.Fatalf("local catalog entry not preferred: %+v", d.Candidates)
	}
}

func TestDiscoverKeepsCatalogWhenOneEntryIsBroken(t *testing.T) {
	root := fixture(t)
	writeFile(t, root, ".cursor-plugin/marketplace.json", `{"name":"market","plugins":[{"name":"demo","source":"./"},{"name":"broken","source":"./broken"}]}`)
	writeFile(t, root, "broken/README.md", "no manifest")
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Candidates) != 2 || d.Candidates[0].Problem != "" || d.Candidates[1].ProblemKey != "plugins.problem.noManifest" {
		t.Fatalf("broken entry not isolated: %+v", d.Candidates)
	}
}

func TestDiscoverClaudeEntryWithoutManifest(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".claude-plugin/marketplace.json", `{"name":"market","plugins":[{"name":"tool","source":"./tool","description":"Tool"}]}`)
	writeFile(t, root, "tool/skills/x/SKILL.md", "---\nname: x\ndescription: X\n---\n")
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	c := d.Candidates[0]
	if c.Problem != "" || !slices.Equal(c.Targets, []string{"claude"}) || !slices.Contains(c.Components, "skills") {
		t.Fatalf("manifest-less Claude entry not usable: %+v", c)
	}
}

func TestDiscoverScopedPackageNameKeepsPi(t *testing.T) {
	root := fixture(t)
	writeFile(t, root, "package.json", `{"name":"@owner/demo","pi":{"skills":["./skills"]}}`)
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if c := d.Candidates[0]; c.Name != "demo" || !slices.Contains(c.Targets, "pi") {
		t.Fatalf("scoped npm name blocked Pi: %+v", c.TargetInfo["pi"])
	}
}

func TestDiscoverUnnamedPackageKeepsPi(t *testing.T) {
	root := fixture(t)
	writeFile(t, root, "package.json", `{"pi":{"skills":["./skills"]}}`)
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if c := d.Candidates[0]; c.Name != "demo" || !slices.Contains(c.Targets, "pi") {
		t.Fatalf("unnamed package.json blocked Pi: %+v", c.TargetInfo["pi"])
	}
}

func TestDiscoverClaudeEntryThatIsOneSkill(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".claude-plugin/marketplace.json", `{"name":"market","plugins":[{"name":"tool","source":"./tool","strict":false}]}`)
	writeFile(t, root, "tool/SKILL.md", "---\nname: tool\ndescription: Tool\n---\n")
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if c := d.Candidates[0]; c.Problem != "" || !slices.Equal(c.Components, []string{"skills"}) {
		t.Fatalf("root SKILL.md not read as the plugin's skill: %+v", c)
	}
}

func TestDiscoverRenamedEntryStaysInstallableInClaude(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".claude-plugin/marketplace.json", `{"name":"market","plugins":[{"name":"foo","source":"./p"}]}`)
	writeFile(t, root, "p/.claude-plugin/plugin.json", `{"name":"bar"}`)
	writeFile(t, root, "p/.codex-plugin/plugin.json", `{"name":"bar","skills":"./skills"}`)
	writeFile(t, root, "p/skills/x/SKILL.md", "---\nname: x\ndescription: X\n---\n")
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	c := d.Candidates[0]
	if c.Problem != "" || c.Name != "foo" || !slices.Contains(c.Targets, "claude") || slices.Contains(c.Targets, "codex") {
		t.Fatalf("renamed entry should install in Claude only: %+v", c)
	}
}

func TestClaudeCatalogEntryCarriesStrictDefinition(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".claude-plugin/marketplace.json", `{"name":"market","plugins":[{"name":"lsp","source":"./lsp","version":"1.0.0","strict":false,"lspServers":{"clangd":{"command":"clangd"}}}]}`)
	writeFile(t, root, "lsp/README.md", "catalog-defined")
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := json.Marshal(pluginEntry(d.Candidates[0], "./content/lsp", "claude"))
	if err != nil {
		t.Fatal(err)
	}
	if d.Candidates[0].Problem != "" || !strings.Contains(string(entry), `"version":"1.0.0"`) || !strings.Contains(string(entry), `"strict":false`) || !strings.Contains(string(entry), `"clangd"`) {
		t.Fatalf("catalog definition not carried into the install catalog: %+v %s", d.Candidates[0], entry)
	}
}

func TestDiscoverUsesEachAgentsOwnCatalogPath(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".agents/plugins/marketplace.json", `{"name":"market","plugins":[{"name":"ctx","source":{"source":"local","path":"./plugins/codex/ctx"}}]}`)
	writeFile(t, root, ".claude-plugin/marketplace.json", `{"name":"market","plugins":[{"name":"ctx","source":"./plugins/claude/ctx"}]}`)
	for _, dir := range []string{"plugins/codex/ctx", "plugins/claude/ctx"} {
		writeFile(t, root, dir+"/.claude-plugin/plugin.json", `{"name":"ctx"}`)
		writeFile(t, root, dir+"/skills/x/SKILL.md", "---\nname: x\ndescription: X\n---\n")
	}
	writeFile(t, root, "plugins/codex/ctx/.codex-plugin/plugin.json", `{"name":"ctx","skills":"./skills"}`)
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	c := d.Candidates[0]
	if len(d.Candidates) != 1 || c.Problem != "" || c.pathFor("codex") != "plugins/codex/ctx" || c.pathFor("claude") != "plugins/claude/ctx" {
		t.Fatalf("per-Agent paths not kept: %+v", d.Candidates)
	}
}

func TestSourceBreaksCrossDirectoryLinkCycle(t *testing.T) {
	root := fixture(t)
	for _, dir := range []string{"a", "b"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("../b", filepath.Join(root, "a", "next")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../a", filepath.Join(root, "b", "next")); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "copy")
	if err := copyTree(root, dest); err != nil {
		t.Fatal(err)
	}
	for _, link := range []string{"a/next", "b/next"} {
		if _, err := os.Lstat(filepath.Join(dest, link)); !os.IsNotExist(err) {
			t.Fatalf("cyclic link %s copied: %v", link, err)
		}
	}
}

func TestSourceLinkParentAfterDirectoryLink(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "nested/deep/keep", "data")
	writeFile(t, root, "nested/target", "data")
	if err := os.Symlink("nested/deep", filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("alias/../target", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	resolved, err := resolveSourcePath(root, filepath.Join(root, "link"))
	if err != nil || resolved != filepath.Join(root, "nested/target") {
		t.Fatalf("wrong link resolution: %s %v", resolved, err)
	}
}
