package mcp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Baseline fixtures use complete observations; public selection regressions use
// an inert local HTTP server. No test launches an executable MCP server or reads credentials.
func baselineFixture(t *testing.T) (*Service, *Source, Server, contractObservation) {
	t.Helper()
	s := checkService(t, "mcp:\n  servers:\n    echo:\n      url: https://example.invalid/mcp\n")
	source, err := LoadSource(s.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	name, tool, err := captureTool(json.RawMessage(`{"name":"echo","description":"Echo text","inputSchema":{"type":"object"}}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	now := contractObservation{Tools: map[string]ToolContract{name: tool}, Live: &CheckLive{ProtocolVersion: "2025-11-25", ServerInfo: LiveServerInfo{Name: "fixture", Version: "1"}, Tools: 1}}
	return s, source, source.Servers["echo"], now
}

func baselineObserve(s *Service, source *Source, root, name string, server Server, now contractObservation, opts CheckOptions) (*toolBaseline, *CheckReport) {
	b := s.loadToolBaseline(source, 1, opts)
	review, fs := b.compare(0, root, name, server, now)
	report := &CheckReport{Servers: []CheckServer{{Name: name, Project: root, Contract: review, Findings: fs}}}
	return b, report
}

func baselineAccept(t *testing.T, s *Service, source *Source, root, name string, server Server, now contractObservation) *CheckReport {
	t.Helper()
	_, preview := baselineObserve(s, source, root, name, server, now, CheckOptions{Live: true})
	revision := preview.Servers[0].Contract.Revision
	if revision == "" {
		t.Fatalf("no review revision: %+v", preview.Servers[0])
	}
	b, report := baselineObserve(s, source, root, name, server, now, CheckOptions{Live: true, AcceptTools: true, Names: []string{name}, Revision: revision})
	if report.Servers[0].Contract.Status == "accepted" {
		t.Fatal("acceptance claimed before persistence")
	}
	b.save(s, report)
	if report.ToolsAcceptance != ToolsAcceptanceAccepted || report.Servers[0].Contract.Status != "accepted" {
		t.Fatalf("acceptance failed: %+v", report)
	}
	return report
}

func baselineBytes(t *testing.T, s *Service) []byte {
	t.Helper()
	data, err := os.ReadFile(s.toolPinsPath())
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func writeBaseline(t *testing.T, s *Service, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(s.toolPinsPath()), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.toolPinsPath(), data, 0600); err != nil {
		t.Fatal(err)
	}
}
func assertBaselineLevel(t *testing.T, report *CheckReport, level string) {
	t.Helper()
	if !hasFinding(report.Servers[0].Findings, level, "tool-change", "") {
		t.Fatalf("missing %s: %+v", level, report.Servers[0])
	}
}

// Catches reintroducing first-use trust or a write on a read-only check.
func TestToolBaselineExplicitTrustAndStableAcceptance(t *testing.T) {
	s, source, server, now := baselineFixture(t)
	b, report := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true})
	if review := report.Servers[0].Contract; review.Status != "untrusted" || len(review.Revision) != 64 || len(review.Tools) != 1 {
		t.Fatalf("review = %+v", review)
	}
	assertBaselineLevel(t, report, "warning")
	b.save(s, report)
	if _, err := os.Stat(s.toolPinsPath()); !os.IsNotExist(err) {
		t.Fatalf("first check wrote store: %v", err)
	}
	baselineAccept(t, s, source, "", "echo", server, now)
	before := baselineBytes(t, s)
	_, report = baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true})
	if report.Servers[0].Contract.Status != "unchanged" {
		t.Fatalf("review = %+v", report.Servers[0])
	}
	baselineAccept(t, s, source, "", "echo", server, now)
	if !bytes.Equal(before, baselineBytes(t, s)) {
		t.Fatal("no-op acceptance rewrote store")
	}
	// Re-ordered descriptor keys produce the same observation/revision.
	_, tool, err := captureTool(json.RawMessage(`{"inputSchema": {"type":"object"}, "description":"Echo text", "name":"echo"}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	now.Tools = map[string]ToolContract{"echo": tool}
	_, again := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true})
	if again.Servers[0].Contract.Revision != report.Servers[0].Contract.Revision {
		t.Fatal("equivalent inventory changed review")
	}
}

func TestToolBaselineDriftPreservesApproval(t *testing.T) {
	for _, change := range []string{"description", "inputSchema", "new", "removed"} {
		t.Run(change, func(t *testing.T) {
			s, source, server, now := baselineFixture(t)
			baselineAccept(t, s, source, "", "echo", server, now)
			before := baselineBytes(t, s)
			raw := `{"name":"echo","description":"Changed text","inputSchema":{"type":"object"}}`
			if change == "inputSchema" {
				raw = `{"name":"echo","description":"Echo text","inputSchema":{"type":"object","required":["x"]}}`
			}
			if change == "new" {
				raw = `{"name":"extra","inputSchema":{"type":"object"}}`
			}
			name, tool, err := captureTool(json.RawMessage(raw), nil)
			if err != nil {
				t.Fatal(err)
			}
			now.Tools = map[string]ToolContract{name: tool}
			if change == "removed" {
				now.Tools = map[string]ToolContract{}
			}
			b, report := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true})
			b.save(s, report)
			if report.Servers[0].Contract.Status != "drifted" || len(report.Servers[0].Contract.Changes) == 0 {
				t.Fatalf("review = %+v", report.Servers[0])
			}
			assertBaselineLevel(t, report, "warning")
			if !bytes.Equal(before, baselineBytes(t, s)) {
				t.Fatal("drift replaced baseline")
			}
			baselineAccept(t, s, source, "", "echo", server, now)
		})
	}
}

func TestToolBaselineLegacyPreservedAndExplicitlyMigrated(t *testing.T) {
	s, source, server, now := baselineFixture(t)
	legacy := []byte(`{"version":1,"servers":{"":{"echo":{"echo":"` + strings.Repeat("a", 64) + `"},"other":{"old":"` + strings.Repeat("b", 64) + `"}},"/unselected":{"remote":{}}}}`)
	writeBaseline(t, s, legacy)
	b, report := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true})
	b.save(s, report)
	if report.Servers[0].Contract.Status != "legacy" || !bytes.Equal(legacy, baselineBytes(t, s)) {
		t.Fatalf("legacy changed: %+v", report)
	}
	baselineAccept(t, s, source, "", "echo", server, now)
	pins, err := s.readToolPins()
	if err != nil {
		t.Fatal(err)
	}
	if pins.Version != 2 || len(pins.Records) != 1 || pins.Servers[""]["other"]["old"] != strings.Repeat("b", 64) || pins.Servers["/unselected"]["remote"] == nil {
		t.Fatalf("lost legacy: %+v", pins)
	}
}

func TestToolBaselineScopeAndConfigIsolation(t *testing.T) {
	s, source, server, now := baselineFixture(t)
	baselineAccept(t, s, source, "", "echo", server, now)
	root := t.TempDir()
	baselineAccept(t, s, source, root, "echo", server, now)
	other := *s
	other.ConfigPath = filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(other.ConfigPath, source.configBytes, 0600); err != nil {
		t.Fatal(err)
	}
	otherSource, err := LoadSource(other.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	_, report := baselineObserve(&other, otherSource, "", "echo", server, now, CheckOptions{Live: true})
	if report.Servers[0].Contract.Status != "untrusted" {
		t.Fatal("another config inherited trust")
	}
	baselineAccept(t, &other, otherSource, "", "echo", server, now)
	project := *s
	project.ProjectRoot = t.TempDir()
	project.ConfigPath = filepath.Join(project.ProjectRoot, ".skillshare", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(project.ConfigPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(project.ConfigPath, source.configBytes, 0600); err != nil {
		t.Fatal(err)
	}
	projectSource, err := LoadSource(project.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	baselineAccept(t, &project, projectSource, "", "echo", server, now)
	pins, err := s.readToolPins()
	if err != nil {
		t.Fatal(err)
	}
	if len(pins.Records) != 4 {
		t.Fatalf("scope records = %d", len(pins.Records))
	}
	_, report = baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true})
	if report.Servers[0].Contract.Status != "unchanged" {
		t.Fatal("other scope replaced original")
	}
}

func TestToolBaselineStaleRevisionSourceAndStore(t *testing.T) {
	for _, race := range []string{"revision", "source", "store", "same-record"} {
		t.Run(race, func(t *testing.T) {
			s, source, server, now := baselineFixture(t)
			_, preview := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true})
			revision := preview.Servers[0].Contract.Revision
			if race == "revision" {
				revision = strings.Repeat("0", 64)
			}
			b, report := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true, AcceptTools: true, Names: []string{"echo"}, Revision: revision})
			var expected []byte
			switch race {
			case "source":
				if err := os.WriteFile(source.ConfigPath, append(source.configBytes, []byte("# concurrent edit\n")...), 0600); err != nil {
					t.Fatal(err)
				}
			case "store":
				baselineAccept(t, s, source, t.TempDir(), "other", server, now)
				expected = baselineBytes(t, s)
			case "same-record":
				baselineAccept(t, s, source, "", "echo", server, now)
				expected = baselineBytes(t, s)
			}
			b.save(s, report)
			if report.ToolsAcceptance != ToolsAcceptanceStale || report.Servers[0].Contract.Status != "error" {
				t.Fatalf("stale acceptance = %+v", report)
			}
			assertBaselineLevel(t, report, "error")
			if expected != nil {
				if !bytes.Equal(expected, baselineBytes(t, s)) {
					t.Fatal("concurrent approval overwritten")
				}
			} else if _, err := os.Stat(s.toolPinsPath()); !os.IsNotExist(err) {
				t.Fatal("stale review wrote baseline")
			}
		})
	}
}

func TestToolBaselineInvalidStoreNeverOverwritten(t *testing.T) {
	for name, data := range map[string][]byte{
		"case-alias":     []byte(`{"VERSION":2,"servers":{},"records":{}}`),
		"case-duplicate": []byte(`{"Version":1,"version":2,"servers":{},"records":{}}`),
		"invalid":        []byte("not JSON"), "future": []byte(`{"version":3,"servers":{}}`), "unknown": []byte(`{"version":2,"servers":{},"records":{},"extra":true}`),
		"duplicate": []byte(`{"version":2,"version":2,"servers":{},"records":{}}`), "null": []byte(`{"version":2,"servers":{},"records":null}`),
		"legacy-hash": []byte(`{"version":1,"servers":{"":{"echo":{"echo":"bad"}}}}`), "oversized": bytes.Repeat([]byte(" "), maxToolPinBytes+1),
		"depth": []byte(`{"version":2,"servers":{},"records":{},"extra":` + strings.Repeat("[", 65) + `0` + strings.Repeat("]", 65) + `}`),
	} {
		t.Run(name, func(t *testing.T) {
			s, source, server, now := baselineFixture(t)
			writeBaseline(t, s, data)
			for _, accept := range []bool{false, true} {
				b, report := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true, AcceptTools: accept, Names: []string{"echo"}, Revision: strings.Repeat("a", 64)})
				b.save(s, report)
				if report.Servers[0].Contract.Status != "error" || report.Servers[0].Contract.Revision != "" {
					t.Fatalf("invalid store review: %+v", report)
				}
				assertBaselineLevel(t, report, "error")
				if !bytes.Equal(data, baselineBytes(t, s)) {
					t.Fatal("invalid store overwritten")
				}
			}
		})
	}
}

func TestToolBaselineRejectsTamperedDefinitionAndHash(t *testing.T) {
	for _, part := range []string{"hash", "inputHash", "proseHash", "outputHash", "metadataHash", "definition", "recordKey", "identity", "null-root", "unknown-record", "case-record", "unknown-tool", "case-tool"} {
		t.Run(part, func(t *testing.T) {
			s, source, server, now := baselineFixture(t)
			baselineAccept(t, s, source, "", "echo", server, now)
			var file map[string]any
			if err := json.Unmarshal(baselineBytes(t, s), &file); err != nil {
				t.Fatal(err)
			}
			records := file["records"].(map[string]any)
			for key, v := range records {
				record := v.(map[string]any)
				tool := record["tools"].(map[string]any)["echo"].(map[string]any)
				switch part {
				case "definition":
					tool[part] = map[string]any{"name": "echo", "inputSchema": map[string]any{}}
				case "recordKey":
					delete(records, key)
					records["bad"] = v
				case "identity":
					record["identity"] = "bad"
				case "null-root":
					record["root"] = nil
				case "unknown-record":
					record["unrecognized"] = true
				case "case-record":
					record["Root"] = record["root"]
					delete(record, "root")
				case "unknown-tool":
					tool["unrecognized"] = true
				case "case-tool":
					tool["Hash"] = tool["hash"]
					delete(tool, "hash")
				default:
					tool[part] = strings.Repeat("0", 64)
				}
				break
			}
			data, err := json.Marshal(file)
			if err != nil {
				t.Fatal(err)
			}
			writeBaseline(t, s, data)
			_, report := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true})
			if report.Servers[0].Contract.Status != "error" {
				t.Fatalf("tampering accepted: %+v", report)
			}
		})
	}
}

func TestToolBaselineRejectsSymlinkAndNonregularStore(t *testing.T) {
	for _, kind := range []string{"symlink", "directory"} {
		t.Run(kind, func(t *testing.T) {
			s, source, server, now := baselineFixture(t)
			if err := os.MkdirAll(filepath.Dir(s.toolPinsPath()), 0700); err != nil {
				t.Fatal(err)
			}
			if kind == "directory" {
				if err := os.Mkdir(s.toolPinsPath(), 0700); err != nil {
					t.Fatal(err)
				}
			} else {
				target := filepath.Join(t.TempDir(), "pins")
				if err := os.WriteFile(target, []byte(`{"version":2,"servers":{},"records":{}}`), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, s.toolPinsPath()); err != nil {
					t.Skip(err)
				}
			}
			_, report := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true})
			if report.Servers[0].Contract.Status != "error" {
				t.Fatal("unsafe store accepted")
			}
		})
	}
}

func TestToolBaselineSaveFailureAndMissingObservationTruthful(t *testing.T) {
	s, source, server, now := baselineFixture(t)
	_, preview := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true})
	opts := CheckOptions{Live: true, AcceptTools: true, Names: []string{"echo"}, Revision: preview.Servers[0].Contract.Revision}
	b, report := baselineObserve(s, source, "", "echo", server, now, opts)
	// Occupying the existing operation lock prevents persistence without relying on root permissions.
	lock, err := s.lock()
	if err != nil {
		t.Fatal(err)
	}
	b.save(s, report)
	if err := lock.Unlock(); err != nil {
		t.Fatal(err)
	}
	if report.ToolsAcceptance != ToolsAcceptanceError || report.Servers[0].Contract.Status != "error" {
		t.Fatalf("failure claimed success: %+v", report)
	}
	assertBaselineLevel(t, report, "error")
	b = s.loadToolBaseline(source, 1, opts)
	report = &CheckReport{Servers: []CheckServer{{Name: "echo"}}}
	b.save(s, report)
	if report.ToolsAcceptance != ToolsAcceptanceError || report.Servers[0].Contract.Status != "error" {
		t.Fatal("unobserved server accepted")
	}
}

func TestToolBaselineStrictPromotesTrustWarnings(t *testing.T) {
	s, source, server, now := baselineFixture(t)
	for _, status := range []string{"untrusted", "legacy", "drifted"} {
		if status == "legacy" {
			writeBaseline(t, s, []byte(`{"version":1,"servers":{"":{"echo":{}}}}`))
		}
		if status == "drifted" {
			baselineAccept(t, s, source, "", "echo", server, now)
			server.Args = []string{"changed"}
		}
		_, report := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true, StrictTools: true})
		if report.Servers[0].Contract.Status != status {
			t.Fatalf("want %s: %+v", status, report)
		}
		assertBaselineLevel(t, report, "error")
	}
}

func TestToolBaselineDomainOptionsAndRootSelection(t *testing.T) {
	s, _, _, _ := baselineFixture(t)
	for _, opts := range []CheckOptions{
		{AcceptTools: true}, {StrictTools: true}, {Live: true, Revision: strings.Repeat("a", 64)},
		{Live: true, AcceptTools: true, Revision: strings.Repeat("a", 64)},
		{Live: true, AcceptTools: true, Names: []string{"echo"}},
		{Live: true, AcceptTools: true, Names: []string{"echo"}, Revision: "bad"},
		{Live: true, AcceptTools: true, Names: []string{"echo", "echo"}, Revision: strings.Repeat("a", 64)},
	} {
		if _, err := s.Check(opts); err == nil {
			t.Fatalf("accepted malformed options: %+v", opts)
		}
	}
	root := t.TempDir()
	config := "mcp:\n  servers:\n    echo:\n      url: https://example.invalid/mcp\n  projects:\n    " + root + ":\n      servers:\n        echo:\n          url: https://example.invalid/project\n"
	if err := os.WriteFile(s.ConfigPath, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	opts := offlineCheck("echo")
	opts.Live = true
	opts.AcceptTools = true
	opts.Revision = strings.Repeat("a", 64)
	if _, err := s.Check(opts); err == nil {
		t.Fatal("ambiguous root accepted")
	}
	for _, selected := range []string{"", root} {
		opts := offlineCheck("echo")
		opts.Root = &selected
		report, err := s.Check(opts)
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Servers) != 1 || report.Servers[0].Project != selected {
			t.Fatalf("root selection: %+v", report)
		}
	}
	missing := "/not-configured"
	opts = offlineCheck()
	opts.Root = &missing
	if _, err := s.Check(opts); err == nil {
		t.Fatal("unknown root accepted")
	}
}

func TestToolBaselineSecondProbeMustMatchReviewedInventory(t *testing.T) {
	s, source, server, now := baselineFixture(t)
	_, preview := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true})
	now.Tools = map[string]ToolContract{}
	b, report := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true, AcceptTools: true, Names: []string{"echo"}, Revision: preview.Servers[0].Contract.Revision})
	b.save(s, report)
	if report.ToolsAcceptance != ToolsAcceptanceStale || report.Servers[0].Contract.Revision != "" {
		t.Fatalf("changed second inventory accepted: %+v", report)
	}
	if _, err := os.Stat(s.toolPinsPath()); !os.IsNotExist(err) {
		t.Fatal("changed second probe wrote store")
	}
}

func TestToolBaselineAuditIncompleteAlwaysPreventsRevision(t *testing.T) {
	for _, literal := range []bool{false, true} {
		s, source, server, now := baselineFixture(t)
		if literal {
			server.Env = map[string]Value{"MODE": {Literal: "fixture-mode"}}
		}
		raw, err := json.Marshal(map[string]any{"name": "echo", "description": strings.Repeat("benign text\n", 9000), "inputSchema": map[string]any{"type": "object"}})
		if err != nil {
			t.Fatal(err)
		}
		_, tool, err := captureTool(raw, nil)
		if err != nil {
			t.Fatal(err)
		}
		now.Tools = map[string]ToolContract{"echo": tool}
		_, report := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true})
		if report.Servers[0].Contract.Status != "error" || report.Servers[0].Contract.Revision != "" {
			t.Fatal("incomplete audit offered approval")
		}
		assertBaselineLevel(t, report, "error")
	}
}

func TestToolBaselineExternalSourceCAS(t *testing.T) {
	s, _, server, now := baselineFixture(t)
	path := filepath.Join(filepath.Dir(s.ConfigPath), "servers.yaml")
	if err := os.WriteFile(path, []byte("servers:\n  echo:\n    url: https://example.invalid/mcp\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.ConfigPath, []byte("sources:\n  mcp: servers.yaml\n"), 0600); err != nil {
		t.Fatal(err)
	}
	source, err := LoadSource(s.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	_, preview := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true})
	b, report := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true, AcceptTools: true, Names: []string{"echo"}, Revision: preview.Servers[0].Contract.Revision})
	if err := os.WriteFile(path, append(source.bytes, []byte("# concurrent external edit\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	b.save(s, report)
	if report.ToolsAcceptance != ToolsAcceptanceStale {
		t.Fatalf("external source race accepted: %+v", report)
	}
}

// Store framing must not lower the descriptor parser's independent nesting cap.
func TestToolBaselineNestedSnapshotRoundTrip(t *testing.T) {
	s, source, server, now := baselineFixture(t)
	value := `"leaf"`
	for range 60 {
		value = `{"nested":` + value + `}`
	}
	raw := json.RawMessage(`{"name":"echo","inputSchema":{"type":"object"},"extension":` + value + `}`)
	_, tool, err := captureTool(raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	now.Tools = map[string]ToolContract{"echo": tool}
	baselineAccept(t, s, source, "", "echo", server, now)
	_, report := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true})
	if report.Servers[0].Contract.Status != "unchanged" {
		t.Fatalf("valid nested snapshot lost trust: %+v", report)
	}
}

// Unlike an occupied operation lock, a read-only destination directory reaches
// atomicWrite after the valid source/store CAS and fails while creating its temp file.
func TestToolBaselineAtomicWriteFailureAfterValidCAS(t *testing.T) {
	s, source, server, now := baselineFixture(t)
	baselineAccept(t, s, source, "", "echo", server, now)
	before := baselineBytes(t, s)
	_, changed, err := captureTool(json.RawMessage(`{"name":"echo","description":"New reviewed text","inputSchema":{"type":"object"}}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	now.Tools = map[string]ToolContract{"echo": changed}
	_, preview := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true})
	b, report := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true, AcceptTools: true, Names: []string{"echo"}, Revision: preview.Servers[0].Contract.Revision})
	if b.updates[0] == nil {
		t.Fatal("no valid acceptance queued")
	}
	directory := filepath.Dir(s.toolPinsPath())
	info, err := os.Stat(directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(directory, 0500); err != nil {
		t.Skipf("cannot restrict fixture directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(directory, info.Mode().Perm()); err != nil {
			t.Errorf("restore fixture directory: %v", err)
		}
	})
	// Root and some filesystems ignore mode restrictions; that cannot exercise this fault.
	probe, err := os.CreateTemp(directory, "write-permission-probe-*")
	if err == nil {
		probe.Close()
		os.Remove(probe.Name())
		t.Skip("fixture directory remains writable with current privileges")
	}
	// Demonstrate that the operation lock and exact source/store CAS prerequisites
	// remain available. Only atomicWrite's creation needs directory write permission.
	lock, err := s.lock()
	if err != nil {
		t.Fatalf("fault occurs before writer: %v", err)
	}
	if err := lock.Unlock(); err != nil {
		t.Fatal(err)
	}
	if err := source.CheckUnchanged(); err != nil {
		t.Fatal(err)
	}
	current, exists, err := readToolPinBytes(s.toolPinsPath())
	if err != nil || !exists || !bytes.Equal(current, b.before) {
		t.Fatalf("CAS prerequisites changed: %v", err)
	}
	b.save(s, report)
	if report.ToolsAcceptance != ToolsAcceptanceError || report.Servers[0].Contract.Status != "error" || report.Servers[0].Contract.Revision != "" {
		t.Fatalf("write failure claimed approval: %+v", report)
	}
	assertBaselineLevel(t, report, "error")
	if !bytes.Equal(before, baselineBytes(t, s)) {
		t.Fatal("failed atomic write changed prior approval")
	}
	for _, finding := range report.Servers[0].Findings {
		if finding.Level == "info" && finding.Check == "tool-change" {
			t.Fatalf("write failure emitted success: %+v", finding)
		}
	}
}

// Exercise public Check selection and reprobes: equal endpoints/contracts cannot
// reuse a token across a configured root or another project config sharing state.
func TestToolBaselineConfiguredScopesRejectCrossRootRevisions(t *testing.T) {
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusOK)
			return
		}
		var request struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if json.NewDecoder(r.Body).Decode(&request) != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if len(request.ID) == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		var result any
		switch request.Method {
		case "server/discover":
			result = map[string]any{"supportedVersions": []string{liveProtocolVersion}, "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": LiveServerInfo{Name: "scope-fixture", Version: "1"}}
		case "initialize":
			result = map[string]any{"protocolVersion": legacyProtocolVersion, "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": LiveServerInfo{Name: "scope-fixture", Version: "1"}}
		case "tools/list":
			result = map[string]any{"tools": []any{map[string]any{"name": "echo", "description": "Echo text", "inputSchema": map[string]any{"type": "object"}}}}
		default:
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
	}))
	t.Cleanup(endpoint.Close)
	root := t.TempDir()
	rootYAML, _ := json.Marshal(root)
	config := "mcp:\n  targets: []\n  servers:\n    echo:\n      targets: []\n      url: " + endpoint.URL + "\n  projects:\n    " + string(rootYAML) + ":\n      servers:\n        echo:\n          targets: []\n          url: " + endpoint.URL + "\n"
	s := checkService(t, config)
	project := *s
	project.ProjectRoot = root
	project.ConfigPath = filepath.Join(root, ".skillshare", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(project.ConfigPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(project.ConfigPath, []byte("mcp:\n  targets: []\n  servers:\n    echo:\n      targets: []\n      url: "+endpoint.URL+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	check := func(service *Service, scope, revision string, accept bool) *CheckReport {
		t.Helper()
		opts := offlineCheck("echo")
		opts.Root = &scope
		opts.Live = true
		opts.AcceptTools = accept
		opts.Revision = revision
		opts.StrictTools = !accept
		report, err := service.Check(opts)
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Servers) != 1 || report.Servers[0].Project != scope || report.Servers[0].Contract == nil {
			t.Fatalf("selection/probe failed: %+v", report)
		}
		return report
	}
	type selectedScope struct {
		service *Service
		root    string
	}
	scopes := []selectedScope{{s, ""}, {s, root}, {&project, ""}}
	// All previews read one unchanged store so cross-scope refusal is attributable
	// to identity/scope, rather than a store update invalidating the source token.
	for i, from := range scopes {
		reviewed := check(from.service, from.root, "", false)
		revision := reviewed.Servers[0].Contract.Revision
		if revision == "" {
			t.Fatalf("no preview token for scope %d: %+v", i, reviewed)
		}
		for j, to := range scopes {
			if i != j {
				refused := check(to.service, to.root, revision, true)
				if refused.ToolsAcceptance != ToolsAcceptanceStale || refused.Servers[0].Contract.Status != "error" {
					t.Fatalf("token crossed scopes %d->%d: %+v", i, j, refused)
				}
				if _, err := os.Stat(s.toolPinsPath()); !os.IsNotExist(err) {
					t.Fatalf("cross-scope token created store: %v", err)
				}
			}
		}
	}
	// Successful acceptance through the same public selection boundary preserves
	// approvals for every other configured scope in the shared state directory.
	for i, scope := range scopes {
		preview := check(scope.service, scope.root, "", false)
		accepted := check(scope.service, scope.root, preview.Servers[0].Contract.Revision, true)
		if accepted.ToolsAcceptance != ToolsAcceptanceAccepted || accepted.Servers[0].Contract.Status != "accepted" {
			t.Fatalf("same-scope acceptance %d failed: %+v", i, accepted)
		}
	}
	pins, err := s.readToolPins()
	if err != nil {
		t.Fatal(err)
	}
	if len(pins.Records) != len(scopes) {
		t.Fatalf("scope approvals overwritten: %d", len(pins.Records))
	}
	for i, scope := range scopes {
		report := check(scope.service, scope.root, "", false)
		if report.Servers[0].Contract.Status != "unchanged" || report.Summary.Errors != 0 {
			t.Fatalf("scope %d lost approval: %+v", i, report)
		}
	}
}

// A rotated fromEnv value may newly match text in the approved snapshot that
// disappeared from the current inventory. That old text must never enter a diff.
func TestToolBaselineRotatedCredentialRejectsOldSnapshotBeforeDiff(t *testing.T) {
	const rotated = "ROTATED_FIXTURE_TOKEN_793"
	for label, raw := range map[string]string{
		"removed name":         `{"name":"retired-` + rotated + `","inputSchema":{"type":"object"}}`,
		"removed path":         `{"name":"echo","inputSchema":{"type":"object","properties":{"` + rotated + `":{"type":"string"}}}}`,
		"old prose":            `{"name":"echo","description":"previous text ` + rotated + `","inputSchema":{"type":"object"}}`,
		"decoded metadata key": `{"name":"echo","inputSchema":{"type":"object"},"extension":{"\u0052OTATED_FIXTURE_TOKEN_793":"old"}}`,
		"decoded nested value": `{"name":"echo","inputSchema":{"type":"object","default":{"old":["\u0052OTATED_FIXTURE_TOKEN_793"]}}}`,
	} {
		t.Run(label, func(t *testing.T) {
			s, source, server, current := baselineFixture(t)
			server.Env = map[string]Value{"TOKEN": {FromEnv: "FIXTURE_ROTATION_TOKEN"}}
			old := current
			old.Secrets = []string{"previous-fixture-credential"}
			name, tool, err := captureTool(json.RawMessage(raw), old.Secrets)
			if err != nil {
				t.Fatal(err)
			}
			old.Tools = map[string]ToolContract{name: tool}
			baselineAccept(t, s, source, "", "echo", server, old)
			before := baselineBytes(t, s)
			current.Secrets = old.Secrets
			_, preRotation := baselineObserve(s, source, "", "echo", server, current, CheckOptions{Live: true})
			revision := preRotation.Servers[0].Contract.Revision
			if revision == "" || preRotation.Servers[0].Contract.Status != "drifted" {
				t.Fatalf("missing pre-rotation review: %+v", preRotation)
			}
			current.Secrets = []string{rotated}
			for _, opts := range []CheckOptions{{Live: true}, {Live: true, StrictTools: true}, {Live: true, AcceptTools: true, Names: []string{"echo"}, Revision: revision}} {
				b, report := baselineObserve(s, source, "", "echo", server, current, opts)
				b.save(s, report)
				review := report.Servers[0].Contract
				if review.Status != "error" || review.Revision != "" || len(review.Changes) != 0 || review.IdentityChanged {
					t.Fatalf("old credential entered review: %+v", report)
				}
				assertBaselineLevel(t, report, "error")
				if opts.AcceptTools && report.ToolsAcceptance != ToolsAcceptanceError {
					t.Fatalf("credential-bearing old snapshot accepted: %+v", report)
				}
				payload, err := json.Marshal(report)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(payload), rotated) {
					t.Fatal("rotated credential leaked from old snapshot")
				}
				if !bytes.Equal(before, baselineBytes(t, s)) {
					t.Fatal("refusal rewrote old approval")
				}
			}
		})
	}
}

func TestToolBaselineRotatedCredentialRejectsSelectedLegacyNames(t *testing.T) {
	s, source, server, now := baselineFixture(t)
	secret := "ROTATED_LEGACY_FIXTURE_281"
	writeBaseline(t, s, []byte(`{"version":1,"servers":{"":{"echo":{"removed-`+secret+`":"`+strings.Repeat("a", 64)+`"}}}}`))
	before := baselineBytes(t, s)
	now.Secrets = []string{secret}
	b, report := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true})
	b.save(s, report)
	if review := report.Servers[0].Contract; review.Status != "error" || review.Revision != "" {
		t.Fatalf("legacy credential allowed review: %+v", report)
	}
	payload, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), secret) {
		t.Fatal("legacy credential leaked")
	}
	if !bytes.Equal(before, baselineBytes(t, s)) {
		t.Fatal("legacy refusal changed evidence")
	}
}

func TestToolBaselineObservationCredentialRejectedBeforeSnapshot(t *testing.T) {
	s, source, server, now := baselineFixture(t)
	secret := "CURRENT_FIXTURE_SECRET_927"
	name, tool, err := captureTool(json.RawMessage(`{"name":"echo","description":"`+secret+`","inputSchema":{"type":"object"}}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	now.Tools = map[string]ToolContract{name: tool}
	now.Secrets = []string{secret}
	_, report := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true})
	if review := report.Servers[0].Contract; review.Status != "error" || review.Revision != "" || len(review.Tools) != 0 {
		t.Fatalf("credential snapshot exposed: %+v", report)
	}
	payload, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), secret) {
		t.Fatal("current credential exposed")
	}
}

func TestToolBaselineRotatedCredentialChecksActiveRecordBeforeLegacyEvidence(t *testing.T) {
	s, source, server, now := baselineFixture(t)
	secret := "SUPERSEDED_LEGACY_FIXTURE_361"
	writeBaseline(t, s, []byte(`{"version":1,"servers":{"":{"echo":{"removed-`+secret+`":"`+strings.Repeat("a", 64)+`"}}}}`))
	baselineAccept(t, s, source, "", "echo", server, now)
	// The retained v1 hashes are no longer this server's active review snapshot.
	// Only the explicitly approved, selected v2 record contributes diff content.
	now.Secrets = []string{secret}
	_, report := baselineObserve(s, source, "", "echo", server, now, CheckOptions{Live: true})
	if review := report.Servers[0].Contract; review.Status != "unchanged" || review.Revision == "" {
		t.Fatalf("superseded legacy evidence blocked active review: %+v", report)
	}
	payload, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), secret) {
		t.Fatal("superseded legacy contents exposed")
	}
}
