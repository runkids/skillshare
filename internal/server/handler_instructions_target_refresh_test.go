package server

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
)

func TestProjectTargetInstructions_ReloadPathAfterRemoteLock(t *testing.T) {
	s, root := newTestProjectServerWithExtras(t, []config.ExtraConfig{{
		Name: "rules", Remote: "example-org/ai-hub/rules", Targets: []config.ExtraTargetConfig{},
	}})
	s.projectCfg.Targets = []config.ProjectTargetEntry{{Name: "myagent",
		Skills:       &config.ResourceTargetConfig{Path: ".myagent/skills"},
		Instructions: &config.TargetInstructionsConfig{Path: ".myagent/before.md"},
	}}
	if err := s.projectCfg.Save(root); err != nil {
		t.Fatal(err)
	}
	if err := s.reloadConfig(); err != nil {
		t.Fatal(err)
	}
	before, after := filepath.Join(root, ".myagent/before.md"), filepath.Join(root, ".myagent/published.md")
	if err := os.MkdirAll(filepath.Dir(before), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(before, []byte("# Original\n"), 0644); err != nil {
		t.Fatal(err)
	}
	calls, releases := 0, 0
	old := lockRemoteExtraRoot
	lockRemoteExtraRoot = func(locked string) (func(), error) {
		calls++
		if locked != s.projectCfg.EffectiveExtrasSource(root) {
			t.Errorf("wrong root %s", locked)
		}
		fresh, err := config.LoadProjectReadOnly(root)
		if err != nil {
			return nil, err
		}
		fresh.Targets[0].Instructions.Path = ".myagent/published.md"
		if err := fresh.Save(root); err != nil {
			return nil, err
		}
		return func() { releases++ }, nil
	}
	t.Cleanup(func() { lockRemoteExtraRoot = old })
	rr := instructionsRequest(t, s, http.MethodPut, "/api/targets/myagent/instructions", `{"content":"# Updated\n"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("%d %s", rr.Code, rr.Body)
	}
	if calls != 1 || releases != 1 {
		t.Errorf("locks/releases=%d/%d want=1/1", calls, releases)
	}
	if data, err := os.ReadFile(after); err != nil || string(data) != "# Updated\n" {
		t.Errorf("published instruction path not used: %q %v", data, err)
	}
	if data, err := os.ReadFile(before); err != nil || string(data) != "# Original\n" {
		t.Errorf("stale instruction path changed: %q %v", data, err)
	}
}
