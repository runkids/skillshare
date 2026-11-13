package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/profile"
)

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
	for _, tc := range []struct{ body string; blocked bool }{
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
