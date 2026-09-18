package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPiRender(t *testing.T) {
	for _, extension := range []string{"pi-mcp-adapter", "pi-mcp-extension"} {
		t.Run(extension, func(t *testing.T) {
			out, err := Render("pi", Server{URL: "https://example.com/mcp", PiExtension: extension})
			if err != nil {
				t.Fatal(err)
			}
			if out["url"] != "https://example.com/mcp" {
				t.Fatal(out)
			}
			if extension == "pi-mcp-extension" && out["transport"] != "streamable-http" {
				t.Fatal(out)
			}
			if extension == "pi-mcp-adapter" && out["transport"] != nil {
				t.Fatal(out)
			}
		})
	}
	if _, err := Render("pi", Server{Command: "echo"}); err == nil {
		t.Fatal("must choose extension")
	}
	if _, err := Render("pi", Server{URL: "https://example.com", PiExtension: "pi-mcp-extension", BearerToken: &Value{FromEnv: "TOKEN"}}); err == nil {
		t.Fatal("extension cannot interpolate headers")
	}
	out, err := Render("pi", Server{Command: "echo", PiExtension: "pi-mcp-adapter", Env: map[string]Value{"LITERAL": {Literal: "!date"}, "TOKEN": {FromEnv: "TOKEN"}}})
	if err != nil {
		t.Fatal(err)
	}
	env := out["env"].(map[string]string)
	if env["LITERAL"] != "!!date" || env["TOKEN"] != "${TOKEN}" {
		t.Fatal(env)
	}
}

func TestPiSyncScopesAndPreservation(t *testing.T) {
	for _, project := range []bool{false, true} {
		s := testService(t)
		if project {
			s.ProjectRoot = filepath.Join(s.Home, "project")
		}
		path := filepath.Join(s.Home, ".pi", "agent", "mcp.json")
		if project {
			path = filepath.Join(s.ProjectRoot, ".pi", "mcp.json")
		}
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		original := `{"settings":{"maxRetries":3},"mcpServers":{"mine":{"command":"mine"}}}`
		if err := os.WriteFile(path, []byte(original), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(s.ConfigPath, []byte("mcp:\n  targets: [pi]\n  servers:\n    docs:\n      url: https://example.com/mcp\n      piExtension: pi-mcp-extension\n"), 0600); err != nil {
			t.Fatal(err)
		}
		p, err := s.Preview()
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.Apply(p.Revision); err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(path)
		if !strings.Contains(string(data), `"mine"`) || !strings.Contains(string(data), `"maxRetries"`) || !strings.Contains(string(data), `"streamable-http"`) {
			t.Fatal(string(data))
		}
		p, err = s.Preview()
		if err != nil || p.Changes[0].Action != "unchanged" {
			t.Fatalf("%+v %v", p, err)
		}
	}
}

func TestPiImportTransportAndMixedExtensions(t *testing.T) {
	candidates, err := Import("pi", []byte(`{"mcpServers":{"docs":{"transport":"streamable-http","url":"https://example.com/mcp","lifecycle":"eager"}}}`), "")
	if err != nil || len(candidates) != 1 || len(candidates[0].Problems) != 0 || candidates[0].Server.Transport != "streamable-http" {
		t.Fatalf("%+v %v", candidates, err)
	}
	candidates, err = Import("pi", []byte(`{"mcpServers":{"docs":{"transport":"sse","url":"https://example.com/sse"}}}`), "")
	if err != nil || len(candidates[0].Problems) == 0 {
		t.Fatal("legacy SSE silently converted")
	}
	s := testService(t)
	_, _, err = s.render(&Source{Targets: []string{"pi"}, Servers: map[string]Server{
		"a": {Command: "echo", PiExtension: "pi-mcp-adapter"},
		"b": {Command: "echo", PiExtension: "pi-mcp-extension"},
	}})
	if err == nil {
		t.Fatal("mixed extensions must not share a file")
	}
}

func TestPiCredentials(t *testing.T) {
	server := Server{Command: "echo", PiExtension: "pi-mcp-extension", Env: map[string]Value{"TOKEN": {FromEnv: "TOKEN"}, "MODE": {Literal: "dev"}}}
	out, err := Render("pi", server)
	if err != nil {
		t.Fatal(err)
	}
	if out["env"].(map[string]string)["TOKEN"] != "" {
		t.Fatal("must inherit TOKEN instead of writing an unexpanded placeholder")
	}
	server.Env["TOKEN"] = Value{FromEnv: "OTHER"}
	if _, err = Render("pi", server); err == nil {
		t.Fatal("cannot rename inherited variables")
	}
	out, err = Render("pi", Server{URL: "https://example.com/mcp", PiExtension: "pi-mcp-adapter", BearerToken: &Value{FromEnv: "TOKEN"}})
	if err != nil || out["headers"].(map[string]string)["Authorization"] != "Bearer ${TOKEN}" {
		t.Fatalf("%+v %v", out, err)
	}
}

func TestPiDirectoryOverride(t *testing.T) {
	s := testService(t)
	s.ConfigDirs = map[string]string{"pi": filepath.Join(s.Home, "custom-pi")}
	path, err := s.nativePath("pi")
	if err != nil || path != filepath.Join(s.Home, "custom-pi", "mcp.json") {
		t.Fatalf("%s %v", path, err)
	}
	_, _, err = s.render(&Source{Targets: []string{"pi"}, Servers: map[string]Server{"docs": {Command: "echo", PiExtension: "pi-mcp-extension"}}})
	if err == nil {
		t.Fatal("extension ignores the directory override")
	}
	candidates, err := Import("", []byte(`{"transport":"sse","url":"https://example.com/sse"}`), "docs")
	if err != nil || len(candidates[0].Problems) == 0 {
		t.Fatal("pasted SSE must not become streamable HTTP")
	}
}
