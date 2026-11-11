package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/search"
)

func TestSavedHubURLSet(t *testing.T) {
	hub := config.HubConfig{Hubs: []config.HubEntry{
		{Label: "team", URL: "git@ghe.corp.com:team/skills.git"},
		{Label: "blank", URL: "  "},
	}}
	set := savedHubURLSet(hub)
	if !set["git@ghe.corp.com:team/skills.git"] {
		t.Error("expected saved SSH hub URL to be in set")
	}
	if set[""] {
		t.Error("blank URL should not be added to set")
	}
}

func TestHandleSearch_SSHHubNotSaved_Forbidden(t *testing.T) {
	s, _ := newTestServer(t)

	target := "/api/search?q=x&hub=" + url.QueryEscape("git@ghe.corp.com:team/skills.git")
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for unsaved SSH hub, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlePreview_CacheDoesNotOutliveHostAuthorization(t *testing.T) {
	t.Setenv("SKILLSHARE_CNB_HOSTS", "")
	t.Setenv("SKILLSHARE_GITEA_HOSTS", "")
	for _, provider := range []string{"cnb", "gitea"} {
		t.Run(provider, func(t *testing.T) {
			s, _ := newTestServer(t)
			if provider == "cnb" {
				s.cfg.CNBHosts = []string{"forge.corp.example"}
			} else {
				s.cfg.GiteaHosts = []string{"forge.corp.example"}
			}
			// This deliberately invalid source cannot trigger network I/O on a miss.
			source := "invalid-" + strings.ReplaceAll(t.Name(), "/", "-")
			key := previewKey(source, "main", s.parseOpts())
			previewCache.Store(key, &previewCacheEntry{
				data:      &search.SkillPreview{Content: "private cached skill"},
				expiresAt: time.Now().Add(time.Minute),
			})
			t.Cleanup(func() { previewCache.Delete(key) })
			target := "/api/preview?source=" + url.QueryEscape(source) + "&branch=main"
			fetch := func() *httptest.ResponseRecorder {
				rr := httptest.NewRecorder()
				s.handlePreview(rr, httptest.NewRequest(http.MethodGet, target, nil))
				return rr
			}
			if rr := fetch(); rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "private cached skill") {
				t.Fatalf("authorized cache hit = %d: %s", rr.Code, rr.Body.String())
			}
			s.cfg.CNBHosts = nil
			s.cfg.GiteaHosts = nil
			if rr := fetch(); rr.Code != http.StatusNotFound || strings.Contains(rr.Body.String(), "private cached skill") {
				t.Fatalf("revoked host authorization reused private preview: %d: %s", rr.Code, rr.Body.String())
			}
		})
	}
}
