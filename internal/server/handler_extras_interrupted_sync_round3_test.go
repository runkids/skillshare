package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"skillshare/internal/config"
)

func TestExtrasSync_InterruptedVisibilityAPI(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, named := range []bool{false, true} {
			t.Run(fmt.Sprintf("project=%t/named=%t", project, named), func(t *testing.T) {
				entry := config.ExtraConfig{Name: "rules", Remote: "file:///fixture-unavailable-round3", Branch: "main", Targets: []config.ExtraTargetConfig{{Path: t.TempDir(), Mode: "copy"}}}
				var s *Server
				if project {
					s, _ = newTestProjectServerWithExtras(t, []config.ExtraConfig{entry})
				} else {
					s, _ = newTestServerWithExtras(t, []config.ExtraConfig{entry}, "")
				}
				source := s.extrasSourceDir(entry)
				if err := os.WriteFile(filepath.Join(source, "rule.md"), []byte("candidate at live boundary"), 0644); err != nil {
					t.Fatal(err)
				}
				stage := filepath.Join(filepath.Dir(source), ".skillshare-staging")
				if err := os.MkdirAll(stage, 0700); err != nil {
					t.Fatal(err)
				}
				marker := filepath.Join(stage, "publishing-rules.json")
				data, err := json.Marshal(map[string]any{"version": 1, "name": "rules", "remote": entry.Remote, "branch": "main", "commit": strings.Repeat("a", 40), "had_previous": false, "old": nil})
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(marker, data, 0600); err != nil {
					t.Fatal(err)
				}
				body := `{}`
				if named {
					body = `{"name":"rules"}`
				}
				rr := serveExtras(s, http.MethodPost, "/api/extras/sync", body)
				var decoded struct {
					Extras []extraSyncResult `json:"extras"`
					Error  string            `json:"error"`
				}
				if err := json.Unmarshal(rr.Body.Bytes(), &decoded); err != nil {
					t.Fatal(err)
				}
				messages := decoded.Error
				for _, entry := range decoded.Extras {
					messages += strings.Join(entry.Warnings, "\n")
					for _, target := range entry.Targets {
						messages += target.Error + strings.Join(target.Errors, "\n") + strings.Join(target.Warnings, "\n")
					}
				}
				if !regexp.MustCompile(`(?i)\binterrupted\b`).MatchString(messages) {
					t.Errorf("API hides interrupted provenance: %d %s", rr.Code, rr.Body.String())
				}
				if after, err := os.ReadFile(marker); err != nil || string(after) != string(data) {
					t.Errorf("ordinary API sync changed marker: %q %v", after, err)
				}
			})
		}
	}
}
