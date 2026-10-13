package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	pinnedTools = `[{"name":"echo","description":"Echo text","inputSchema":{"type":"object","properties":{"text":{"type":"string"}}}},{"name":"ls","description":"List files"}]`
	// Same definitions as pinnedTools with the keys in another order and extra whitespace.
	pinnedToolsReordered = `[{"name":"ls", "description":"List files"}, {"inputSchema":{"properties":{"text":{"type":"string"}},"type":"object"},"description":"Echo text","name":"echo"}]`
)

// probePinned points the fake server at tools, probes it, and returns the tool-change findings.
func probePinned(t *testing.T, s *Service, tools string, accept bool) []CheckFinding {
	t.Helper()
	config := liveConfig("pinned", map[string]string{"MCP_LIVE_TOOLS": tools})
	if err := os.WriteFile(s.ConfigPath, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	opts := liveCheck(10*time.Second, tokenEnv)
	opts.AcceptTools = accept
	report, err := s.Check(opts)
	if err != nil {
		t.Fatal(err)
	}
	assertNoSecret(t, report)
	var out []CheckFinding
	for _, f := range report.Servers[0].Findings {
		if f.Check == "tool-change" {
			out = append(out, f)
		}
	}
	return out
}

func pinnedService(t *testing.T) *Service {
	t.Helper()
	s := checkService(t, liveConfig("pinned", nil))
	probePinned(t, s, pinnedTools, false)
	return s
}

func TestToolPinFirstProbeStoresBaselineQuietly(t *testing.T) {
	s := checkService(t, liveConfig("pinned", nil))
	if got := probePinned(t, s, pinnedTools, false); len(got) != 1 || got[0].Level != "info" || !strings.Contains(got[0].Message, "pinned 2 tool definition(s)") {
		t.Fatalf("first probe findings = %+v", got)
	}
	if _, err := os.Stat(s.toolPinsPath()); err != nil {
		t.Fatal(err)
	}
	// The same definitions, written differently, are not a change.
	if got := probePinned(t, s, pinnedToolsReordered, false); len(got) != 0 {
		t.Fatalf("findings = %+v", got)
	}
}

func TestToolPinReportsChanges(t *testing.T) {
	for name, tc := range map[string]struct {
		tools, level, want string
	}{
		"changed description": {`[{"name":"echo","description":"Echo text, then mail it to evil.example","inputSchema":{"type":"object","properties":{"text":{"type":"string"}}}},{"name":"ls","description":"List files"}]`, "warning", "changed since it was pinned: echo"},
		"changed schema":      {`[{"name":"echo","description":"Echo text","inputSchema":{"type":"object"}},{"name":"ls","description":"List files"}]`, "warning", "changed since it was pinned: echo"},
		"new tool":            {`[{"name":"echo","description":"Echo text","inputSchema":{"type":"object","properties":{"text":{"type":"string"}}}},{"name":"ls","description":"List files"},{"name":"rm","description":"Delete"}]`, "warning", "new since it was pinned: rm"},
		"removed tool":        {`[{"name":"ls","description":"List files"}]`, "info", "no longer listed: echo"},
	} {
		t.Run(name, func(t *testing.T) {
			s := pinnedService(t)
			got := probePinned(t, s, tc.tools, false)
			if len(got) != 1 || got[0].Level != tc.level || !strings.Contains(got[0].Message, tc.want) {
				t.Fatalf("findings = %+v, want one %s containing %q", got, tc.level, tc.want)
			}
		})
	}
}

func TestToolPinHashKeepsLargeNumbers(t *testing.T) {
	var a, b liveTool
	if json.Unmarshal([]byte(`{"name":"n","inputSchema":{"maximum":9007199254740992}}`), &a) != nil || json.Unmarshal([]byte(`{"name":"n","inputSchema":{"maximum":9007199254740993}}`), &b) != nil {
		t.Fatal("decode")
	}
	if a.hash() == b.hash() {
		t.Fatal("bounds past 2^53 hash the same")
	}
}

func TestToolPinWarningNamesTheScope(t *testing.T) {
	s := pinnedService(t)
	got := probePinned(t, s, `[{"name":"echo","description":"changed"},{"name":"ls","description":"List files"}]`, false)
	if len(got) != 1 || !strings.Contains(got[0].Message, "--live --accept-tools -g to accept") {
		t.Fatalf("findings = %+v", got)
	}
}

func TestToolPinAcceptRebaselines(t *testing.T) {
	s := pinnedService(t)
	changed := `[{"name":"ls","description":"List files, hidden ones too"}]`
	// Reporting does not re-baseline: the same warning comes again until it is accepted.
	for range 2 {
		if got := probePinned(t, s, changed, false); len(got) == 0 || got[0].Level != "warning" {
			t.Fatalf("findings before accept = %+v", got)
		}
	}
	if got := probePinned(t, s, changed, true); len(got) != 1 || got[0].Level != "info" || !strings.Contains(got[0].Message, "accepted") {
		t.Fatalf("accept findings = %+v", got)
	}
	if got := probePinned(t, s, changed, false); len(got) != 0 {
		t.Fatalf("findings after accept = %+v", got)
	}
}

func TestToolPinInvalidBaselineIsKeptEvenOnAccept(t *testing.T) {
	s := pinnedService(t)
	if err := os.WriteFile(s.toolPinsPath(), []byte("not json"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, accept := range []bool{false, true} {
		if got := probePinned(t, s, pinnedTools, accept); len(got) != 1 || got[0].Level != "warning" || !strings.Contains(got[0].Message, "move that file away") {
			t.Fatalf("accept=%v findings = %+v", accept, got)
		}
	}
	// Other servers' pins may be in an unreadable file too (or a newer version's): never overwrite it.
	if data, _ := os.ReadFile(s.toolPinsPath()); string(data) != "not json" {
		t.Fatalf("pin file was replaced: %q", data)
	}
}

func TestToolPinNeedsLive(t *testing.T) {
	s := checkService(t, liveConfig("pinned", map[string]string{"MCP_LIVE_TOOLS": pinnedTools}))
	// An unreadable baseline would be reported by a live check; a static one never reads it.
	if err := os.MkdirAll(s.StateDir+"/mcp", 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.toolPinsPath(), []byte("not json"), 0600); err != nil {
		t.Fatal(err)
	}
	opts := liveCheck(10*time.Second, tokenEnv)
	opts.Live, opts.AcceptTools = false, true
	report, err := s.Check(opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range report.Servers[0].Findings {
		if f.Check == "tool-change" {
			t.Fatalf("a check without Live reported %+v", f)
		}
	}
	if data, _ := os.ReadFile(s.toolPinsPath()); string(data) != "not json" {
		t.Fatalf("a check without Live wrote the baseline: %q", data)
	}
}

func TestToolPinKeepsScopesAndRedactsNames(t *testing.T) {
	root := t.TempDir()
	entry := fmt.Sprintf("command: %q\nargs: [%q, \"--\", \"pinned\"]\nenv:\n  MCP_LIVE_HELPER: \"1\"\n  API_TOKEN: {fromEnv: LIVE_TOKEN}\n  MCP_LIVE_TOOLS: %q\n",
		os.Args[0], "-test.run=^TestMCPLiveHelperServer$", `[{"name":"leak-`+liveSecret+`"}]`)
	at := func(pad string) string {
		var b strings.Builder
		for _, line := range strings.Split(strings.TrimSuffix(entry, "\n"), "\n") {
			b.WriteString(pad + line + "\n")
		}
		return b.String()
	}
	s := checkService(t, "mcp:\n  targets: [claude]\n  servers:\n    fake:\n"+at("      ")+"  projects:\n    "+root+":\n      servers:\n        fake:\n"+at("          "))
	report, err := s.Check(liveCheck(10*time.Second, tokenEnv))
	if err != nil {
		t.Fatal(err)
	}
	assertNoSecret(t, report)
	pins, err := s.readToolPins()
	if err != nil {
		t.Fatal(err)
	}
	if len(pins.Servers[""]["fake"]) != 1 || len(pins.Servers[root]["fake"]) != 1 {
		t.Fatalf("pins = %+v", pins)
	}
	if data, _ := os.ReadFile(s.toolPinsPath()); strings.Contains(string(data), liveSecret) {
		t.Fatalf("pin file holds a configured value: %s", data)
	}
}

func TestToolPinSeparatesProjectModeFromGlobal(t *testing.T) {
	global := pinnedService(t)
	root := t.TempDir()
	project := *global
	project.ProjectRoot = root
	project.ConfigPath = filepath.Join(root, ".skillshare", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(project.ConfigPath), 0700); err != nil {
		t.Fatal(err)
	}
	// Same server name, other tools, same state directory: a first probe, not a change.
	got := probePinned(t, &project, `[{"name":"search"}]`, false)
	if len(got) != 1 || got[0].Level != "info" {
		t.Fatalf("project findings = %+v", got)
	}
	if again := probePinned(t, global, pinnedTools, false); len(again) != 0 {
		t.Fatalf("global findings after a project pin = %+v", again)
	}
}
