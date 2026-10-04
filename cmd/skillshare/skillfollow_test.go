package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/sourcewalk"
)

func TestSkillfollowSurface(t *testing.T) {
	root, ext := t.TempDir(), t.TempDir()
	if err := os.Symlink(ext, filepath.Join(root, "group")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bad"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".skillfollow.local"), []byte("group\nmissing\nbad\n../invalid\n"), 0644); err != nil {
		t.Fatal(err)
	}
	set := sourcewalk.Follow(root, sourcewalk.FollowOptions{})
	data := buildSkillfollowJSON(&set)
	if data.EntryCount != 3 || data.FollowedCount != 1 || data.SkippedCount != 2 || !data.LocalActive {
		t.Fatalf("%+v", data)
	}
	result := &doctorResult{}
	checkSkillfollow(result, &set)
	if len(result.checks) != 4 {
		t.Fatalf("%+v", result.checks)
	}
	for _, state := range []string{"followed", "missing", "invalid-target"} {
		found := false
		for _, check := range result.checks {
			if strings.Contains(check.Message, state) {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing %s: %+v", state, result.checks)
		}
	}
}

func TestSkillfollowAbsentOutput(t *testing.T) {
	if buildSkillfollowJSON(nil) != nil {
		t.Fatal("nil follow should omit JSON")
	}
	result := &doctorResult{}
	checkSkillfollow(result, nil)
	if len(result.checks) != 0 {
		t.Fatal(result.checks)
	}
	data, err := json.Marshal(statusJSONSource{Skillignore: buildSkillignoreJSON(nil)})
	if err != nil || strings.Contains(string(data), "skillfollow") {
		t.Fatalf("%s %v", data, err)
	}
}
