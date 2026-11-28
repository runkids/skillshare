package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// The dashboard shows display_name when set: a claude.ai skill in a Claude target
// reads "skill-creator (claude.ai)", while name keeps the full identity.
func TestHandleAnalyze_LabelsClaudeAISkills(t *testing.T) {
	claudeDir := filepath.Join(t.TempDir(), "claude")
	s, src := newTestServerWithTargets(t, map[string]string{"claude": claudeDir})
	addSkill(t, src, "skill-creator")
	synced := filepath.Join(claudeDir, "synced", "org_user", "skill-creator")
	os.MkdirAll(synced, 0755)
	os.WriteFile(filepath.Join(synced, "SKILL.md"), []byte("---\nname: skill-creator\ndescription: d\n---\nbody"), 0644)

	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/analyze", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Targets []analyzeTargetResponse `json:"targets"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, tgt := range resp.Targets {
		for _, sk := range tgt.Skills {
			got[sk.Name] = sk.DisplayName
		}
	}
	want := map[string]string{"synced__org_user__skill-creator": "skill-creator (claude.ai)", "skill-creator": ""}
	for name, label := range want {
		if l, ok := got[name]; !ok || l != label {
			t.Errorf("skill %q: display_name = %q (present %v), want %q; all: %v", name, l, ok, label, got)
		}
	}
}
