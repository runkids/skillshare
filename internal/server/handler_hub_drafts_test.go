package server

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"skillshare/internal/hub"
)

func TestHubDraftAPI(t *testing.T) {
	s, _ := newTestServer(t)
	request := func(method, path string, body []byte, want int) []byte {
		t.Helper()
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, httptest.NewRequest(method, path, bytes.NewReader(body)))
		if rr.Code != want {
			t.Fatalf("%s %s: %d %s", method, path, rr.Code, rr.Body.String())
		}
		return rr.Body.Bytes()
	}
	raw := request("POST", "/api/hub/drafts/import", []byte(`{"schemaVersion":1,"skills":[{"name":"review","source":"acme/review"}]}`), 200)
	var response struct {
		Draft hub.Draft `json:"draft"`
	}
	json.Unmarshal(raw, &response)
	d := response.Draft
	path := "/api/hub/drafts/" + d.ID
	request("GET", path, nil, 200)
	request("GET", "/api/hub/drafts", nil, 200)
	request("POST", path+"/export", []byte(`{"revision":"stale"}`), 409)
	body, _ := json.Marshal(d)
	request("PUT", path, body, 200)
	request("PUT", path, body, 409)
	request("DELETE", path+"?revision="+d.Revision, nil, 409)
	raw = request("GET", path, nil, 200)
	json.Unmarshal(raw, &response)
	body, _ = json.Marshal(map[string]string{"revision": response.Draft.Revision})
	request("POST", path+"/export", body, 200)
	request("DELETE", path+"?revision="+response.Draft.Revision, nil, 200)
	request("GET", path, nil, 404)
	request("POST", "/api/hub/drafts/import", []byte(`{"schemaVersion":9,"skills":[]}`), 400)
}

func TestHubDraftProjectIsolation(t *testing.T) {
	s, _ := newTestServer(t)
	d, err := s.hubDraftStore().Create(hub.Draft{Name: "global"})
	if err != nil {
		t.Fatal(err)
	}
	globalDir := s.hubDraftStore().Dir
	s.projectRoot = t.TempDir()
	if s.hubDraftStore().Dir == globalDir {
		t.Fatal("project uses global draft directory")
	}
	if _, err := s.hubDraftStore().Get(d.ID); err == nil {
		t.Fatal("global draft visible in project")
	}
}
