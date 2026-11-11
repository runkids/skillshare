package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	versioncheck "skillshare/internal/version"
)

type updateRoundTrip func(*http.Request) (*http.Response, error)

func (f updateRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestVersionCheckRechecksPolicyBeforePublishing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", path)
	t.Setenv(config.AutoUpdateCheckEnv, "")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	if err := os.WriteFile(path, []byte("updates: {auto_check: true}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	oldVersion, oldTransport := versioncheck.Version, http.DefaultTransport
	versioncheck.Version = "1.0.0"
	defer func() { versioncheck.Version = oldVersion; http.DefaultTransport = oldTransport }()
	calls := 0
	http.DefaultTransport = updateRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if err := os.WriteFile(path, []byte("updates: {auto_check: false}\n"), 0600); err != nil {
			t.Fatal(err)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"tag_name":"v9.9.9"}`)), Request: r}, nil
	})
	source := t.TempDir()
	if err := os.MkdirAll(filepath.Join(source, "skillshare"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "skillshare", "SKILL.md"), []byte("---\nmetadata:\n  version: 1.0.0\n---\n"), 0600); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	(&Server{cfg: &config.Config{Source: source}}).handleVersionCheck(rec, httptest.NewRequest(http.MethodGet, "/api/version", nil))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || body["autoCheckEnabled"] != false || body["cliUpdateAvailable"] != false || body["cliLatest"] != nil {
		t.Fatalf("inflight opt-out leaked: calls=%d body=%s", calls, rec.Body.String())
	}
}

func TestManualVersionCheckSkillResult(t *testing.T) {
	for _, project := range []bool{false, true} {
		mode := "global"
		if project {
			mode = "project"
		}
		for _, tt := range []struct {
			name         string
			status       int
			body         string
			networkError bool
			wantStatus   int
		}{
			{name: "network failure", networkError: true, wantStatus: http.StatusBadGateway},
			{name: "HTTP failure", status: http.StatusServiceUnavailable, wantStatus: http.StatusBadGateway},
			{name: "missing version", status: http.StatusOK, body: "---\nname: skillshare\n---\n", wantStatus: http.StatusBadGateway},
			{name: "current", status: http.StatusOK, body: "---\nmetadata:\n  version: 1.0.0\n---\n", wantStatus: http.StatusOK},
		} {
			t.Run(mode+"/"+tt.name, func(t *testing.T) {
				t.Setenv(config.AutoUpdateCheckEnv, "false")
				source := t.TempDir()
				if err := os.MkdirAll(filepath.Join(source, "skillshare"), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(source, "skillshare", "SKILL.md"), []byte("---\nmetadata:\n  version: 1.0.0\n---\n"), 0600); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(t.TempDir(), "config.yaml")
				if err := os.WriteFile(path, []byte("source: "+source+"\n"), 0600); err != nil {
					t.Fatal(err)
				}
				t.Setenv("SKILLSHARE_CONFIG", path)
				oldVersion, oldTransport := versioncheck.Version, http.DefaultTransport
				versioncheck.Version = "1.0.0"
				t.Cleanup(func() { versioncheck.Version = oldVersion; http.DefaultTransport = oldTransport })
				calls := 0
				http.DefaultTransport = updateRoundTrip(func(r *http.Request) (*http.Response, error) {
					calls++
					status, body := http.StatusOK, `{"tag_name":"v1.0.0"}`
					if r.URL.String() == versioncheck.SkillSourceURL {
						if tt.networkError {
							return nil, errors.New("skill request failed")
						}
						status, body = tt.status, tt.body
					}
					return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
				})
				s := &Server{cfg: &config.Config{Source: source}}
				if project {
					s.projectRoot = t.TempDir()
					s.cfg.Source = t.TempDir()
				}
				rec := httptest.NewRecorder()
				s.handleVersionCheck(rec, httptest.NewRequest(http.MethodGet, "/api/version?manual=true", nil))
				if rec.Code != tt.wantStatus || calls != 2 {
					t.Fatalf("status=%d calls=%d body=%s", rec.Code, calls, rec.Body.String())
				}
				var body map[string]any
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if tt.wantStatus != http.StatusOK {
					if body["error"] != "failed to check built-in skill version" {
						t.Fatalf("missing check failure: %s", rec.Body.String())
					}
				} else if body["skillLatest"] != "1.0.0" || body["skillUpdateAvailable"] != false || body["cliUpdateAvailable"] != false {
					t.Fatalf("current versions not verified: %s", rec.Body.String())
				}
			})
		}
	}
}

func TestPatchUpdatePreference(t *testing.T) {
	s, _ := newTestServer(t)
	t.Setenv(config.AutoUpdateCheckEnv, "")
	rec := httptest.NewRecorder()
	s.handlePatchConfig(rec, httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(`{"autoUpdateCheck":false}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("patch: %d %s", rec.Code, rec.Body.String())
	}
	policy, err := config.ReadUpdatePolicy()
	if err != nil || policy.Enabled {
		t.Fatalf("saved preference: %+v %v", policy, err)
	}
	t.Setenv(config.AutoUpdateCheckEnv, "true")
	rec = httptest.NewRecorder()
	s.handlePatchConfig(rec, httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(`{"autoUpdateCheck":false}`)))
	if rec.Code != http.StatusConflict {
		t.Fatalf("env must own effective setting: %d", rec.Code)
	}
}

func TestManualVersionCheckRemainsAvailableWhenDisabled(t *testing.T) {
	t.Setenv(config.AutoUpdateCheckEnv, "false")
	old := versioncheck.Version
	versioncheck.Version = "dev"
	defer func() { versioncheck.Version = old }()
	s := &Server{cfg: &config.Config{Source: t.TempDir()}}
	rec := httptest.NewRecorder()
	s.handleVersionCheck(rec, httptest.NewRequest(http.MethodGet, "/api/version?manual=true", nil))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["cliUpdateAvailable"] != true || body["autoCheckEnabled"] != false {
		t.Fatalf("manual check blocked: %s", rec.Body.String())
	}
}

func TestVersionCheckDisabledSuppressesCachedUpdates(t *testing.T) {
	t.Setenv("SKILLSHARE_AUTO_UPDATE_CHECK", "")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	versioncheck.SaveRemoteSkillVersion("9.9.9")
	path := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", path)
	if err := os.WriteFile(path, []byte("updates:\n  auto_check: false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	s := &Server{cfg: &config.Config{Source: t.TempDir()}}
	rec := httptest.NewRecorder()
	s.handleVersionCheck(rec, httptest.NewRequest(http.MethodGet, "/api/version", nil))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["cliUpdateAvailable"] != false || body["skillUpdateAvailable"] != false || body["cliLatest"] != nil || body["skillLatest"] != nil {
		t.Fatalf("disabled check leaked an update: %s", rec.Body.String())
	}
	if body["autoCheckEnabled"] != false {
		t.Fatalf("policy missing: %s", rec.Body.String())
	}
}
