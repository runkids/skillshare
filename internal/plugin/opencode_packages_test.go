package plugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func openCodePackageService(t *testing.T, version *string) *Service {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	for _, key := range []string{"OPENCODE_CONFIG", "OPENCODE_CONFIG_DIR", "OPENCODE_CONFIG_CONTENT"} {
		t.Setenv(key, "")
	}
	return &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state"), Run: func(_ context.Context, _ string, _ []string, bin string, args ...string) ([]byte, error) {
		if bin != "opencode" || len(args) != 1 || args[0] != "--version" {
			return nil, fmt.Errorf("unexpected command %s %v", bin, args)
		}
		return []byte(*version), nil
	}}
}

func TestOpenCodeDirectoryPackageLifecycle(t *testing.T) {
	version := "2.3.0"
	s := openCodePackageService(t, &version)
	source := t.TempDir()
	manifest := `{"name":"kata-native-agent-hooks","type":"module","exports":"./index.js","dependencies":{"@opencode/plugin":"^2.0.0"},"kata":{"api":"v2"}}`
	writePluginFile(t, source, "package.json", manifest)
	writePluginFile(t, source, "index.js", "export default {}")
	add := Request{Action: "add", Name: "native", Source: source, Targets: []string{"opencode"}, Entry: "index.js"}
	p, err := s.Preview(context.Background(), add)
	if err != nil || p.Blocked {
		t.Fatalf("%+v %v", p, err)
	}
	id := p.Changes[0].ID
	if filepath.Base(openCodeImportPath(id)) != "content" {
		t.Fatalf("not directory: %s", id)
	}
	applyPluginRequest(t, s, add)
	root := openCodeImportPath(id)
	if b, err := os.ReadFile(filepath.Join(root, "package.json")); err != nil || string(b) != manifest {
		t.Fatalf("manifest lost: %s %v", b, err)
	}
	for i := 0; i < 2; i++ {
		p, err = s.Preview(context.Background(), Request{Action: "sync"})
		if err != nil || p.Blocked || len(p.Changes) != 1 || p.Changes[0].Action != "noop" {
			t.Fatalf("not noop: %+v %v", p, err)
		}
		applyPluginRequest(t, s, Request{Action: "sync"})
	}
	writePluginFile(t, source, "index.js", "export default { changed: true }")
	applyPluginRequest(t, s, Request{Action: "update", Name: "native", Targets: []string{"opencode"}})
	if b, err := os.ReadFile(filepath.Join(root, "index.js")); err != nil || !strings.Contains(string(b), "changed") {
		t.Fatalf("not updated: %s %v", b, err)
	}
	applyPluginRequest(t, s, Request{Action: "remove", Name: "native"})
	h := s.host(context.Background(), "opencode")
	if h.Error != "" || len(h.Installed) != 0 {
		t.Fatalf("%+v", h)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("snapshot remains: %v", err)
	}
}

func TestOpenCodeDirectoryImportAndFlatRefusal(t *testing.T) {
	for _, tt := range []struct{ scope, id string }{{"global", ""}, {"project", "./plugins/demo"}, {"account", "../native/demo"}} {
		t.Run(tt.scope, func(t *testing.T) {
			version := "2.3.0"
			s := openCodePackageService(t, &version)
			target := "opencode"
			if tt.scope == "project" {
				s.ProjectRoot = t.TempDir()
			}
			if tt.scope == "account" {
				target = "opencode-work"
				s.Accounts = map[string]Account{target: {Agent: "opencode", Dir: t.TempDir()}}
			}
			file, err := s.openCodeConfigPath(target)
			if err != nil {
				t.Fatal(err)
			}
			source := t.TempDir()
			id := fileURL(source)
			if tt.id != "" {
				id = tt.id
				source = filepath.Clean(filepath.Join(filepath.Dir(file), filepath.FromSlash(id)))
			}
			writePluginFile(t, source, "package.json", `{"name":"demo","exports":"./index.js","dependencies":{"@opencode/plugin":"^2.0.0"}}`)
			writePluginFile(t, source, "index.js", "export default {}")
			writePluginFile(t, filepath.Dir(file), filepath.Base(file), `{"plugins":["`+id+`"]}`)
			matches, err := s.importMatchesRecipe(context.Background(), target, Installed{ID: id}, declaration{Source: source, Plugin: "demo"})
			if err != nil || !matches {
				t.Fatalf("directory identity: %v %v", matches, err)
			}
			applyPluginRequest(t, s, Request{Action: "import", Name: "native", From: target, Plugin: id})
			applyPluginRequest(t, s, Request{Action: "sync"})
			data, err := os.ReadFile(file)
			if err != nil || !strings.Contains(string(data), id) {
				t.Fatalf("registration spelling changed: %s %v", data, err)
			}
			p, err := s.Preview(context.Background(), Request{Action: "update", Name: "native", Targets: []string{target}})
			if err != nil || len(p.Changes) != 1 || p.Changes[0].Action != "skip" {
				t.Fatalf("local update must not install dependencies: %+v %v", p, err)
			}
			applyPluginRequest(t, s, Request{Action: "remove", Name: "native"})
			if _, err := os.Stat(filepath.Join(source, "index.js")); err != nil {
				t.Fatal("import removed original package", err)
			}
			flat := id + "/index.js"
			writePluginFile(t, filepath.Dir(file), filepath.Base(file), `{"plugins":["`+flat+`"]}`)
			p, err = s.Preview(context.Background(), Request{Action: "import", Name: "flat", From: target, Plugin: flat})
			if err != nil || !p.Blocked {
				t.Fatalf("flat import accepted: %+v %v", p, err)
			}
		})
	}
}

func TestOpenCodeVersionChangesInvalidatePreview(t *testing.T) {
	version := "1.18.31"
	s := openCodePackageService(t, &version)
	source := t.TempDir()
	writePluginFile(t, source, "package.json", `{"name":"demo","main":"index.js","dependencies":{"@opencode-ai/plugin":"1"}}`)
	writePluginFile(t, source, "index.js", "export const Demo = async () => ({})")
	add := Request{Action: "add", Source: source, Targets: []string{"opencode"}}
	p, err := s.Preview(context.Background(), add)
	if err != nil || p.Blocked {
		t.Fatalf("%+v %v", p, err)
	}
	if !strings.HasSuffix(p.Changes[0].ID, "/index.js") {
		t.Fatal(p.Changes[0].ID)
	}
	version = "2.3.0"
	if _, err := s.Apply(context.Background(), add, p.Revision); err == nil {
		t.Fatal("runtime change accepted")
	}
}

func TestOpenCodeDirectoryRejectsEscapingEntry(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "entry.js")
	writePluginFile(t, filepath.Dir(outside), filepath.Base(outside), "export default {}")
	writePluginFile(t, root, "package.json", `{"name":"demo","exports":"./index.js"}`)
	if err := os.Symlink(outside, filepath.Join(root, "index.js")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, ok := openCodeImportRoot(fileURL(root), ""); ok {
		t.Fatal("escaping directory import accepted")
	}
}
