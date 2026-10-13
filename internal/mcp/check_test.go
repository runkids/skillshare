package mcp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func checkService(t *testing.T, config string) *Service {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	return &Service{ConfigPath: path, Home: dir, StateDir: filepath.Join(dir, "state"), Platform: "linux"}
}

// offlineCheck resolves every command and host and finds no variables, unless a test says otherwise.
func offlineCheck(names ...string) CheckOptions {
	return CheckOptions{
		Names:      names,
		LookupEnv:  func(string) (string, bool) { return "", false },
		LookPath:   func(name string) (string, error) { return "/usr/bin/" + name, nil },
		LookupHost: func(context.Context, string) ([]string, error) { return []string{"192.0.2.1"}, nil },
	}
}

func findings(t *testing.T, report *CheckReport, name string) []CheckFinding {
	t.Helper()
	for _, server := range report.Servers {
		if server.Name == name {
			return server.Findings
		}
	}
	t.Fatalf("no result for %s in %+v", name, report.Servers)
	return nil
}

// subjectOf is the subject of the first finding of that check, or "" when there is none.
func subjectOf(list []CheckFinding, check string) string {
	for _, f := range list {
		if f.Check == check {
			return f.Subject
		}
	}
	return ""
}

func hasFinding(list []CheckFinding, level, check, target string) bool {
	for _, f := range list {
		if f.Level == level && f.Check == check && f.Target == target {
			return true
		}
	}
	return false
}

func TestCheckMissingEnvIsErrorWithoutValue(t *testing.T) {
	s := checkService(t, "mcp:\n  targets: [claude]\n  servers:\n    gh:\n      url: https://api.example.com/mcp\n      bearerToken: {fromEnv: GH_TOKEN}\n      headers:\n        X-Team: {fromEnv: TEAM_ID}\n")
	opts := offlineCheck()
	opts.LookupEnv = func(name string) (string, bool) {
		if name == "TEAM_ID" {
			return "secret-team", true
		}
		return "", false
	}
	report, err := s.Check(opts)
	if err != nil {
		t.Fatal(err)
	}
	list := findings(t, report, "gh")
	if !hasFinding(list, "error", "env", "") || report.Servers[0].OK {
		t.Fatalf("missing GH_TOKEN must be an error: %+v", list)
	}
	if got := subjectOf(list, "env"); got != "GH_TOKEN" {
		t.Fatalf("subject = %q, want the variable name", got)
	}
	for _, f := range list {
		if strings.Contains(f.Message, "TEAM_ID") || strings.Contains(f.Message, "secret-team") {
			t.Fatalf("a set variable was reported: %+v", f)
		}
	}
}

func TestCheckCommandNotOnPath(t *testing.T) {
	s := checkService(t, "mcp:\n  targets: [claude]\n  servers:\n    local:\n      command: no-such-mcp-binary\n")
	opts := offlineCheck()
	opts.LookPath = func(string) (string, error) { return "", errors.New("not found") }
	report, err := s.Check(opts)
	if err != nil {
		t.Fatal(err)
	}
	if !hasFinding(findings(t, report, "local"), "error", "command", "") {
		t.Fatalf("unresolved command must be an error: %+v", report.Servers)
	}
	if got := subjectOf(findings(t, report, "local"), "command"); got != "no-such-mcp-binary" {
		t.Fatalf("subject = %q, want the command", got)
	}
}

func TestCheckUnresolvedHostIsWarningAndSkippable(t *testing.T) {
	s := checkService(t, "mcp:\n  targets: [claude]\n  servers:\n    docs:\n      url: https://mcp.skillshare.invalid/mcp\n")
	opts := offlineCheck()
	opts.LookupHost = nil // the real resolver: .invalid never resolves
	report, err := s.Check(opts)
	if err != nil {
		t.Fatal(err)
	}
	if !hasFinding(findings(t, report, "docs"), "warning", "dns", "") || !report.Servers[0].OK {
		t.Fatalf("unresolved host must be a warning only: %+v", report.Servers)
	}
	if got := subjectOf(findings(t, report, "docs"), "dns"); got != "mcp.skillshare.invalid" {
		t.Fatalf("subject = %q, want the host", got)
	}
	opts.SkipDNS = true
	opts.LookupHost = func(context.Context, string) ([]string, error) {
		t.Fatal("--no-dns must not look up hosts")
		return nil, nil
	}
	if report, err = s.Check(opts); err != nil {
		t.Fatal(err)
	}
	if hasFinding(findings(t, report, "docs"), "warning", "dns", "") {
		t.Fatal("DNS finding reported with SkipDNS")
	}
}

func TestCheckSyncState(t *testing.T) {
	s := checkService(t, "mcp:\n  targets: [claude, cursor]\n  servers:\n    docs:\n      url: https://example.com/mcp\n")
	report, err := s.Check(offlineCheck())
	if err != nil {
		t.Fatal(err)
	}
	list := findings(t, report, "docs")
	if !hasFinding(list, "warning", "sync", "claude") || report.Summary.Warnings != 2 {
		t.Fatalf("unsynced entries must warn: %+v", list)
	}
	p, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(p.Revision); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Home, ".claude.json"), []byte(`{"mcpServers":{"docs":{"type":"http","url":"https://changed.example/mcp"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if report, err = s.Check(offlineCheck()); err != nil {
		t.Fatal(err)
	}
	list = findings(t, report, "docs")
	if !hasFinding(list, "error", "sync", "claude") || !hasFinding(list, "info", "sync", "cursor") || report.Summary.Errors != 1 {
		t.Fatalf("an edited entry must be a conflict and the other in sync: %+v", list)
	}
}

func TestCheckClientRuleNamesTarget(t *testing.T) {
	s := checkService(t, "mcp:\n  targets: [claude, cursor]\n  servers:\n    workspace:\n      url: https://example.com/mcp\n    docs:\n      url: https://example.com/docs\n")
	report, err := s.Check(offlineCheck())
	if err != nil {
		t.Fatal(err)
	}
	list := findings(t, report, "workspace")
	if !hasFinding(list, "error", "client-rule", "claude") || hasFinding(list, "error", "client-rule", "cursor") {
		t.Fatalf("Claude's reserved name must be refused for claude only: %+v", list)
	}
	if !hasFinding(list, "warning", "sync", "cursor") || hasFinding(list, "warning", "sync", "claude") {
		t.Fatalf("the accepted target must still report its sync state: %+v", list)
	}
	if !hasFinding(findings(t, report, "docs"), "warning", "sync", "claude") {
		t.Fatal("a refused server must not hide the others' sync state")
	}
	if report, err = s.Check(offlineCheck("docs")); err != nil || report.Summary.Errors != 0 {
		t.Fatalf("an unselected refused server must not fail the check: %v %+v", err, report)
	}
}

func TestCheckServerWithoutTargets(t *testing.T) {
	s := checkService(t, "mcp:\n  targets: [claude]\n  servers:\n    parked:\n      url: https://example.com/mcp\n      targets: []\n")
	report, err := s.Check(offlineCheck())
	if err != nil {
		t.Fatal(err)
	}
	list := findings(t, report, "parked")
	if len(list) != 1 || !hasFinding(list, "info", "targets", "") || !report.Servers[0].OK {
		t.Fatalf("a server without targets is kept in Skillshare only: %+v", list)
	}
}

func TestCheckUnknownNameListsKnown(t *testing.T) {
	s := checkService(t, "mcp:\n  targets: [claude]\n  servers:\n    docs:\n      url: https://example.com/mcp\n")
	_, err := s.Check(offlineCheck("missing"))
	if err == nil || !strings.Contains(err.Error(), "docs") {
		t.Fatalf("unknown name must list known servers, got %v", err)
	}
}

// projectCheckService declares docs globally and, under one mcp.projects root, a docs of its
// own and a local server that needs a variable and a command.
func projectCheckService(t *testing.T) (*Service, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "app")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	s := checkService(t, "mcp:\n  targets: [claude]\n  servers:\n    docs:\n      url: https://example.com/mcp\n  projects:\n    "+root+":\n      servers:\n        docs:\n          url: https://example.com/app-docs\n        local:\n          command: no-such-mcp-binary\n          env:\n            API_KEY: {fromEnv: APP_TOKEN}\n")
	return s, root
}

func checkResult(t *testing.T, report *CheckReport, project, name string) CheckServer {
	t.Helper()
	for _, server := range report.Servers {
		if server.Project == project && server.Name == name {
			return server
		}
	}
	t.Fatalf("no result for %s in project %q: %+v", name, project, report.Servers)
	return CheckServer{}
}

func TestCheckProjectServerEnvAndCommand(t *testing.T) {
	s, root := projectCheckService(t)
	opts := offlineCheck()
	opts.LookPath = func(string) (string, error) { return "", errors.New("not found") }
	report, err := s.Check(opts)
	if err != nil {
		t.Fatal(err)
	}
	local := checkResult(t, report, root, "local")
	if local.OK || !hasFinding(local.Findings, "error", "env", "") || !hasFinding(local.Findings, "error", "command", "") {
		t.Fatalf("a project server's variable and command must be checked: %+v", local)
	}
	if len(report.Servers) != 3 || checkResult(t, report, "", "docs").Project != "" {
		t.Fatalf("want the global docs plus both project servers: %+v", report.Servers)
	}
}

func TestCheckProjectSyncStateFromItsRoot(t *testing.T) {
	s, root := projectCheckService(t)
	p, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(p.Revision); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".mcp.json"), []byte(`{"mcpServers":{"docs":{"type":"http","url":"https://changed.example/mcp"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := s.Check(offlineCheck("docs"))
	if err != nil {
		t.Fatal(err)
	}
	if !hasFinding(checkResult(t, report, root, "docs").Findings, "error", "sync", "claude") {
		t.Fatalf("the edited project entry must be a conflict: %+v", report.Servers)
	}
	if !hasFinding(checkResult(t, report, "", "docs").Findings, "info", "sync", "claude") {
		t.Fatalf("the global docs must stay in sync: %+v", report.Servers)
	}
}

func TestCheckNameMatchesGlobalAndProjects(t *testing.T) {
	s, root := projectCheckService(t)
	report, err := s.Check(offlineCheck("docs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Servers) != 2 || report.Servers[0].Project != "" || report.Servers[1].Project != root {
		t.Fatalf("docs must match the global server and the project's: %+v", report.Servers)
	}
	if report, err = s.Check(offlineCheck("local")); err != nil || len(report.Servers) != 1 || report.Servers[0].Project != root {
		t.Fatalf("a project-only name must select that server: %v %+v", err, report)
	}
}

func TestCheckUnknownNameListsProjectServers(t *testing.T) {
	s, _ := projectCheckService(t)
	_, err := s.Check(offlineCheck("missing"))
	if err == nil || !strings.Contains(err.Error(), "known servers: docs, local") {
		t.Fatalf("unknown name must list project servers once each, got %v", err)
	}
}

func TestCheckPin(t *testing.T) {
	cases := []struct {
		name    string
		command string
		args    []string
		level   string // "" means no finding
		subject string
	}{
		{"npx unversioned", "npx", []string{"-y", "@modelcontextprotocol/server-everything"}, "warning", "@modelcontextprotocol/server-everything"},
		{"npx plain", "npx", []string{"mcp-server"}, "warning", "mcp-server"},
		{"npx latest", "npx", []string{"--yes", "mcp-server@latest"}, "warning", "mcp-server"},
		{"npx scoped latest", "npx", []string{"@scope/pkg@latest"}, "warning", "@scope/pkg"},
		{"npx pinned", "npx", []string{"-y", "mcp-server@1.2.3"}, "", ""},
		{"npx scoped pinned", "npx", []string{"-y", "@scope/pkg@1.2.3"}, "", ""},
		{"npx -p unversioned", "npx", []string{"-p", "mcp-server", "mcp-bin"}, "warning", "mcp-server"},
		{"npx -p pinned", "npx", []string{"-p", "mcp-server@2.0.0", "mcp-bin"}, "", ""},
		{"npx repeated -p, later unversioned", "npx", []string{"--package=lodash@4.17.21", "--package=cowsay", "cowsay", "hi"}, "warning", "cowsay"},
		{"npx repeated -p pinned", "npx", []string{"-p", "lodash@4.17.21", "-p", "cowsay@1.6.0", "cowsay"}, "", ""},
		{"npx dist-tag", "npx", []string{"-y", "mcp-server@next"}, "warning", "mcp-server"},
		{"npx range", "npx", []string{"-y", "@scope/pkg@^1.2.3"}, "warning", "@scope/pkg"},
		{"npx prerelease pinned", "npx", []string{"-y", "mcp-server@1.2.3-beta.1"}, "", ""},
		{"npx local path", "npx", []string{"./local-server"}, "", ""},
		{"uvx unversioned", "uvx", []string{"mcp-server-git"}, "warning", "mcp-server-git"},
		{"uvx @latest", "uvx", []string{"mcp-server-git@latest"}, "warning", "mcp-server-git"},
		{"uvx @version", "uvx", []string{"mcp-server-git@1.2"}, "", ""},
		{"uvx --from pinned", "uvx", []string{"--from", "mcp-server-git==1.2", "mcp-git"}, "", ""},
		{"uvx --from range", "uvx", []string{"--from", "mcp-server-git>=1.2", "mcp-git"}, "warning", "mcp-server-git"},
		{"uvx skips option value", "uvx", []string{"--python", "3.12", "mcp-server-git"}, "warning", "mcp-server-git"},
		{"uvx --with unversioned beside pinned --from", "uvx", []string{"--with", "requests", "--from", "mcp-server==1.2", "mcp-server"}, "warning", "requests"},
		{"uvx -w pinned", "uvx", []string{"-w", "requests==2.32.0", "mcp-server-git@1.2"}, "", ""},
		{"uvx --from env marker only", "uvx", []string{"--from", `mcp-server; python_version == "3.12"`, "mcp-server"}, "warning", "mcp-server"},
		{"uvx --from pinned with env marker", "uvx", []string{"--from", `mcp-server==1.2; python_version == "3.12"`, "mcp-server"}, "", ""},
		{"uvx --from wildcard", "uvx", []string{"--from", "mcp-server==1.*", "mcp-server"}, "warning", "mcp-server"},
		{"uvx --from arbitrary equality", "uvx", []string{"--from", "mcp-server===1.2", "mcp-server"}, "", ""},
		{"pipx run unversioned", "pipx", []string{"run", "mcp-server-git"}, "warning", "mcp-server-git"},
		{"pipx run pinned", "pipx", []string{"run", "mcp-server-git==1.2"}, "", ""},
		{"pipx other subcommand", "pipx", []string{"install", "mcp-server-git"}, "", ""},
		{"docker no tag", "docker", []string{"run", "-i", "--rm", "-e", "TOKEN", "ghcr.io/org/server"}, "warning", "ghcr.io/org/server"},
		{"podman latest", "podman", []string{"run", "--rm", "org/server:latest"}, "warning", "org/server"},
		{"docker registry port", "docker", []string{"run", "localhost:5000/server"}, "warning", "localhost:5000/server"},
		{"docker tag only", "docker", []string{"run", "-i", "--rm", "org/server:1.4"}, "info", "org/server"},
		{"docker digest", "docker", []string{"run", "-i", "org/server:1.4@sha256:abc123"}, "", ""},
		{"docker value flag", "docker", []string{"run", "-v", "/a:/b", "org/server"}, "warning", "org/server"},
		{"docker hardened digest", "docker", []string{"run", "-i", "--rm", "--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--pull", "always", "--read-only", "org/server:1.2@sha256:abc123"}, "", ""},
		{"docker hardened unpinned", "docker", []string{"run", "-i", "--rm", "--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--pull", "always", "--read-only", "org/server"}, "warning", "org/server"},
		{"docker --pull always", "podman", []string{"run", "--pull", "always", "org/server:1.4"}, "info", "org/server"},
		{"absolute binary", "/usr/local/bin/mcp-server", []string{"--flag"}, "", ""},
		{"node script", "node", []string{"/opt/server/index.js"}, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := checkPin(Server{Command: tc.command, Args: tc.args})
			if tc.level == "" {
				if len(got) != 0 {
					t.Fatalf("want no finding, got %+v", got)
				}
				return
			}
			if len(got) != 1 || got[0].Level != tc.level || got[0].Check != "pin" || got[0].Subject != tc.subject {
				t.Fatalf("want %s pin finding for %q, got %+v", tc.level, tc.subject, got)
			}
		})
	}
}

func TestCheckPinWarningDoesNotFailServer(t *testing.T) {
	s := checkService(t, "mcp:\n  targets: [claude]\n  servers:\n    local:\n      command: npx\n      args: [-y, mcp-server]\n")
	report, err := s.Check(offlineCheck())
	if err != nil {
		t.Fatal(err)
	}
	if !hasFinding(findings(t, report, "local"), "warning", "pin", "") || !report.Servers[0].OK {
		t.Fatalf("an unpinned package is a warning only: %+v", report.Servers)
	}
}
