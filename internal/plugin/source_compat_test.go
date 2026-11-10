package plugin

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestManifestFailureIsTargetLocal(t *testing.T) {
	root := fixture(t)
	writeFile(t, root, ".cursor-plugin/plugin.json", "{")
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	c := d.Candidates[0]
	if !slices.Contains(c.Targets, "claude") || slices.Contains(c.Targets, "cursor") {
		t.Fatalf("wrong targets: %+v", c)
	}
}

func TestOpenCodeConventionalEntryWithoutSDK(t *testing.T) {
	root := fixture(t)
	writeFile(t, root, "package.json", `{"name":"demo","main":".opencode/plugins/demo.js"}`)
	writeFile(t, root, ".opencode/plugins/demo.js", "export const Demo = async () => ({})")
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(d.Candidates[0].Targets, "opencode") {
		t.Fatal("OpenCode convention was not detected")
	}
}

func TestOrdinaryNPMPackageIsNotPlugin(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "package.json", `{"name":"ordinary","main":"index.js"}`)
	writeFile(t, root, "index.js", "export default {}")
	if _, err := Discover(context.Background(), root); err == nil {
		t.Fatal("ordinary package accepted")
	}
}

func TestPiConventionalPackage(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "package.json", `{"name":"demo","keywords":["pi-package"]}`)
	writeFile(t, root, "extensions/demo.ts", "export default () => {}")
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(d.Candidates[0].Targets, "pi") {
		t.Fatal("Pi convention was not detected")
	}
}

func TestMissingOpenCodeEntryNamesTheSentenceAndTheFile(t *testing.T) {
	root := fixture(t)
	writeFile(t, root, "package.json", `{"name":"demo","dependencies":{"@opencode-ai/plugin":"1"}}`)
	d, err := DiscoverOptions(context.Background(), root, "", "dist/main.js")
	if err != nil {
		t.Fatal(err)
	}
	info := d.Candidates[0].TargetInfo["opencode"]
	if info.ProblemKey != "plugins.problem.opencodeEntryMissing" || info.ProblemArgs["entry"] != "dist/main.js" || info.Problem == "" {
		t.Fatalf("problem is not translatable: %+v", info)
	}
}

func TestExplicitOpenCodeEntry(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "package.json", `{"name":"custom","main":"index.js"}`)
	writeFile(t, root, "entry.js", "export const plugin = async () => ({})")
	d, err := DiscoverOptions(context.Background(), root, "", "entry.js")
	if err != nil {
		t.Fatal(err)
	}
	if d.Candidates[0].TargetInfo["opencode"].Entry != "entry.js" {
		t.Fatalf("entry ignored: %+v", d)
	}
	if _, err := DiscoverOptions(context.Background(), root, "", "../outside.js"); err == nil {
		t.Fatal("escaped explicit entry")
	}
	if _, err := DiscoverRef(context.Background(), root, "main"); err == nil {
		t.Fatal("local source accepted Git ref")
	}
}

func TestBrokenCatalogDoesNotHideValidManifest(t *testing.T) {
	root := fixture(t)
	writeFile(t, root, ".cursor-plugin/marketplace.json", "{")
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Candidates) != 1 || len(d.Warnings) != 1 {
		t.Fatalf("unexpected discovery: %+v", d)
	}
}

func TestDroidPluginTargetNotSupported(t *testing.T) {
	if slices.Contains(Targets, "droid") {
		t.Fatal("Droid must not be offered")
	}
	root := fixture(t)
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(d.Candidates[0].Targets, "droid") {
		t.Fatal("Droid compatibility must not be advertised")
	}
}

// Codex 0.159 installs from a portable root plugin.json ahead of .codex-plugin/plugin.json, so
// its version is what Codex reports after an update.
func TestCodexPortableManifestPrecedesNative(t *testing.T) {
	root := fixture(t)
	writeFile(t, root, "plugin.json", `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"demo","version":"portable"}`)
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if info := d.Candidates[0].TargetInfo["codex"]; info.Manifest != "plugin.json" || info.Version != "portable" {
		t.Fatalf("Codex manifest: %+v", info)
	}
	writeFile(t, root, "plugin.json", "{")
	d, err = Discover(context.Background(), root)
	if err != nil || !slices.Contains(d.Candidates[0].Targets, "codex") {
		t.Fatalf("broken fallback hid native manifest: %+v %v", d, err)
	}
}

// Codex reports 1.0.0 for a portable manifest without a version; expecting it keeps the
// post-update version check instead of skipping it.
func TestCodexExpectsDefaultVersionOfVersionlessPortableManifest(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "plugin.json", `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"demo"}`)
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if got := d.Candidates[0].TargetInfo["codex"].Version; got != "1.0.0" {
		t.Fatalf("Codex version = %q, want 1.0.0", got)
	}
	if got := d.Candidates[0].TargetInfo["cursor"].Version; got != "" {
		t.Fatalf("cursor version = %q, want none", got)
	}
}

// Codex overlays .codex-plugin/plugin.json on a portable manifest, so its logo still applies.
func TestCodexKeepsNativeLogoUnderPortableManifest(t *testing.T) {
	root := fixture(t)
	writeFile(t, root, ".codex-plugin/plugin.json", `{"name":"demo","version":"1.0.0","skills":"./skills","interface":{"logo":"./logo.png"}}`)
	writeFile(t, root, "logo.png", "png-bytes")
	writeFile(t, root, "plugin.json", `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"demo"}`)
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if got := d.Candidates[0].TargetInfo["codex"].Logo; got != "data:image/png;base64,cG5nLWJ5dGVz" {
		t.Fatalf("native logo lost: %q", got)
	}
}

// Codex 0.166 reads .claude-plugin/plugin.json when it finds no manifest of its own, such as
// mattpocock/skills, which lists each skill. It reads only some fields: agents stay Claude's.
func TestCodexReadsAClaudeOnlyPackage(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".claude-plugin/plugin.json", `{"name":"skills","version":"1.3.2","skills":["./skills/engineering/tdd","./skills/productivity/grill-me"],"agents":["./agents/reviewer.md"]}`)
	writeFile(t, root, "skills/engineering/tdd/SKILL.md", "---\nname: tdd\ndescription: TDD\n---\n")
	writeFile(t, root, "skills/productivity/grill-me/SKILL.md", "---\nname: grill-me\ndescription: Grill\n---\n")
	writeFile(t, root, "agents/reviewer.md", "---\nname: reviewer\n---\n")
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	c := d.Candidates[0]
	codex := c.TargetInfo["codex"]
	if !slices.Contains(c.Targets, "codex") || codex.Manifest != ".claude-plugin/plugin.json" || codex.Version != "1.3.2" || !slices.Equal(codex.Components, []string{"skills"}) {
		t.Fatalf("Codex package: %+v", codex)
	}
	if claude := c.TargetInfo["claude"]; !slices.Contains(c.Targets, "claude") || !slices.Equal(claude.Components, []string{"agents", "skills"}) {
		t.Fatalf("Claude package changed: %+v", claude)
	}
}

// A Claude package with nothing Codex reads, such as agents only, is not offered to Codex.
func TestCodexSkipsAClaudePackageWithNothingItReads(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".claude-plugin/plugin.json", `{"name":"rev","agents":["./agents/r.md"]}`)
	writeFile(t, root, "agents/r.md", "---\nname: r\n---\n")
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if c := d.Candidates[0]; slices.Contains(c.Targets, "codex") || !slices.Contains(c.Targets, "claude") {
		t.Fatalf("targets = %v", c.Targets)
	}
}

// Codex stops at the first manifest location that is present, valid or not, so a Claude
// manifest never stands in for a Codex one that is broken, unsupported or linked.
func TestCodexClaudeFallbackKeepsManifestPrecedence(t *testing.T) {
	link := func(target, name string) func(*testing.T, string) {
		return func(t *testing.T, root string) {
			path := filepath.Join(root, name)
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			if err := os.Symlink(target, path); err != nil {
				t.Fatal(err)
			}
		}
	}
	file := func(name, data string) func(*testing.T, string) {
		return func(t *testing.T, root string) { writeFile(t, root, name, data) }
	}
	for _, tc := range []struct {
		name, manifest string
		setup          func(*testing.T, string)
	}{
		{"own manifest", ".codex-plugin/plugin.json", file(".codex-plugin/plugin.json", `{"name":"demo","version":"2.0.0"}`)},
		{"portable manifest", "plugin.json", file("plugin.json", `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"demo"}`)},
		{"unrelated root plugin.json", ".claude-plugin/plugin.json", file("plugin.json", `{"name":"demo"}`)},
		{"unrelated root schema", ".claude-plugin/plugin.json", file("plugin.json", `{"$schema":"https://example.com/custom.schema.json","name":"demo"}`)},
		{"malformed unrelated root", ".claude-plugin/plugin.json", file("plugin.json", "{")},
		{"malformed own manifest", "", file(".codex-plugin/plugin.json", "{")},
		{"own manifest is a folder", "", file(".codex-plugin/plugin.json/x", "")},
		{"unsupported portable schema", "", file("plugin.json", `{"$schema":"https://agent-plugins.org/schemas/2.0.0/plugin.schema.json","name":"demo"}`)},
		{"linked own manifest", "", link("../missing.json", ".codex-plugin/plugin.json")},
		{"linked own folder", "", link("elsewhere", ".codex-plugin")},
		{"linked root plugin.json", "", link(".claude-plugin/plugin.json", "plugin.json")},
		{"linked Claude manifest", "", link("../claude.json", ".claude-plugin/plugin.json")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, root, "skills/demo/SKILL.md", "---\nname: demo\ndescription: Demo\n---\n")
			writeFile(t, root, "claude.json", `{"name":"demo","version":"1.0.0"}`)
			writeFile(t, root, "elsewhere/README.md", "")
			writeFile(t, root, ".claude-plugin/plugin.json", `{"name":"demo","version":"1.0.0"}`)
			tc.setup(t, root)
			d, err := Discover(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			c := d.Candidates[0]
			if !slices.Contains(c.Targets, "claude") {
				t.Fatalf("Claude lost: %+v", c)
			}
			got, offered := c.TargetInfo["codex"], slices.Contains(c.Targets, "codex")
			if offered != (tc.manifest != "") || offered && got.Manifest != tc.manifest {
				t.Fatalf("Codex package = %+v, want manifest %q", got, tc.manifest)
			}
		})
	}
}

func TestCodexLogoComesFromTheManifestInsideThePlugin(t *testing.T) {
	root := fixture(t)
	writeFile(t, root, ".codex-plugin/plugin.json", `{"name":"demo","version":"1.0.0","skills":"./skills","interface":{"logo":"./logo.png"}}`)
	writeFile(t, root, "logo.png", "png-bytes")
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if got := d.Candidates[0].TargetInfo["codex"].Logo; got != "data:image/png;base64,cG5nLWJ5dGVz" {
		t.Fatalf("logo not read: %q", got)
	}
	writeFile(t, root, ".codex-plugin/plugin.json", `{"name":"demo","version":"1.0.0","skills":"./skills","interface":{"logo":"../outside.png"}}`)
	if d, err = Discover(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	if got := d.Candidates[0].TargetInfo["codex"].Logo; got != "" {
		t.Fatalf("logo outside the plugin was read: %q", got)
	}
}
