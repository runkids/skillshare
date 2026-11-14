package plugin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPluginCommandsManifest(t *testing.T) {
	for _, tt := range []struct {
		name, command, path string
		bad                 bool
	}{
		{"valid", "demo-one", "bin/one", false},
		{"basename", "../bad", "bin/one", true},
		{"dot", ".", "bin/one", true},
		{"absolute", "demo-one", "/bin/sh", true},
		{"drive", "demo-one", "C:/bin/sh", true},
		{"backslash", "demo-one", `bin\one`, true},
		{"traversal", "demo-one", "../bin/one", true},
		{"git", "demo-one", ".git/config", true},
		{"missing", "demo-one", "missing", true},
		{"directory", "demo-one", "bin", true},
		{"nonexecutable", "demo-one", "skills/demo/SKILL.md", true},
		{"escape", "demo-one", "escape", true},
		{"internal-link", "demo-one", "inside", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := fixture(t)
			writePluginFile(t, root, "bin/one", "#!/bin/sh\ntrue\n")
			if err := os.Chmod(filepath.Join(root, "bin/one"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("/bin/sh", filepath.Join(root, "escape")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("bin/one", filepath.Join(root, "inside")); err != nil {
				t.Fatal(err)
			}
			manifest := map[string]any{"name": "demo", "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", "commands": []string{"./native"}, "extensions": map[string]any{"cc.runkids.skillshare": map[string]any{"commands": map[string]string{tt.command: tt.path}}, "other.namespace": map[string]any{"anything": true}}}
			raw, _ := json.Marshal(manifest)
			writePluginFile(t, root, "plugin.json", string(raw))
			d, err := Discover(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			c := d.Candidates[0]
			if tt.bad {
				if c.CommandProblem == "" {
					t.Fatalf("unsafe command accepted: %+v", c)
				}
				return
			}
			if c.CommandProblem != "" || c.Commands[tt.command] != tt.path {
				t.Fatalf("commands not discovered: %+v", c)
			}
			if len(c.TargetInfo["codex"].Components) == 0 {
				t.Fatal("native components lost")
			}
		})
	}
	for _, raw := range []string{`{"name":"demo"}`, `{"name":"demo","$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","extensions":{"cc.runkids.skillshare":{"commands":[]}}}`, `{"name":"demo","$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","extensions":{"cc.runkids.skillshare":{"commands":null}}}`} {
		root := fixture(t)
		writePluginFile(t, root, "plugin.json", raw)
		d, err := Discover(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		if raw != `{"name":"demo"}` && d.Candidates[0].CommandProblem == "" {
			t.Fatal("malformed command map accepted")
		}
		if raw == `{"name":"demo"}` && (len(d.Candidates[0].Commands) > 0 || d.Candidates[0].CommandProblem != "") {
			t.Fatal("absent metadata changed")
		}
	}
}

func TestPluginCommandsCatalogPath(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"primary", "alternate"} {
		writePluginFile(t, root, path+"/plugin.json", `{"name":"demo","$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","extensions":{"cc.runkids.skillshare":{"commands":{"demo-one":"bin/one"}}}}`)
		writePluginFile(t, root, path+"/bin/one", "#!/bin/sh\ntrue\n")
		if err := os.Chmod(filepath.Join(root, path, "bin/one"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	writePluginFile(t, root, ".agents/plugins/marketplace.json", `{"name":"primary","plugins":[{"name":"demo","source":"./primary"}]}`)
	writePluginFile(t, root, ".cursor-plugin/marketplace.json", `{"name":"alternate","plugins":[{"name":"demo","source":"./alternate"}]}`)
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Candidates) != 1 || d.Candidates[0].Path != "primary" || d.Candidates[0].Commands["demo-one"] != "bin/one" || d.Candidates[0].pathFor("cursor") != "alternate" {
		t.Fatalf("provider/path lost: %+v", d)
	}
	writePluginFile(t, root, "primary/plugin.json", `{"name":"demo","$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","extensions":{"cc.runkids.skillshare":{"commands":{"demo-one":"../alternate/bin/one"}}}}`)
	d, err = Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if d.Candidates[0].CommandProblem == "" {
		t.Fatal("nested candidate reached sibling executable")
	}
}
