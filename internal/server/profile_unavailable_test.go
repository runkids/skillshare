package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/profile"
)

func TestProfiles_AnalyzeUnavailable(t *testing.T) {
	for _, withTarget := range []bool{true, false} {
		name := "zero_targets"
		targets := map[string]string{}
		if withTarget {
			name = "configured_target"
			targets["claude"] = t.TempDir()
		}
		t.Run(name, func(t *testing.T) {
			s, source := newTestServerWithTargets(t, targets)
			addSkill(t, source, "alpha")
			request := func() *httptest.ResponseRecorder {
				rr := httptest.NewRecorder()
				s.handleAnalyze(rr, httptest.NewRequest(http.MethodGet, "/api/analyze", nil))
				return rr
			}
			available := request()
			var baseline struct {
				Targets []analyzeTargetResponse `json:"targets"`
			}
			if err := json.Unmarshal(available.Body.Bytes(), &baseline); err != nil || available.Code != http.StatusOK {
				t.Fatalf("available control: %d %s, %v", available.Code, available.Body.String(), err)
			}
			if withTarget {
				if len(baseline.Targets) != 1 || baseline.Targets[0].SkillCount != 1 || len(baseline.Targets[0].Skills) != 1 || baseline.Targets[0].Skills[0].Name != "alpha" {
					t.Fatalf("available Analyze lost alpha: %s", available.Body.String())
				}
			} else if len(baseline.Targets) != 0 {
				t.Fatalf("zero-target control invented targets: %s", available.Body.String())
			}
			statePath, _, err := config.ProfilePaths(config.ConfigPath())
			if err != nil {
				t.Fatal(err)
			}
			// Local selection survives a definition disappearing from shared config.
			if err := profile.WriteState(statePath, profile.State{Version: 1, Active: "missing"}); err != nil {
				t.Fatal(err)
			}
			s.cfg, err = config.LoadReadOnlyPath(config.ConfigPath())
			if err != nil {
				t.Fatal(err)
			}
			got := request()
			var refusal struct {
				Code  string `json:"error_code"`
				Error string `json:"error"`
			}
			if err := json.Unmarshal(got.Body.Bytes(), &refusal); err != nil {
				t.Fatal(err)
			}
			if got.Code != http.StatusConflict || refusal.Code != "profile_unavailable" || !strings.Contains(refusal.Error, "missing") || !strings.Contains(refusal.Error, "skillshare profile use --none") {
				t.Errorf("unavailable Analyze lost refusal/repair: %d %s", got.Code, got.Body.String())
			}
		})
	}
}

func TestProfiles_UnavailableBeforeDiffAndMatrix(t *testing.T) {
	s, _ := newTestServer(t)
	path := config.ConfigPath()
	statePath, _, err := config.ProfilePaths(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := profile.WriteState(statePath, profile.State{Version: 1, Active: "missing"}); err != nil {
		t.Fatal(err)
	}
	s.cfg, err = config.LoadReadOnlyPath(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, handler := range []http.HandlerFunc{s.handleDiff, s.handleDiffStream, s.handleSyncMatrix} {
		rr := httptest.NewRecorder()
		handler(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "profile_unavailable") {
			t.Fatalf("unavailable profile returned %d: %s", rr.Code, rr.Body.String())
		}
		if strings.Contains(rr.Header().Get("Content-Type"), "text/event-stream") {
			t.Fatal("stream started before profile refusal")
		}
	}
}

func TestProfiles_UnavailableBlocksSkillsButNotAgents(t *testing.T) {
	s, _ := newTestServer(t)
	statePath, _, err := config.ProfilePaths(config.ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	if err := profile.WriteState(statePath, profile.State{Version: 1, Active: "missing"}); err != nil {
		t.Fatal(err)
	}
	s.cfg, err = config.LoadReadOnlyPath(config.ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		body    string
		blocked bool
	}{
		{`{"kind":"skill","dryRun":true}`, true},
		{`{"dryRun":true}`, true},
		{`{"kind":"agent","dryRun":true}`, false},
	} {
		rr := httptest.NewRecorder()
		s.handleSync(rr, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body)))
		if (rr.Code == http.StatusConflict) != tc.blocked {
			t.Fatalf("body %s returned %d: %s", tc.body, rr.Code, rr.Body.String())
		}
	}
}
