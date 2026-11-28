package main

import (
	"strings"
	"testing"

	"skillshare/internal/skillignore"
)

// Skills in one folder are counted under it instead of listed one per line.
func TestPrintIgnoredSkills_CountsSkillsPerFolder(t *testing.T) {
	stats := &skillignore.IgnoreStats{RootFile: ".skillignore", IgnoredSkills: []string{
		"_pack/skills/a", "_pack/skills/b", "_pack/skills/c", "loose", "group/only",
	}}
	out := captureStdoutStderr(t, func() { printIgnoredSkills(stats) })

	for _, want := range []string{"5 skill(s) ignored by .skillignore", "_pack (3)", "loose", "group/only", "skillshare list --status disabled"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "_pack/skills/a") {
		t.Errorf("skills in a folder are still listed one by one:\n%s", out)
	}
}
