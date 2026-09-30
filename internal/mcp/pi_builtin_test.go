package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPiBuiltinRenderAndValidation(t *testing.T) {
	s := Server{URL: "https://example.com/mcp", BearerToken: &Value{FromEnv: "TOKEN"}, PiOptions: map[string]any{"exposure": "deferred", "timeout": 120, "custom": map[string]any{"flag": true}}}
	out, err := Render("pi", s)
	if err != nil || out["transport"] != nil || out["exposure"] != "deferred" || out["headers"].(map[string]string)["Authorization"] != "Bearer ${TOKEN}" {
		t.Fatalf("%+v %v", out, err)
	}
	for _, options := range []map[string]any{{"exposure": "search"}, {"timeout": 0}, {"toolExposure": map[string]any{"get_*": "search"}}, {"cwd": true}, {"settings": map[string]any{}}} {
		s.PiOptions = options
		if err := s.Validate("docs"); err == nil {
			t.Fatalf("invalid options accepted: %+v", options)
		}
	}
	s.PiOptions = nil
	s.Targets = []string{"pi"}
	service := testService(t)
	if got := service.RenderNative("docs.with.dot", s); got[0].Error == "" {
		t.Fatal("Pi built-in rejects dots in names")
	}
	if _, err := Render("pi", Server{Command: "echo", Env: map[string]Value{"MODE": {Literal: "!date"}}}); err == nil {
		t.Fatal("a portable literal must not execute as a Pi secret command")
	}
}

func TestPiBuiltinSyncImportAndScopes(t *testing.T) {
	for _, project := range []bool{false, true} {
		s := testService(t)
		if project {
			s.ProjectRoot = filepath.Join(s.Home, "project")
		}
		if err := os.WriteFile(s.ConfigPath, []byte("mcp:\n  targets: [pi]\n  servers:\n    docs:\n      command: docs\n      piOptions:\n        exposure: deferred\n        timeout: 120\n        custom: {flag: true}\n"), 0600); err != nil {
			t.Fatal(err)
		}
		p, err := s.Preview()
		if err != nil || p.Blocked {
			t.Fatalf("%+v %v", p, err)
		}
		if !strings.HasSuffix(p.Changes[0].Path, string(filepath.Separator)+"mcp.json") {
			t.Fatal(p.Changes)
		}
		if _, err := s.Apply(p.Revision); err != nil {
			t.Fatal(err)
		}
		c, err := s.ImportClient("pi")
		if err != nil || len(c) != 1 || c[0].Server.PiOptions["exposure"] != "deferred" || c[0].Server.PiOptions["custom"] == nil {
			t.Fatalf("%+v %v", c, err)
		}
		p, err = s.Preview()
		if err != nil || p.Changes[0].Action != "unchanged" {
			t.Fatalf("%+v %v", p, err)
		}
	}
}

// Sync always prunes; a config that still opts out with piOptionsPrune: false loads and
// prunes all the same.
func TestPiOptionsPruneOnlyOwnedUnchangedFields(t *testing.T) {
	s := testService(t)
	write := func(options string) {
		t.Helper()
		if err := os.WriteFile(s.ConfigPath, []byte("mcp:\n  targets: [pi]\n  servers:\n    docs:\n      command: docs\n      piOptionsPrune: false\n"+options), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("      piOptions: {exposure: deferred, timeout: 120}\n")
	p, err := s.Preview()
	if err != nil || len(p.Notices) != 1 || !strings.Contains(p.Notices[0], "piOptionsPrune") {
		t.Fatalf("%+v %v", p, err)
	}
	if _, err := s.Apply(p.Revision); err != nil {
		t.Fatal(err)
	}
	path := p.Changes[0].Path
	data, _ := os.ReadFile(path)
	var native map[string]any
	if err := json.Unmarshal(data, &native); err != nil {
		t.Fatal(err)
	}
	entry := native["mcpServers"].(map[string]any)["docs"].(map[string]any)
	entry["custom"] = "manual"
	if err := writeJSONFile(path, native); err != nil {
		t.Fatal(err)
	}
	write("      piOptions: {timeout: 120}\n")
	source, err := LoadSource(s.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	server := source.Servers["docs"]
	server.Targets = []string{"pi"}
	view := s.RenderNative("docs", server)
	if len(view) != 1 || view[0].Error != "" || strings.Contains(view[0].Content, "exposure") || !strings.Contains(view[0].Content, "manual") {
		t.Fatalf("prune preview: %+v", view)
	}
	if data, _ := os.ReadFile(path); !strings.Contains(string(data), "exposure") {
		t.Fatal("preview modified native file")
	}
	p, err = s.Preview()
	if err != nil || p.Blocked || p.Changes[0].Action != "update" {
		t.Fatalf("%+v %v", p, err)
	}
	if _, err := s.Apply(p.Revision); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if strings.Contains(string(data), "exposure") || !strings.Contains(string(data), "manual") {
		t.Fatal(string(data))
	}
	// A field edited directly in Pi must conflict before pruning it.
	if err := json.Unmarshal(data, &native); err != nil {
		t.Fatal(err)
	}
	native["mcpServers"].(map[string]any)["docs"].(map[string]any)["timeout"] = 99
	if err := writeJSONFile(path, native); err != nil {
		t.Fatal(err)
	}
	write("")
	server.PiOptions = nil
	view = s.RenderNative("docs", server)
	if len(view) != 1 || !strings.Contains(view[0].Error, "changed since sync: timeout") || view[0].Content != "" {
		t.Fatalf("conflict preview: %+v", view)
	}
	p, err = s.Preview()
	if err != nil || !p.Blocked {
		t.Fatalf("manual change was silently removed: %+v %v", p, err)
	}
}

func TestPiToolExposureOrderAndCredentialImport(t *testing.T) {
	input := []byte(`{"mcpServers":{"docs":{"command":"docs","exposure":"deferred","toolExposure":{"z*":"hidden","*":"direct"},"oauth":{"clientSecret":"!printf example-secret"},"custom":{"token":"example-token"}}}}`)
	candidates, err := Import("pi", input, "")
	if err != nil || len(candidates) != 1 || len(candidates[0].Problems) > 0 {
		t.Fatalf("%+v %v", candidates, err)
	}
	data, _ := json.Marshal(candidates[0].Server)
	if strings.Contains(string(data), "example-secret") || strings.Contains(string(data), "example-token") {
		t.Fatal("import retained literal credentials")
	}
	if strings.Index(string(data), `"z*"`) > strings.Index(string(data), `"*"`) {
		t.Fatal("import reordered patterns")
	}
	s := testService(t)
	server := candidates[0].Server
	server.Targets = []string{"pi"}
	if _, err := s.Mutate(Mutation{Name: "docs", Server: &server, Replace: true}, "", false); err != nil {
		t.Fatal(err)
	}
	p, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(p.Revision); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(p.Changes[0].Path)
	if strings.Index(string(data), `"z*"`) > strings.Index(string(data), `"*"`) {
		t.Fatal("source round trip reordered patterns")
	}
	// An unrelated native rewrite must keep the wildcard order too.
	server.URL, server.Command = "https://example.com/mcp", ""
	p, err = s.PreviewMutations([]Mutation{{Name: "docs", Server: &server, Replace: true}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Mutate(Mutation{Name: "docs", Server: &server, Replace: true}, p.Revision, true); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(p.Changes[0].Path)
	if strings.Index(string(data), `"z*"`) > strings.Index(string(data), `"*"`) {
		t.Fatal("rewrite reordered patterns")
	}
}

func TestPiBuiltinEnabledAndNativePreview(t *testing.T) {
	s := testService(t)
	server := Server{Command: "docs", Targets: []string{"pi"}, PiOptions: PiOptions{"enabled": false, "exposure": "direct"}}
	if _, err := s.Mutate(Mutation{Name: "docs", Server: &server, Replace: true}, "", true); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.Home, ".pi", "agent", "mcp.json")
	native, _ := os.ReadFile(path)
	var doc map[string]any
	_ = json.Unmarshal(native, &doc)
	entry := doc["mcpServers"].(map[string]any)["docs"].(map[string]any)
	entry["type"] = "stdio"
	entry["oauth"] = map[string]any{"clientSecret": "native-secret"}
	entry["toolExposure"] = map[string]any{"get_*": "direct"}
	if err := writeJSONFile(path, doc); err != nil {
		t.Fatal(err)
	}
	server.PiOptions = nil
	if _, err := s.Mutate(Mutation{Name: "docs", Server: &server, Replace: true}, "", true); err != nil {
		t.Fatal(err)
	}
	p, err := s.Preview()
	if err != nil || p.Blocked || p.Changes[0].Action != "unchanged" {
		t.Fatalf("cleared enabled drifted: %+v %v", p, err)
	}
	server.Command, server.URL = "", "https://example.com/mcp"
	if _, err := s.Mutate(Mutation{Name: "docs", Server: &server, Replace: true}, "", true); err != nil {
		t.Fatal(err)
	}
	native, _ = os.ReadFile(path)
	if strings.Contains(string(native), `"type"`) {
		t.Fatal("stdio type remained on an HTTP server")
	}
	view := s.RenderNative("docs", server)
	if len(view) != 1 || view[0].Error != "" || strings.Contains(view[0].Content, "native-secret") || !strings.Contains(view[0].Content, "<kept in Pi>") || !strings.Contains(view[0].Content, "toolExposure") {
		t.Fatalf("unsafe/incomplete preview: %+v", view)
	}
}

func TestPiBuiltinNativeDisableDoesNotConflict(t *testing.T) {
	s := testService(t)
	server := Server{Command: "docs", Targets: []string{"pi"}}
	if _, err := s.Mutate(Mutation{Name: "docs", Server: &server, Replace: true}, "", true); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.Home, ".pi", "agent", "mcp.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var native map[string]any
	if err := json.Unmarshal(data, &native); err != nil {
		t.Fatal(err)
	}
	native["mcpServers"].(map[string]any)["docs"].(map[string]any)["enabled"] = false
	if err := writeJSONFile(path, native); err != nil {
		t.Fatal(err)
	}
	p, err := s.Preview()
	if err != nil || p.Blocked || p.Changes[0].Action != "unchanged" {
		t.Fatalf("%+v %v", p, err)
	}
	server.PiOptions = PiOptions{"enabled": true}
	p, err = s.PreviewMutation(Mutation{Name: "docs", Server: &server, Replace: true})
	if err != nil || p.Blocked || p.Changes[0].Action != "update" {
		t.Fatalf("%+v %v", p, err)
	}
}
