package plugin

import (
	"context"
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

func TestNativeManifestPrecedesPortableFallback(t *testing.T) {
	root := fixture(t)
	writeFile(t, root, "plugin.json", `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"demo","version":"old"}`)
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if d.Candidates[0].TargetInfo["codex"].Manifest != ".codex-plugin/plugin.json" {
		t.Fatal("portable fallback hid native manifest")
	}
	writeFile(t, root, "plugin.json", "{")
	d, err = Discover(context.Background(), root)
	if err != nil || !slices.Contains(d.Candidates[0].Targets, "codex") {
		t.Fatalf("broken fallback hid native manifest: %+v %v", d, err)
	}
}
