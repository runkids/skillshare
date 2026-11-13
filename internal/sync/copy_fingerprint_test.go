package sync

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
)

func fingerprintFixture(t *testing.T) (string, string, *Manifest) {
	t.Helper()
	src, dst := filepath.Join(t.TempDir(), "source"), filepath.Join(t.TempDir(), "output")
	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("---\nname: demo\n---\n# Demo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := copySkillToTarget(src, dst, "repo-demo", DefaultFileIgnorePatterns()); err != nil {
		t.Fatal(err)
	}
	sum, err := DirChecksumWithIgnore(src, DefaultFileIgnorePatterns())
	if err != nil {
		t.Fatal(err)
	}
	return src, dst, &Manifest{Managed: map[string]string{"repo-demo": sum}, Naming: map[string]string{"repo-demo": "prefixed"}}
}

func TestCopyFingerprint_OutputAndEdits(t *testing.T) {
	_, dst, m := fingerprintFixture(t)
	sum, err := CopyOutputFingerprint(dst)
	if err != nil {
		t.Fatal(err)
	}
	m.Outputs = map[string]string{"repo-demo": sum}
	if !m.OwnsCopy("repo-demo", dst) {
		t.Fatal("fresh rewritten copy not recognized")
	}
	if err := os.WriteFile(filepath.Join(dst, "SKILL.md"), []byte("---\nname: edited\n---\n# Demo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if m.OwnsCopy("repo-demo", dst) {
		t.Fatal("edited generated name recognized as clean")
	}
}

func TestCopyFingerprint_LegacyReplay(t *testing.T) {
	src, dst, m := fingerprintFixture(t)
	scratch := t.TempDir()
	t.Setenv("TMPDIR", scratch)
	if !m.OwnsCopy("repo-demo", dst, CopySource{Path: src, OutputName: "repo-demo"}) {
		t.Fatal("unchanged legacy rewritten copy not recognized")
	}
	entries, err := os.ReadDir(scratch)
	if err != nil || len(entries) != 0 {
		t.Fatalf("replay scratch retained: %v, %v", entries, err)
	}
	if err := os.WriteFile(filepath.Join(dst, "note"), []byte("edit"), 0644); err != nil {
		t.Fatal(err)
	}
	if m.OwnsCopy("repo-demo", dst, CopySource{Path: src, OutputName: "repo-demo"}) {
		t.Fatal("edited legacy copy recognized as clean")
	}
}

func TestCopyFingerprint_LegacyChangedSource(t *testing.T) {
	src, dst, m := fingerprintFixture(t)
	if err := os.WriteFile(filepath.Join(src, "new"), []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	if m.OwnsCopy("repo-demo", dst, CopySource{Path: src, OutputName: "repo-demo"}) {
		t.Fatal("legacy copy with changed source recognized as clean")
	}
}

func TestCopyFingerprint_LegacyReplayWithCustomIgnore(t *testing.T) {
	src, dst, m := fingerprintFixture(t)
	if err := os.WriteFile(filepath.Join(src, "private.tmp"), []byte("ignored"), 0644); err != nil {
		t.Fatal(err)
	}
	ignore := append(DefaultFileIgnorePatterns(), "private.tmp")
	sum, err := DirChecksumWithIgnore(src, ignore)
	if err != nil {
		t.Fatal(err)
	}
	m.Managed["repo-demo"] = sum
	if !m.OwnsCopy("repo-demo", dst, CopySource{Path: src, OutputName: "repo-demo", IgnorePatterns: ignore}) {
		t.Fatal("legacy replay did not use the copy's ignore patterns")
	}
}

func TestCopyFingerprint_OldManifestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ManifestFile), []byte(`{"managed":{"old":"source"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := ReadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	renameManifestEntry(m, "old", "new", "flat")
	if err := WriteManifest(dir, m); err != nil {
		t.Fatal(err)
	}
	m, err = ReadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Managed["new"] != "source" || len(m.Outputs) != 0 {
		t.Fatalf("legacy manifest round trip: %+v", m)
	}
}

func TestCopyFingerprint_LegacyRevertedNameIsEdited(t *testing.T) {
	src, dst, m := fingerprintFixture(t)
	data, err := os.ReadFile(filepath.Join(src, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "SKILL.md"), data, 0644); err != nil {
		t.Fatal(err)
	}
	if m.OwnsCopy("repo-demo", dst, CopySource{Path: src, OutputName: "repo-demo"}) {
		t.Fatal("reverted generated name recognized as clean")
	}
}

func TestCopyFingerprint_LegacyReplayFailure(t *testing.T) {
	src, dst, m := fingerprintFixture(t)
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	if m.OwnsCopy("repo-demo", dst, CopySource{Path: src, OutputName: "repo-demo"}) {
		t.Fatal("copy verified without replay scratch")
	}
}

func TestCopyFingerprint_LegacyPlainAndNoise(t *testing.T) {
	src, dst, m := fingerprintFixture(t)
	if err := os.RemoveAll(dst); err != nil {
		t.Fatal(err)
	}
	if err := copySkillToTarget(src, dst, "", DefaultFileIgnorePatterns()); err != nil {
		t.Fatal(err)
	}
	if !m.OwnsCopy("repo-demo", dst) {
		t.Fatal("unrewritten legacy copy not recognized")
	}
	if err := os.WriteFile(filepath.Join(dst, ".DS_Store"), []byte("noise"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dst, "SKILL.md"), 0755); err != nil {
		t.Fatal(err)
	}
	if !m.OwnsCopy("repo-demo", dst) {
		t.Fatal("ignored noise or mode changed ownership")
	}
}

func TestCopyFingerprint_RenameMovesOutput(t *testing.T) {
	m := &Manifest{Managed: map[string]string{"old": "source"}, Outputs: map[string]string{"old": "output"}}
	renameManifestEntry(m, "old", "new", "flat")
	if m.Outputs["new"] != "output" || m.Outputs["old"] != "" {
		t.Fatalf("rename lost fingerprint: %+v", m.Outputs)
	}
	renameManifestEntry(m, "new", "old", "flat")
	if m.Outputs["old"] != "output" || m.Outputs["new"] != "" {
		t.Fatalf("rollback lost fingerprint: %+v", m.Outputs)
	}
	m.Remove("old")
	if len(m.Outputs) != 0 {
		t.Fatal("remove retained fingerprint")
	}
}

func TestCopyFingerprint_BackfillOnlyVerified(t *testing.T) {
	src, dst, m := fingerprintFixture(t)
	// Exercise the same backfill helper used by both skip paths.
	backfillCopyOutput(m, "repo-demo", dst, CopySource{Path: src, OutputName: "repo-demo"}, true)
	if len(m.Outputs) != 0 {
		t.Fatal("dry run recorded fingerprint")
	}
	backfillCopyOutput(m, "repo-demo", dst, CopySource{Path: src, OutputName: "repo-demo"}, false)
	if m.Outputs["repo-demo"] == "" {
		t.Fatal("verified legacy copy was not recorded")
	}
}

func TestCopyFingerprint_CopySyncPaths(t *testing.T) {
	source, targetDir := t.TempDir(), filepath.Join(t.TempDir(), "target")
	skill := filepath.Join(source, "demo")
	if err := os.MkdirAll(skill, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: demo\n---\n# Demo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	target := config.TargetConfig{Path: targetDir}
	run := func(dry, force bool) {
		t.Helper()
		if _, err := SyncTargetCopy("test", target, source, dry, force); err != nil {
			t.Fatal(err)
		}
	}
	read := func() *Manifest {
		t.Helper()
		m, err := ReadManifest(targetDir)
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	clear := func(m *Manifest, clearMtime bool) {
		t.Helper()
		delete(m.Outputs, "demo")
		if clearMtime {
			delete(m.Mtimes, "demo")
		}
		if err := WriteManifest(targetDir, m); err != nil {
			t.Fatal(err)
		}
	}
	entry := filepath.Join(targetDir, "demo")
	run(false, false)
	m := read()
	if m.Outputs["demo"] == "" || !m.OwnsCopy("demo", entry) {
		t.Fatal("new copy lacks output fingerprint")
	}
	clear(m, false)
	run(true, false)
	if read().Outputs["demo"] != "" {
		t.Fatal("dry run backfilled output")
	}
	run(false, false)
	if read().Outputs["demo"] == "" {
		t.Fatal("mtime skip did not backfill clean copy")
	}
	clear(read(), true)
	run(false, false)
	if read().Outputs["demo"] == "" {
		t.Fatal("checksum skip did not backfill clean copy")
	}
	clear(read(), false)
	if err := os.WriteFile(filepath.Join(entry, "local"), []byte("edit"), 0644); err != nil {
		t.Fatal(err)
	}
	run(false, false)
	if read().Outputs["demo"] != "" {
		t.Fatal("edited legacy copy was backfilled")
	}
	run(false, true)
	if !read().OwnsCopy("demo", entry) {
		t.Fatal("forced refresh did not record output")
	}
	if err := os.RemoveAll(entry); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte("file"), 0644); err != nil {
		t.Fatal(err)
	}
	run(false, false)
	if !read().OwnsCopy("demo", entry) {
		t.Fatal("non-directory replacement did not record output")
	}
}
