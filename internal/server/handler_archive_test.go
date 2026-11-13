package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveDashboardRoundTrip(t *testing.T) {
	var payload bytes.Buffer
	z := zip.NewWriter(&payload)
	f, _ := z.Create("brief/SKILL.md")
	_, _ = f.Write([]byte("# Brief"))
	_ = z.Close()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(payload.Bytes()) }))
	defer srv.Close()
	old := http.DefaultTransport
	http.DefaultTransport = srv.Client().Transport
	defer func() { http.DefaultTransport = old }()
	for _, project := range []bool{false, true} {
		s, _ := newUpdateFollowServer(t, project, false)
		raw := srv.URL + "/pack.zip"
		for _, endpoint := range []string{"discover", "install"} {
			body, _ := json.Marshal(map[string]string{"source": raw})
			rr := httptest.NewRecorder()
			s.mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/"+endpoint, bytes.NewReader(body)))
			if rr.Code != http.StatusOK {
				t.Fatalf("%s: %d %s", endpoint, rr.Code, rr.Body)
			}
		}
		for _, stream := range []bool{false, true} {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/update", strings.NewReader(`{"name":"pack"}`))
			if stream {
				req.URL.Path = "/api/update/stream"
				s.handleUpdateStream(rr, req)
			} else {
				s.handleUpdate(rr, req)
			}
			results := updateResults(t, rr, stream)
			if len(results) != 1 || results[0].Action != "skipped" {
				t.Fatalf("no-op response = %+v (%s)", results, rr.Body)
			}
		}
		if err := os.WriteFile(filepath.Join(s.skillsSource(), "pack", "SKILL.md"), []byte("# Mine"), 0600); err != nil {
			t.Fatal(err)
		}
		rr := httptest.NewRecorder()
		s.mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/resources/pack/local-changes", nil))
		var changes struct {
			Files []struct{ Path, Status, Diff string }
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &changes); err != nil {
			t.Fatal(err)
		}
		if rr.Code != http.StatusOK || len(changes.Files) != 1 || changes.Files[0].Status != "modified" || changes.Files[0].Diff != "" {
			t.Fatalf("ZIP changes endpoint: %d %s", rr.Code, rr.Body)
		}

	}
}
