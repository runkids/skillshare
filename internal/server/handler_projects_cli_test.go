package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/testutil"
)

// This crosses the real CLI and dashboard adapters; use an explicitly rebuilt binary.
func TestProjectCLI_ConvertedDisabledSkills(t *testing.T) {
	if os.Getenv("SKILLSHARE_TEST_BINARY") == "" {
		t.Skip("requires SKILLSHARE_TEST_BINARY")
	}
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
	sb.CreateSkill("alpha", map[string]string{"SKILL.md": "---\nname: alpha\n---\n# Alpha\n"})
	root := filepath.Join(sb.Home, "app")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".claude", "skills")
	sb.RunCLI("target", "add", "app-claude", path, "--no-skills").AssertSuccess(t)
	cfg, err := config.LoadReadOnly()
	if err != nil {
		t.Fatal(err)
	}
	s := New(cfg, "127.0.0.1:0", "", "")
	body, _ := json.Marshal(map[string]string{"root": root})
	rr := projectRequest(t, s, http.MethodPost, "/api/projects/convert", string(body))
	if rr.Code != http.StatusOK {
		t.Fatalf("convert: %d %s", rr.Code, rr.Body)
	}
	before, err := os.ReadFile(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"list", "show"} {
		args := []string{"project", action}
		if action == "show" {
			args = append(args, "app")
		}
		r := sb.RunCLI(args...)
		r.AssertSuccess(t)
		r.AssertRowContains(t, "Skills", "off")
	}
	r := sb.RunCLI("project", "edit", "app", "--skills", "--dry-run", "--sync", "--json")
	r.AssertSuccess(t)
	var preview struct {
		Project struct {
			Skills map[string]any `json:"skills"`
		} `json:"project"`
		Applied  bool     `json:"applied"`
		Affected []string `json:"affected"`
		Sync     struct {
			Results []any `json:"results"`
		} `json:"sync"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &preview); err != nil {
		t.Fatal(err)
	}
	if len(preview.Project.Skills) != 3 {
		t.Errorf("skills JSON contract changed: %v", preview.Project.Skills)
	}
	if preview.Applied || len(preview.Affected) != 1 || len(preview.Sync.Results) != 1 {
		t.Errorf("enable preview: %s", r.Stdout)
	}
	after, err := os.ReadFile(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("preview persisted config")
	}
	if _, err := os.Lstat(filepath.Join(path, "alpha")); !os.IsNotExist(err) {
		t.Fatalf("preview wrote skill: %v", err)
	}
	r = sb.RunCLI("project", "edit", "app", "--skills", "--sync", "--json")
	r.AssertSuccess(t)
	cfg, err = config.LoadReadOnly()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Projects[root].Skills.IsEnabled() {
		t.Error("explicit --skills kept converted resource disabled")
	}
	if _, err := os.Lstat(filepath.Join(path, "alpha", "SKILL.md")); err != nil {
		t.Errorf("enabled sync did not send alpha: %v", err)
	}
	sb.RunCLI("project", "show", "app").AssertRowContains(t, "Skills", "on")
}
