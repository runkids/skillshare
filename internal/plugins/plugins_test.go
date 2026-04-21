package plugins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
)

func TestSyncBundleCodexInstallsMarketplaceCacheAndConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	source := filepath.Join(t.TempDir(), "plugins", "demo")
	mkdirAll(t, filepath.Join(source, ".codex-plugin"))
	writeFile(t, filepath.Join(source, ".codex-plugin", "plugin.json"), `{"name":"demo","version":"1.0.0"}`)

	bundle := Bundle{Name: "demo", SourceDir: source, HasCodex: true}
	res, err := SyncBundle(bundle, "", "codex", true)
	if err != nil {
		t.Fatalf("SyncBundle() error = %v", err)
	}
	if !res.Installed {
		t.Fatalf("expected installed result")
	}

	marketplacePlugin := filepath.Join(home, ".agents", "plugins", "demo", ".codex-plugin", "plugin.json")
	if _, err := os.Stat(marketplacePlugin); err != nil {
		t.Fatalf("marketplace plugin missing: %v", err)
	}
	cachePlugin := filepath.Join(home, ".codex", "plugins", "cache", "skillshare", "demo", "local", ".codex-plugin", "plugin.json")
	if _, err := os.Stat(cachePlugin); err != nil {
		t.Fatalf("cache plugin missing: %v", err)
	}
	cfgData, err := os.ReadFile(config.CodexConfigPath())
	if err != nil {
		t.Fatalf("read codex config: %v", err)
	}
	if !strings.Contains(string(cfgData), `[plugins."demo@skillshare"]`) || !strings.Contains(string(cfgData), `enabled = true`) {
		t.Fatalf("codex config missing plugin enablement:\n%s", string(cfgData))
	}
}

func TestSyncBundleClaudeRendersAndInvokesCLI(t *testing.T) {
	home := t.TempDir()
	binDir := filepath.Join(t.TempDir(), "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	source := filepath.Join(t.TempDir(), "plugins", "demo")
	mkdirAll(t, filepath.Join(source, ".claude-plugin"))
	writeFile(t, filepath.Join(source, ".claude-plugin", "plugin.json"), `{"name":"demo","version":"1.0.0"}`)

	logPath := filepath.Join(t.TempDir(), "claude.log")
	mkdirAll(t, binDir)
	writeFile(t, filepath.Join(binDir, "claude"), "#!/bin/sh\necho \"$@\" >> "+logPath+"\n")
	if err := os.Chmod(filepath.Join(binDir, "claude"), 0o755); err != nil {
		t.Fatalf("chmod claude stub: %v", err)
	}

	bundle := Bundle{Name: "demo", SourceDir: source, HasClaude: true}
	if _, err := SyncBundle(bundle, "", "claude", true); err != nil {
		t.Fatalf("SyncBundle() error = %v", err)
	}

	rendered := filepath.Join(config.ClaudeMarketplaceRoot(""), "plugins", "demo", ".claude-plugin", "plugin.json")
	if _, err := os.Stat(rendered); err != nil {
		t.Fatalf("rendered plugin missing: %v", err)
	}
	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read claude log: %v", err)
	}
	logText := string(logData)
	for _, fragment := range []string{"plugin marketplace add", "plugin install demo@skillshare", "plugin enable demo@skillshare"} {
		if !strings.Contains(logText, fragment) {
			t.Fatalf("expected claude invocation containing %q, got:\n%s", fragment, logText)
		}
	}
}

func TestImportMergesExistingBundleAcrossEcosystems(t *testing.T) {
	home := t.TempDir()
	sourceRoot := filepath.Join(t.TempDir(), "plugins")
	t.Setenv("HOME", home)

	claudePlugin := filepath.Join(home, ".claude", "plugins", "demo")
	writeFile(t, filepath.Join(claudePlugin, ".claude-plugin", "plugin.json"), `{"name":"demo","version":"1.0.0"}`)
	writeFile(t, filepath.Join(claudePlugin, "README.md"), "claude")
	if _, err := Import(sourceRoot, "demo", ImportOptions{From: "claude"}); err != nil {
		t.Fatalf("claude Import() error = %v", err)
	}

	codexPlugin := filepath.Join(home, ".agents", "plugins", "demo")
	writeFile(t, filepath.Join(codexPlugin, ".codex-plugin", "plugin.json"), `{"name":"demo","version":"1.0.0"}`)
	writeFile(t, filepath.Join(codexPlugin, "vendor.txt"), "codex")
	bundle, err := Import(sourceRoot, "demo", ImportOptions{From: "codex"})
	if err != nil {
		t.Fatalf("codex Import() error = %v", err)
	}
	if !bundle.HasClaude || !bundle.HasCodex {
		t.Fatalf("expected merged bundle to have both manifests: %+v", bundle)
	}
	if _, err := os.Stat(filepath.Join(sourceRoot, "demo", "README.md")); err != nil {
		t.Fatalf("expected claude file preserved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(sourceRoot, "demo", "vendor.txt")); err != nil {
		t.Fatalf("expected codex file merged: %v", err)
	}
}

func TestImportClaudeUsesInstalledPluginsMetadataAndDetectsAmbiguity(t *testing.T) {
	home := t.TempDir()
	sourceRoot := filepath.Join(t.TempDir(), "plugins")
	t.Setenv("HOME", home)

	alpha := filepath.Join(home, ".claude", "plugins", "cache", "alpha", "demo", "1.0.0")
	beta := filepath.Join(home, ".claude", "plugins", "cache", "beta", "demo", "2.0.0")
	writeFile(t, filepath.Join(alpha, ".claude-plugin", "plugin.json"), `{"name":"demo","version":"1.0.0"}`)
	writeFile(t, filepath.Join(beta, ".claude-plugin", "plugin.json"), `{"name":"demo","version":"2.0.0"}`)
	writeFile(t, config.ClaudeInstalledPluginsPath(), `{
  "plugins": {
    "demo@alpha": [{"installPath": "`+filepath.ToSlash(alpha)+`"}],
    "demo@beta": [{"installPath": "`+filepath.ToSlash(beta)+`"}]
  }
}`)

	if _, err := Import(sourceRoot, "demo", ImportOptions{From: "claude"}); err == nil || !strings.Contains(err.Error(), "use full ref") {
		t.Fatalf("expected ambiguity error, got %v", err)
	}
	bundle, err := Import(sourceRoot, "demo@alpha", ImportOptions{From: "claude"})
	if err != nil {
		t.Fatalf("Import() full ref error = %v", err)
	}
	if bundle.Name != "demo" || !bundle.HasClaude {
		t.Fatalf("unexpected bundle: %+v", bundle)
	}
}

func TestResolveCodexImportPathUsesHashedCacheAndConfiguredRefs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	writeFile(t, config.CodexConfigPath(), `
[plugins."demo@provider-a"]
enabled = true

[plugins."demo@provider-b"]
enabled = false
`)
	providerA := filepath.Join(config.CodexPluginCacheBase(), "provider-a", "demo", "hash-a")
	providerB := filepath.Join(config.CodexPluginCacheBase(), "provider-b", "demo", "hash-b")
	writeFile(t, filepath.Join(providerA, ".codex-plugin", "plugin.json"), `{"name":"demo","version":"1.0.0"}`)
	writeFile(t, filepath.Join(providerB, ".codex-plugin", "plugin.json"), `{"name":"demo","version":"2.0.0"}`)

	if _, err := resolveCodexImportPath("demo"); err == nil || !strings.Contains(err.Error(), "use full ref") {
		t.Fatalf("expected ambiguity error, got %v", err)
	}
	resolved, err := resolveCodexImportPath("demo@provider-a")
	if err != nil {
		t.Fatalf("resolveCodexImportPath() error = %v", err)
	}
	if resolved != providerA {
		t.Fatalf("resolved path = %q, want %q", resolved, providerA)
	}
}

func TestSyncBundleClaudeUsesInstalledMetadataForUpdate(t *testing.T) {
	home := t.TempDir()
	binDir := filepath.Join(t.TempDir(), "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	source := filepath.Join(t.TempDir(), "plugins", "demo")
	mkdirAll(t, filepath.Join(source, ".claude-plugin"))
	writeFile(t, filepath.Join(source, ".claude-plugin", "plugin.json"), `{"name":"demo","version":"1.0.0"}`)
	writeFile(t, config.ClaudeInstalledPluginsPath(), `{
  "plugins": {
    "demo@skillshare": [{"installPath": "/tmp/demo"}]
  }
}`)

	logPath := filepath.Join(t.TempDir(), "claude.log")
	mkdirAll(t, binDir)
	writeFile(t, filepath.Join(binDir, "claude"), "#!/bin/sh\necho \"$@\" >> "+logPath+"\n")
	if err := os.Chmod(filepath.Join(binDir, "claude"), 0o755); err != nil {
		t.Fatalf("chmod claude stub: %v", err)
	}

	bundle := Bundle{Name: "demo", SourceDir: source, HasClaude: true}
	if _, err := SyncBundle(bundle, "", "claude", true); err != nil {
		t.Fatalf("SyncBundle() error = %v", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read claude log: %v", err)
	}
	if !strings.Contains(string(logData), "plugin update demo@skillshare") {
		t.Fatalf("expected update flow, got:\n%s", string(logData))
	}
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	mkdirAll(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
