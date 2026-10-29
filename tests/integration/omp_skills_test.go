//go:build !online

package integration

import (
	"fmt"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestSync_OMPAccountShellKeepsFilters(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	for _, name := range []string{"work", "personal"} {
		sb.CreateSkill(name, map[string]string{"SKILL.md": "---\nname: " + name + "\ndescription: Account isolation regression\n---\n# Skill\n"})
	}
	account := filepath.Join(sb.Home, ".omp-work", "agent")
	sb.WriteConfig(fmt.Sprintf("source: %q\ntargets:\n  omp: {}\n  omp-work:\n    agent: omp\n    config_dir: %q\n    skills:\n      include: [work]\n", sb.SourcePath, account))
	for i := 0; i < 2; i++ {
		sb.RunCLIEnv(map[string]string{"PI_CODING_AGENT_DIR": account}, "sync", "-g").AssertSuccess(t)
		for _, name := range []string{"work", "personal"} {
			if !sb.FileExists(filepath.Join(sb.Home, ".omp", "agent", "skills", name, "SKILL.md")) {
				t.Fatalf("default target missing %s", name)
			}
		}
		if !sb.FileExists(filepath.Join(account, "skills", "work", "SKILL.md")) {
			t.Fatal("account missing included skill")
		}
		if sb.FileExists(filepath.Join(account, "skills", "personal", "SKILL.md")) {
			t.Fatal("plain OMP target bypassed account filters")
		}
	}
}
