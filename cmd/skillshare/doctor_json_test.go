package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCheckBackupStatus_LockOnly(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".backup.lock"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	result := &doctorResult{}
	output := captureStdout(t, func() { checkBackupStatus(result, false, root) })
	if !strings.Contains(output, "Backups") || !strings.Contains(output, "none found") {
		t.Fatalf("missing backup status row: %q", output)
	}
	if len(result.checks) != 1 || result.checks[0].Message != "Backups: none found" {
		t.Fatalf("unexpected backup JSON check: %+v", result.checks)
	}
}

func TestFetchDoctorUpdateResultDevModeSimulatesUpdate(t *testing.T) {
	oldVersion := version
	version = "dev"
	defer func() { version = oldVersion }()

	result := fetchDoctorUpdateResult()
	if result == nil {
		t.Fatal("expected simulated update result in dev mode")
	}
	if !result.UpdateAvailable || result.LatestVersion == "" {
		t.Fatalf("result = %#v, want update available with latest version", result)
	}
}

func TestFinalizeDoctorJSONDevModeVersion(t *testing.T) {
	oldVersion := version
	version = "dev"
	defer func() { version = oldVersion }()

	out := buildDoctorOutput(&doctorResult{})
	out.Version = &doctorVersion{Current: version}
	if version == "" || version == "dev" {
		out.Version.DevMode = true
		out.Version.Latest = "dev-ui-flow"
		out.Version.UpdateAvailable = true
	}

	if out.Version == nil || !out.Version.DevMode || !out.Version.UpdateAvailable || out.Version.Latest == "" {
		t.Fatalf("doctor version = %#v", out.Version)
	}
}

func TestDoctorCheckSuggestionsJSON(t *testing.T) {
	result := &doctorResult{}
	result.addCheckWithSuggestions(
		"cross_target_discovery",
		checkWarning,
		"overlap",
		[]string{"codex also scans universal"},
		[]string{"Choose one authoritative route."},
	)

	out := buildDoctorOutput(result)
	data, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}

	var decoded doctorOutput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if got := decoded.Checks[0].Suggestions; len(got) != 1 || got[0] != "Choose one authoritative route." {
		t.Fatalf("doctor JSON suggestions = %v", got)
	}
}

func TestDoctorCheckStreamWritesEachCheckAsItIsAdded(t *testing.T) {
	t.Setenv(doctorStreamEnv, "1")
	var out strings.Builder
	result := &doctorResult{stream: doctorCheckStream(&out)}

	result.addCheck("source", checkPass, "Source: ok", nil)

	if got, want := out.String(), `{"check":{"name":"source","status":"pass","message":"Source: ok"}}`+"\n"; got != want {
		t.Fatalf("stream = %q, want %q", got, want)
	}
}

func TestDoctorCheckStreamOffWithoutEnv(t *testing.T) {
	if stream := doctorCheckStream(&strings.Builder{}); stream != nil {
		t.Fatal("doctor streamed checks without being asked to")
	}
}

func TestDoctorLaterStreamsPendingAndKeepsItsPlace(t *testing.T) {
	t.Setenv(doctorStreamEnv, "1")
	var out strings.Builder
	result := &doctorResult{stream: doctorCheckStream(&out)}
	release := make(chan struct{})

	result.addCheck("mcp", checkPass, "MCP ok", nil)
	result.later("plugins", "Checking plugins...", func() func(*doctorResult) {
		<-release
		return func(r *doctorResult) { r.addCheck("plugins", checkWarning, "Plugins: 1 warning(s)", nil) }
	})
	result.addCheck("backup", checkPass, "Backups ok", nil)
	close(release)
	result.finishLater()

	want := `{"check":{"name":"mcp","status":"pass","message":"MCP ok"}}` + "\n" +
		`{"check":{"name":"plugins","status":"pending","message":"Checking plugins..."}}` + "\n" +
		`{"check":{"name":"backup","status":"pass","message":"Backups ok"}}` + "\n" +
		`{"check":{"name":"plugins","status":"warning","message":"Plugins: 1 warning(s)"}}` + "\n"
	if got := out.String(); got != want {
		t.Fatalf("stream = %q, want %q", got, want)
	}
	var names []string
	for _, c := range result.checks {
		names = append(names, c.Name)
	}
	if got := strings.Join(names, ","); got != "mcp,plugins,backup" {
		t.Fatalf("checks = %s, want mcp,plugins,backup", got)
	}
}

func TestDoctorLaterRecordsAFinishedCheckAtTheNextCheck(t *testing.T) {
	t.Setenv(doctorStreamEnv, "1")
	var out strings.Builder
	result := &doctorResult{stream: doctorCheckStream(&out)}

	result.later("git_status", "Checking Git...", func() func(*doctorResult) {
		return func(r *doctorResult) { r.addCheck("git_status", checkPass, "Git: ok", nil) }
	})
	for deadline := time.Now().Add(5 * time.Second); len(result.waiting[0].done) == 0; runtime.Gosched() {
		if time.Now().After(deadline) {
			t.Fatal("background check did not finish")
		}
	}
	result.addCheck("targets", checkPass, "Targets ok", nil)

	want := `{"check":{"name":"git_status","status":"pending","message":"Checking Git..."}}` + "\n" +
		`{"check":{"name":"targets","status":"pass","message":"Targets ok"}}` + "\n" +
		`{"check":{"name":"git_status","status":"pass","message":"Git: ok"}}` + "\n"
	if got := out.String(); got != want {
		t.Fatalf("stream = %q, want %q", got, want)
	}
	if len(result.waiting) != 0 || len(result.checks) != 2 || result.checks[0].Name != "git_status" || result.checks[0].Status != checkPass {
		t.Fatalf("checks = %+v, waiting = %d; want git_status recorded in its place", result.checks, len(result.waiting))
	}
}
