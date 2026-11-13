package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
)

func TestConfigRecovery_UnparseableOriginAdmissionAndCorrection(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprintf("project=%t", project), func(t *testing.T) {
			var s *Server
			if project {
				s, _ = newTestProjectServerWithExtras(t, nil)
			} else {
				s, _ = newTestServer(t)
			}
			before, err := os.ReadFile(s.configPath())
			if err != nil {
				t.Fatal(err)
			}
			raw := func(remote string) string {
				text := "extras: [{name: rules, remote: " + remote + ", targets: []}]\n"
				if !project {
					text = fmt.Sprintf("source: %q\n", s.cfg.Source) + text
				}
				return text
			}
			put := func(text string) int {
				body, err := json.Marshal(map[string]string{"raw": text})
				if err != nil {
					t.Fatal(err)
				}
				return serveExtras(s, http.MethodPut, "/api/config", string(body)).Code
			}
			if status := put(raw("nonsense")); status != http.StatusBadRequest {
				t.Errorf("unparseable origin PUT status=%d, want400", status)
			}
			if got, err := os.ReadFile(s.configPath()); err != nil || string(got) != string(before) {
				t.Errorf("refused origin persisted: %s %v", got, err)
			}
			// A file already admitted by the old implementation must remain
			// correctable through the supported editor, not become an identity.
			if err := os.WriteFile(s.configPath(), []byte(raw("nonsense")), 0644); err != nil {
				t.Fatal(err)
			}
			if status := put(raw("example-org/ai-hub")); status != http.StatusOK {
				t.Errorf("invalid baseline correction status=%d, want200", status)
			}
		})
	}
}
