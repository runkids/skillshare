package mcp_test

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/mcp"
)

const testMCPYAML = `servers:
  fetch:
    command: uvx
    args:
      - mcp-server-fetch
  remote-fs:
    url: https://mcp.example.com/fs
    headers:
      Authorization: Bearer token123
  claude-only:
    command: npx
    args:
      - -y
      - "@modelcontextprotocol/server-everything"
    targets:
      - claude
  disabled-server:
    command: npx
    args:
      - some-server
    disabled: true
`

func TestLoadMCPConfig_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.yaml")
	if err := os.WriteFile(path, []byte(testMCPYAML), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := mcp.LoadMCPConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Servers) != 4 {
		t.Fatalf("expected 4 servers, got %d", len(cfg.Servers))
	}

	fetch, ok := cfg.Servers["fetch"]
	if !ok {
		t.Fatal("expected server 'fetch'")
	}
	if fetch.Command != "uvx" {
		t.Errorf("fetch.Command = %q, want %q", fetch.Command, "uvx")
	}
	if len(fetch.Args) != 1 || fetch.Args[0] != "mcp-server-fetch" {
		t.Errorf("fetch.Args = %v, want [mcp-server-fetch]", fetch.Args)
	}

	remote, ok := cfg.Servers["remote-fs"]
	if !ok {
		t.Fatal("expected server 'remote-fs'")
	}
	if remote.URL != "https://mcp.example.com/fs" {
		t.Errorf("remote-fs.URL = %q", remote.URL)
	}
	if remote.Headers["Authorization"] != "Bearer token123" {
		t.Errorf("remote-fs header Authorization = %q", remote.Headers["Authorization"])
	}

	claudeOnly, ok := cfg.Servers["claude-only"]
	if !ok {
		t.Fatal("expected server 'claude-only'")
	}
	if len(claudeOnly.Targets) != 1 || claudeOnly.Targets[0] != "claude" {
		t.Errorf("claude-only.Targets = %v", claudeOnly.Targets)
	}

	disabled, ok := cfg.Servers["disabled-server"]
	if !ok {
		t.Fatal("expected server 'disabled-server'")
	}
	if !disabled.Disabled {
		t.Error("expected disabled-server.Disabled = true")
	}
}

func TestLoadMCPConfig_FileNotExist(t *testing.T) {
	cfg, err := mcp.LoadMCPConfig("/nonexistent/path/mcp.yaml")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if len(cfg.Servers) != 0 {
		t.Errorf("expected empty servers, got %d", len(cfg.Servers))
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.yaml")

	original := &mcp.MCPConfig{
		Servers: map[string]mcp.MCPServer{
			"my-server": {
				Command: "uvx",
				Args:    []string{"mcp-server-fetch"},
				Env:     map[string]string{"API_KEY": "secret"},
			},
			"my-remote": {
				URL:     "https://mcp.example.com",
				Headers: map[string]string{"X-Token": "abc"},
				Targets: []string{"claude", "cursor"},
			},
		},
	}

	if err := original.Save(path); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	loaded, err := mcp.LoadMCPConfig(path)
	if err != nil {
		t.Fatalf("LoadMCPConfig error: %v", err)
	}

	if len(loaded.Servers) != 2 {
		t.Fatalf("expected 2 servers after round-trip, got %d", len(loaded.Servers))
	}

	s := loaded.Servers["my-server"]
	if s.Command != "uvx" {
		t.Errorf("my-server.Command = %q", s.Command)
	}
	if s.Env["API_KEY"] != "secret" {
		t.Errorf("my-server.Env[API_KEY] = %q", s.Env["API_KEY"])
	}

	r := loaded.Servers["my-remote"]
	if r.URL != "https://mcp.example.com" {
		t.Errorf("my-remote.URL = %q", r.URL)
	}
	if len(r.Targets) != 2 {
		t.Errorf("my-remote.Targets = %v", r.Targets)
	}
}

func TestSave_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "mcp.yaml")

	cfg := &mcp.MCPConfig{Servers: map[string]mcp.MCPServer{}}
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save should create parent dirs: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created: %v", err)
	}
}

func TestSave_HeaderComment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.yaml")

	cfg := &mcp.MCPConfig{Servers: map[string]mcp.MCPServer{}}
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if len(content) < 10 || content[:2] != "# " {
		t.Errorf("expected file to start with header comment, got: %q", content[:min(30, len(content))])
	}
}

func TestIsRemote(t *testing.T) {
	stdio := mcp.MCPServer{Command: "uvx", Args: []string{"mcp-server-fetch"}}
	if stdio.IsRemote() {
		t.Error("stdio server should not be remote")
	}

	remote := mcp.MCPServer{URL: "https://mcp.example.com"}
	if !remote.IsRemote() {
		t.Error("URL server should be remote")
	}
}

func TestServersForTarget(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.yaml")
	if err := os.WriteFile(path, []byte(testMCPYAML), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := mcp.LoadMCPConfig(path)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("claude gets global and claude-only servers", func(t *testing.T) {
		servers := cfg.ServersForTarget("claude")
		// fetch (no targets = all), remote-fs (no targets = all), claude-only (targets=[claude])
		// disabled-server is excluded
		if _, ok := servers["fetch"]; !ok {
			t.Error("expected 'fetch' for claude")
		}
		if _, ok := servers["remote-fs"]; !ok {
			t.Error("expected 'remote-fs' for claude")
		}
		if _, ok := servers["claude-only"]; !ok {
			t.Error("expected 'claude-only' for claude")
		}
		if _, ok := servers["disabled-server"]; ok {
			t.Error("disabled-server should be excluded")
		}
	})

	t.Run("cursor gets global servers but not claude-only", func(t *testing.T) {
		servers := cfg.ServersForTarget("cursor")
		if _, ok := servers["fetch"]; !ok {
			t.Error("expected 'fetch' for cursor")
		}
		if _, ok := servers["claude-only"]; ok {
			t.Error("claude-only should not apply to cursor")
		}
	})

	t.Run("disabled servers are always excluded", func(t *testing.T) {
		servers := cfg.ServersForTarget("claude")
		if _, ok := servers["disabled-server"]; ok {
			t.Error("disabled-server must never appear")
		}
	})
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		servers map[string]mcp.MCPServer
		wantErr bool
	}{
		{
			name:    "empty config is valid",
			servers: map[string]mcp.MCPServer{},
			wantErr: false,
		},
		{
			name: "valid stdio server",
			servers: map[string]mcp.MCPServer{
				"s": {Command: "uvx", Args: []string{"mcp-server-fetch"}},
			},
			wantErr: false,
		},
		{
			name: "valid remote server",
			servers: map[string]mcp.MCPServer{
				"r": {URL: "https://mcp.example.com"},
			},
			wantErr: false,
		},
		{
			name: "no command and no url is invalid",
			servers: map[string]mcp.MCPServer{
				"bad": {Args: []string{"some-arg"}},
			},
			wantErr: true,
		},
		{
			name: "both command and url is invalid",
			servers: map[string]mcp.MCPServer{
				"bad": {Command: "uvx", URL: "https://mcp.example.com"},
			},
			wantErr: true,
		},
		{
			name: "disabled server still validated",
			servers: map[string]mcp.MCPServer{
				"bad": {Disabled: true},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &mcp.MCPConfig{Servers: tt.servers}
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMCPConfigPath(t *testing.T) {
	path := mcp.MCPConfigPath("/home/user/.config/skillshare")
	expected := filepath.Join("/home/user/.config/skillshare", "mcp.yaml")
	if path != expected {
		t.Errorf("MCPConfigPath = %q, want %q", path, expected)
	}
}

func TestProjectMCPConfigPath(t *testing.T) {
	path := mcp.ProjectMCPConfigPath("/home/user/myproject")
	expected := filepath.Join("/home/user/myproject", ".skillshare", "mcp.yaml")
	if path != expected {
		t.Errorf("ProjectMCPConfigPath = %q, want %q", path, expected)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
