package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"skillshare/internal/config"
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
