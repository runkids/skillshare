package sync

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func TestCopyFingerprint_BinaryFileBoundaries(t *testing.T) {
	_, dst, m := fingerprintFixture(t)
	asset := filepath.Join(dst, "a")
	if err := os.WriteFile(asset, []byte("x\x00b\x00y"), 0600); err != nil {
		t.Fatal(err)
	}
	original, err := CopyOutputFingerprint(dst)
	if err != nil {
		t.Fatal(err)
	}
	m.Outputs = map[string]string{"repo-demo": original}
	if err := os.WriteFile(asset, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "b"), []byte("y"), 0600); err != nil {
		t.Fatal(err)
	}
	edited, err := CopyOutputFingerprint(dst)
	if err != nil {
		t.Fatal(err)
	}
	if original == edited {
		t.Error("different authored file boundaries share output identity")
	}
	if got := ClassifyEntry(EntryInput{Name: "repo-demo", Path: dst, Manifest: m}); got.Class != EntryModifiedCopy {
		t.Fatalf("binary edit classified as %s", got.Class)
	}
}

func TestCopyFingerprint_ProfileLegacyReproducibility(t *testing.T) {
	for _, rewritten := range []bool{false, true} {
		for _, output := range []string{"missing", "bare"} {
			for _, available := range []bool{false, true} {
				name := output
				if rewritten {
					name += "/rewritten"
				}
				if available {
					name += "/source"
				}
				t.Run(name, func(t *testing.T) {
					src, dst, m := fingerprintFixture(t)
					rewrite := "repo-demo"
					if !rewritten {
						rewrite = ""
						delete(m.Naming, "repo-demo")
						if err := copySkillToTarget(src, dst, "", DefaultFileIgnorePatterns()); err != nil {
							t.Fatal(err)
						}
					}
					if output == "bare" {
						old, err := DirChecksumWithIgnore(dst, DefaultFileIgnorePatterns())
						if err != nil {
							t.Fatal(err)
						}
						m.Outputs = map[string]string{"repo-demo": old}
					}
					input := EntryInput{Name: "repo-demo", Path: dst, Manifest: m}
					if available {
						input.Source = &CopySource{Path: src, OutputName: rewrite}
					}
					want := EntryModifiedCopy
					if available {
						want = EntryCleanCopy
					}
					if got := ClassifyEntry(input); got.Class != want {
						t.Errorf("profile legacy classification = %s, want %s", got.Class, want)
					}
					if !rewritten && !m.OwnsCopy("repo-demo", dst) {
						t.Error("source-free non-profile legacy compatibility changed")
					}
				})
			}
		}
	}
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
	t.Run("clean_edited_and_scratch_cleanup", func(t *testing.T) {
		src, dst, m := fingerprintFixture(t)
		scratch := t.TempDir()
		t.Setenv("TMPDIR", scratch)
		t.Setenv("TMP", scratch)
		t.Setenv("TEMP", scratch)
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
	})
	t.Run("changed_source", func(t *testing.T) {
		src, dst, m := fingerprintFixture(t)
		if err := os.WriteFile(filepath.Join(src, "new"), []byte("changed"), 0644); err != nil {
			t.Fatal(err)
		}
		if m.OwnsCopy("repo-demo", dst, CopySource{Path: src, OutputName: "repo-demo"}) {
			t.Fatal("legacy copy with changed source recognized as clean")
		}
	})
	t.Run("custom_ignore", func(t *testing.T) {
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
	})
	t.Run("reverted_name", func(t *testing.T) {
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
	})
	t.Run("replay_failure", func(t *testing.T) {
		src, dst, m := fingerprintFixture(t)
		badTemp := filepath.Join(t.TempDir(), "missing")
		t.Setenv("TMPDIR", badTemp)
		t.Setenv("TMP", badTemp)
		t.Setenv("TEMP", badTemp)
		if m.OwnsCopy("repo-demo", dst, CopySource{Path: src, OutputName: "repo-demo"}) {
			t.Fatal("copy verified without replay scratch")
		}
	})
	t.Run("plain_noise_and_mode", func(t *testing.T) {
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
	})
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

// Fixed vectors use the published byte framing, independently of the encoder.
func TestCopyFingerprint_V2Encoding(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"empty", nil, "f99dda8b71f65e5f6f4d8703f2f9d92c5438456878750bbda702f350cfc2f28d"},
		{"binary", map[string]string{"a": "x\x00b\x00y"}, "a57aa54b104007548fea118c36331ec685da6717cbac63a8d26832cad195d921"},
		{"split", map[string]string{"b": "y", "a": "x"}, "2372c950d10181d60c151e52fe33b25af5a7e19961a9c5017a569925ba9d6d0f"},
		{"nested", map[string]string{"a/b": "x"}, "ef8bb51afbafa9cfc9f64e9bce68f408d7e2cb58f686d33df2d9d21e70b8851c"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tc.files {
				path := filepath.Join(dir, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := CopyOutputFingerprint(dir)
			if err != nil || got != "sha256-v2:"+tc.want {
				t.Fatalf("framed output = %q, %v", got, err)
			}
			if runtime.GOOS != "windows" && tc.name == "nested" {
				literal := t.TempDir()
				if err := os.WriteFile(filepath.Join(literal, `a\b`), []byte("x"), 0600); err != nil {
					t.Fatal(err)
				}
				got, err := CopyOutputFingerprint(literal)
				if err != nil || got != "sha256-v2:e7dae9a1f26ade6e33a1b3a1b6743a0977ffb57e90562decc9d6ab58a67510ab" {
					t.Fatalf("literal backslash spelling lost: %q, %v", got, err)
				}
			}
		})
	}
}

func TestCopyFingerprint_EncodingMigration(t *testing.T) {
	for _, output := range []string{"missing", "84226a8f0c1e11bd61bdeecf916cc03c7f0982bc28a2b9a86e26da6ff547ad2a", "", "sha256-v3:" + strings.Repeat("0", 64), "sha256-v2:bad", "sha256-v2:" + strings.Repeat("0", 64), strings.Repeat("0", 64), "SHA256-V2:" + strings.Repeat("0", 64), strings.Repeat("A", 64)} {
		t.Run(output, func(t *testing.T) {
			src, dst, _ := fingerprintFixture(t)
			// A real pre-v2 record: source and rewritten-output legacy digests.
			data := `{"managed":{"repo-demo":"016b7454b31b926d56996bd534e1718a613c4dacca999b0deec3643bcd4f0b7b"},"naming":{"repo-demo":"prefixed"}}`
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, ManifestFile), []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			m, err := ReadManifest(dir)
			if err != nil {
				t.Fatal(err)
			}
			if output != "missing" {
				m.Outputs["repo-demo"] = output
			}
			input := CopySource{Path: src, OutputName: "repo-demo"}
			wantClean := output == "missing" || output == "84226a8f0c1e11bd61bdeecf916cc03c7f0982bc28a2b9a86e26da6ff547ad2a"
			entry := ClassifyEntry(EntryInput{Name: "repo-demo", Path: dst, Manifest: m, Source: &input})
			if (entry.Class == EntryCleanCopy) != wantClean {
				t.Fatalf("encoding classified as %s", entry.Class)
			}
			before, present := m.Outputs["repo-demo"]
			backfillCopyOutput(m, "repo-demo", dst, input, true)
			if got, ok := m.Outputs["repo-demo"]; got != before || ok != present {
				t.Fatal("preview promoted output")
			}
			backfillCopyOutput(m, "repo-demo", dst, input, false)
			if wantClean {
				if !strings.HasPrefix(m.Outputs["repo-demo"], "sha256-v2:") {
					t.Fatal("verified legacy output not promoted")
				}
				// The prior reader compares the whole value with its bare digest.
				if m.Outputs["repo-demo"] == "84226a8f0c1e11bd61bdeecf916cc03c7f0982bc28a2b9a86e26da6ff547ad2a" {
					t.Fatal("tagged output accepted by old-reader equality")
				}
			} else if m.Outputs["repo-demo"] != before {
				t.Fatal("unknown output downgraded by backfill")
			}
		})
	}
}
