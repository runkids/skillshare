package install

import (
	"strings"
	"testing"
)

func TestLockCommitFor(t *testing.T) {
	const sha = "8f14e45fceea167a5a36dedd4bea2543ce848564"
	lock := &Lock{Skills: map[string]LockEntry{
		"ok":     {Source: "src", Commit: sha},
		"moved":  {Source: "old-src", Commit: sha},
		"short":  {Source: "src", Commit: sha[:12]},
		"option": {Source: "src", Commit: "--upload-pack=touch /tmp/pwned"},
	}}
	want := map[string]string{"ok": sha, "moved": "", "short": "", "option": "", "absent": ""}
	for name, commit := range want {
		if got := lock.CommitFor(name, "src"); got != commit {
			t.Errorf("CommitFor(%q) = %q, want %q", name, got, commit)
		}
	}
	if got := (*Lock)(nil).CommitFor("ok", "src"); got != "" {
		t.Errorf("nil lock returned %q", got)
	}
	lock.Extras = map[string]LockEntry{"ok": {Source: "remote", Commit: sha}}
	if lock.ExtraCommitFor("ok", "remote") != sha || lock.ExtraCommitFor("ok", "other") != "" || lock.ExtraCommitFor("absent", "remote") != "" || (*Lock)(nil).ExtraCommitFor("ok", "remote") != "" {
		t.Fatal("extra lookup lost identity or nil/absent refusal")
	}
	for _, commit := range []string{sha[:7], "--upload-pack=synthetic", strings.Repeat("z", 40)} {
		lock.SetExtraPin("invalid", LockEntry{Source: "remote", Commit: commit})
		if lock.ExtraCommitFor("invalid", "remote") != "" {
			t.Fatalf("invalid extra pin accepted: %q", commit)
		}
	}
}

func TestLockMovePin(t *testing.T) {
	lock := &Lock{Skills: map[string]LockEntry{"demo": {Source: "src", Commit: "abc"}}}

	if !lock.MovePin("demo", "grp/demo") {
		t.Fatal("MovePin reported no pin for a pinned skill")
	}
	if _, ok := lock.Skills["demo"]; ok {
		t.Error("pin stayed at the old name")
	}
	if got := lock.Skills["grp/demo"]; got.Commit != "abc" || got.Source != "src" {
		t.Errorf("pin at the new name = %+v, want it carried unchanged", got)
	}
	if lock.MovePin("absent", "grp/absent") {
		t.Error("MovePin reported a pin for a skill that has none")
	}
}

func TestExtraPinsSurviveSkillPinChanges(t *testing.T) {
	dir := t.TempDir()
	sha := strings.Repeat("a", 40)
	lock := &Lock{Skills: map[string]LockEntry{"old": {Source: "skill", Commit: sha}, "rules": {Source: "same-name-skill", Commit: strings.Repeat("b", 40)}}}
	lock.SetExtraPin("rules", LockEntry{Source: "remote", Commit: sha})
	if err := lock.Save(dir); err != nil {
		t.Fatal(err)
	}
	if err := MoveLockPins(dir, map[string]string{"old": "new"}); err != nil {
		t.Fatal(err)
	}
	got, err := LoadLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.ExtraCommitFor("rules", "remote") != sha || got.ExtraCommitFor("rules", "other") != "" {
		t.Fatalf("bad extra pin: %+v", got.Extras)
	}
	if got.CommitFor("rules", "same-name-skill") != strings.Repeat("b", 40) || got.ExtraCommitFor("rules", "remote") != sha {
		t.Fatal("same-named skill and extra pins collided")
	}
	delete(got.Skills, "rules")
	delete(got.Skills, "new")
	if err := got.Save(dir); err != nil {
		t.Fatal(err)
	}
	got, err = LoadLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 1 || got.ExtraCommitFor("rules", "remote") != sha {
		t.Fatalf("lost extra pin: %+v", got)
	}
	if !got.DropExtraPin("rules") || got.DropExtraPin("rules") {
		t.Fatal("unexpected drop result")
	}
}
