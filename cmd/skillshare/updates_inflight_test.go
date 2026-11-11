package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"skillshare/internal/config"
	versioncheck "skillshare/internal/version"
)

type updateTestTransport func(*http.Request) (*http.Response, error)

func (f updateTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestStatusVersionOptOutDuringFetch(t *testing.T) {
	source := t.TempDir()
	if err := os.MkdirAll(filepath.Join(source, "skillshare"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "skillshare", "SKILL.md"), []byte("---\nmetadata:\n  version: 1.0.0\n---\n"), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", path)
	t.Setenv(config.AutoUpdateCheckEnv, "")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	if err := os.WriteFile(path, []byte("updates: {auto_check: true}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	old := http.DefaultTransport
	defer func() { http.DefaultTransport = old }()
	calls := 0
	http.DefaultTransport = updateTestTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if err := os.WriteFile(path, []byte("updates: {auto_check: false}\n"), 0600); err != nil {
			t.Fatal(err)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("---\nmetadata:\n  version: 9.9.9\n---\n")), Request: r}, nil
	})
	out := captureStdout(t, func() { checkSkillVersion(&config.Config{Source: source}) })
	if calls != 1 || strings.Contains(out, "9.9.9") || strings.Contains(out, "available") {
		t.Fatalf("inflight update leaked: calls=%d output=%s", calls, out)
	}
}

func TestDoctorOptOutSuppressesBufferedUpdates(t *testing.T) {
	t.Setenv(config.AutoUpdateCheckEnv, "false")
	update := &versioncheck.CheckResult{CurrentVersion: "1.0.0", LatestVersion: "9.9.9", UpdateAvailable: true}
	out := captureStdout(t, func() { printUpdateAvailable(update, &doctorResult{}) })
	if out != "" {
		t.Fatalf("buffered text notice leaked: %s", out)
	}
	result := &doctorResult{warnings: 2, start: time.Now(), next: []string{"skillshare upgrade --skill", "update the built-in skill", "skillshare sync", "sync targets"}, updateCommands: []string{"skillshare upgrade --skill"}, checks: []doctorCheck{
		{Name: "skill_version", Status: checkWarning, Message: "update", UpdateAvailable: true},
		{Name: "config", Status: checkWarning, Message: "unrelated warning"},
	}}
	out = captureStdout(t, func() { printDoctorSummary(result) })
	if strings.Contains(out, "upgrade") || !strings.Contains(out, "1 warning") || !strings.Contains(out, "skillshare sync") {
		t.Fatalf("buffered summary leaked update or hid diagnostics: %s", out)
	}
	ch := make(chan *versioncheck.CheckResult, 1)
	ch <- update
	out = captureStdout(t, func() {
		if err := finalizeDoctorJSON(func() {}, result, ch); err != nil {
			t.Fatal(err)
		}
	})
	var body doctorOutput
	if err := json.Unmarshal([]byte(out), &body); err != nil {
		t.Fatal(err)
	}
	if body.Version.UpdateAvailable || body.Version.Latest != "" || body.Summary.Warnings != 1 || len(body.Checks) != 1 {
		t.Fatalf("buffered JSON leaked update: %s", out)
	}
}
