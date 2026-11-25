package install

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const mergeBaseSkill = "---\nname: alpha\n---\none\ntwo\nthree\nfour\nfive\n"

// installMergeFixture installs a skill from a fresh repo at raw (relative to
// the repo, e.g. "" or "//skills/alpha") and returns the repo, the skill
// directory and its source root. The skill lives at path inside the repo.
func installMergeFixture(t *testing.T, path, raw string) (repo, dest, sourceDir string) {
	t.Helper()
	repo = initTestRepo(t)
	writeRepoFile(t, repo, filepath.Join(path, "SKILL.md"), mergeBaseSkill)
	writeRepoFile(t, repo, filepath.Join(path, "old.md"), "old\n")
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "v1")

	sourceDir = t.TempDir()
	dest = filepath.Join(sourceDir, "alpha")
	source, err := ParseSource("file://" + repo + raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Install(source, dest, InstallOptions{SourceDir: sourceDir, SkipAudit: true}); err != nil {
		t.Fatal(err)
	}
	return repo, dest, sourceDir
}

func writeRepoFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readSkillFile(t *testing.T, dir, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(data), "\r", "")
}

// editSkillFile replaces old with new in a file of the installed skill, the
// way an editor would: it keeps the line endings, which a Windows checkout
// with core.autocrlf makes CRLF.
func editSkillFile(t *testing.T, dir, rel, old, new string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, old) {
		t.Fatalf("%s has no %q", rel, old)
	}
	writeRepoFile(t, dir, rel, strings.Replace(content, old, new, 1))
}

func mergeUpdate(t *testing.T, repo, raw, dest, sourceDir string) error {
	t.Helper()
	source, err := ParseSource("file://" + repo + raw)
	if err != nil {
		t.Fatal(err)
	}
	entry := LoadMetadataOrNew(sourceDir).Get("alpha")
	_, err = Install(source, dest, InstallOptions{SourceDir: sourceDir, Update: true, SkipAudit: true, MergeBase: entry})
	return err
}

func TestMergeUpdate_KeepsLocalEditsAndTakesUpstream(t *testing.T) {
	for _, layout := range []struct{ name, path, raw string }{
		{"whole repo", "", ""},
		{"subdir", "skills/alpha", "//skills/alpha"},
	} {
		t.Run(layout.name, func(t *testing.T) {
			repo, dest, sourceDir := installMergeFixture(t, layout.path, layout.raw)
			skillFile := filepath.Join(layout.path, "SKILL.md")

			editSkillFile(t, dest, "SKILL.md", "one", "one (mine)")
			writeRepoFile(t, dest, "notes.md", "my notes\n")

			editSkillFile(t, repo, skillFile, "five", "five (upstream)")
			writeRepoFile(t, repo, filepath.Join(layout.path, "new.md"), "new\n")
			if err := os.Remove(filepath.Join(repo, layout.path, "old.md")); err != nil {
				t.Fatal(err)
			}
			gitAdd(t, repo, ".")
			gitCommit(t, repo, "v2")

			if err := mergeUpdate(t, repo, layout.raw, dest, sourceDir); err != nil {
				t.Fatalf("merge: %v", err)
			}
			want := strings.NewReplacer("one", "one (mine)", "five", "five (upstream)").Replace(mergeBaseSkill)
			if got := readSkillFile(t, dest, "SKILL.md"); got != want {
				t.Errorf("SKILL.md =\n%s\nwant\n%s", got, want)
			}
			if got := readSkillFile(t, dest, "notes.md"); got != "my notes\n" {
				t.Errorf("notes.md = %q, want the local file kept", got)
			}
			if got := readSkillFile(t, dest, "new.md"); got != "new\n" {
				t.Errorf("new.md = %q, want the upstream file", got)
			}
			if _, err := os.Stat(filepath.Join(dest, "old.md")); !os.IsNotExist(err) {
				t.Errorf("old.md: err = %v, want it removed like upstream", err)
			}

			// The record holds the upstream version, so the skill still reads
			// as modified and the next update merges again instead of
			// overwriting the edits.
			entry := LoadMetadataOrNew(sourceDir).Get("alpha")
			if !entry.LocallyModified(dest) {
				t.Error("merged skill is not reported as modified")
			}

			editSkillFile(t, repo, skillFile, "three", "three (upstream)")
			gitAdd(t, repo, ".")
			gitCommit(t, repo, "v3")
			if err := mergeUpdate(t, repo, layout.raw, dest, sourceDir); err != nil {
				t.Fatalf("second merge: %v", err)
			}
			want = strings.Replace(want, "three", "three (upstream)", 1)
			if got := readSkillFile(t, dest, "SKILL.md"); got != want {
				t.Errorf("after second merge SKILL.md =\n%s\nwant\n%s", got, want)
			}
		})
	}
}

func TestMergeUpdate_ConflictLeavesSkillUntouched(t *testing.T) {
	repo, dest, sourceDir := installMergeFixture(t, "", "")
	editSkillFile(t, dest, "SKILL.md", "three", "three (mine)")
	editSkillFile(t, dest, "old.md", "old", "old (mine)")
	editSkillFile(t, repo, "SKILL.md", "three", "three (upstream)")
	if err := os.Remove(filepath.Join(repo, "old.md")); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "v2")

	beforeSkill := readSkillFile(t, dest, "SKILL.md")
	beforeMeta, err := os.ReadFile(filepath.Join(sourceDir, MetadataFileName))
	if err != nil {
		t.Fatal(err)
	}

	err = mergeUpdate(t, repo, "", dest, sourceDir)
	var conflict *MergeConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("err = %v, want a merge conflict", err)
	}
	if want := []string{"SKILL.md", "old.md"}; !slices.Equal(conflict.Files, want) {
		t.Errorf("conflict files = %v, want %v", conflict.Files, want)
	}
	if got := readSkillFile(t, dest, "SKILL.md"); got != beforeSkill {
		t.Errorf("SKILL.md changed on conflict:\n%s", got)
	}
	if got := readSkillFile(t, dest, "old.md"); got != "old (mine)\n" {
		t.Errorf("old.md = %q, want it kept on conflict", got)
	}
	afterMeta, err := os.ReadFile(filepath.Join(sourceDir, MetadataFileName))
	if err != nil {
		t.Fatal(err)
	}
	if string(afterMeta) != string(beforeMeta) {
		t.Error("metadata changed on conflict")
	}
}

// A commit that leaves the skill alone has nothing to merge: the files stay,
// and the record moves to the new version so check stops reporting it.
func TestMergeUpdate_UpstreamUnchangedIsUpToDate(t *testing.T) {
	repo, dest, sourceDir := installMergeFixture(t, "skills/alpha", "//skills/alpha")
	editSkillFile(t, dest, "SKILL.md", "one", "one (mine)")
	writeRepoFile(t, repo, "README.md", "elsewhere\n")
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "v2")
	before := LoadMetadataOrNew(sourceDir).Get("alpha").Version

	source, err := ParseSource("file://" + repo + "//skills/alpha")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Install(source, dest, InstallOptions{SourceDir: sourceDir, Update: true, SkipAudit: true, MergeBase: LoadMetadataOrNew(sourceDir).Get("alpha")})
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != "up-to-date" {
		t.Errorf("action = %q, want up-to-date", result.Action)
	}
	if got := readSkillFile(t, dest, "SKILL.md"); !strings.Contains(got, "one (mine)") {
		t.Errorf("SKILL.md lost the local edit:\n%s", got)
	}
	if entry := LoadMetadataOrNew(sourceDir).Get("alpha"); entry.Version == before || !entry.LocallyModified(dest) {
		t.Errorf("record = %+v, want the new version and still modified", entry)
	}
}

// A base that is not what was installed would turn upstream changes into
// apparent local ones, so the merge refuses instead of guessing.
func TestMergeUpdate_RefusesUnverifiableBase(t *testing.T) {
	repo, dest, sourceDir := installMergeFixture(t, "", "")
	editSkillFile(t, dest, "SKILL.md", "one", "one (mine)")
	editSkillFile(t, repo, "SKILL.md", "five", "five (upstream)")
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "v2")

	store := LoadMetadataOrNew(sourceDir)
	entry := store.Get("alpha")
	entry.FileHashes = maps.Clone(entry.FileHashes)
	entry.FileHashes["SKILL.md"] = "sha256:0000"
	source, err := ParseSource("file://" + repo)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Install(source, dest, InstallOptions{SourceDir: sourceDir, Update: true, SkipAudit: true, MergeBase: entry})
	if err == nil || !strings.Contains(err.Error(), "installed version") {
		t.Fatalf("err = %v, want a refusal naming the installed version", err)
	}
	if got := readSkillFile(t, dest, "SKILL.md"); !strings.Contains(got, "one (mine)") || strings.Contains(got, "upstream") {
		t.Errorf("SKILL.md changed:\n%s", got)
	}
}

func TestMergeFile(t *testing.T) {
	base := "a\nb\nc\nd\ne\n"
	for _, c := range []struct {
		name, ours, theirs, want string
		conflict                 bool
	}{
		{"separate lines", "A\nb\nc\nd\ne\n", "a\nb\nc\nd\nE\n", "A\nb\nc\nd\nE\n", false},
		{"same line", "a\nb\nC1\nd\ne\n", "a\nb\nC2\nd\ne\n", "", true},
		{"same change", "a\nB\nc\nd\ne\n", "a\nB\nc\nd\ne\n", "a\nB\nc\nd\ne\n", false},
		{"binary", "a\x00mine\n", "a\x00theirs\n", "", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, conflict, err := mergeFile([]byte(base), []byte(c.ours), []byte(c.theirs))
			if err != nil {
				t.Fatal(err)
			}
			if conflict != c.conflict || (!conflict && string(got) != c.want) {
				t.Errorf("mergeFile = %q, conflict %v; want %q, conflict %v", got, conflict, c.want, c.conflict)
			}
		})
	}
}

func TestMergeAgentUpdate(t *testing.T) {
	const base = "# Reviewer\none\ntwo\nthree\n"
	setup := func(t *testing.T) (repo, root, dest string, update func() (*InstallResult, error)) {
		repo = initTestRepo(t)
		writeRepoFile(t, repo, "agents/reviewer.md", base)
		gitAdd(t, repo, ".")
		gitCommit(t, repo, "v1")
		root = t.TempDir()
		discoverAgent := func() (*DiscoveryResult, AgentInfo) {
			t.Helper()
			source, err := ParseSource(fileURL(repo))
			if err != nil {
				t.Fatal(err)
			}
			discovery, err := DiscoverFromGit(source)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { CleanupDiscovery(discovery) })
			if len(discovery.Agents) != 1 {
				t.Fatalf("agents = %+v", discovery.Agents)
			}
			return discovery, discovery.Agents[0]
		}
		discovery, agent := discoverAgent()
		opts := InstallOptions{Kind: "agent", SourceDir: root, SkipAudit: true}
		if _, err := InstallAgentFromDiscovery(discovery, agent, root, opts); err != nil {
			t.Fatal(err)
		}
		dest = filepath.Join(root, "reviewer.md")
		update = func() (*InstallResult, error) {
			discovery, agent := discoverAgent()
			opts.MergeBase = LoadMetadataOrNew(root).Get("reviewer")
			return UpdateAgentFromDiscovery(discovery, agent, root, opts)
		}
		return repo, root, dest, update
	}

	t.Run("clean", func(t *testing.T) {
		repo, root, dest, update := setup(t)
		editSkillFile(t, root, "reviewer.md", "one", "one (mine)")
		editSkillFile(t, repo, "agents/reviewer.md", "three", "three (upstream)")
		gitAdd(t, repo, ".")
		gitCommit(t, repo, "v2")

		result, err := update()
		if err != nil {
			t.Fatal(err)
		}
		if result.Action != "merged" {
			t.Errorf("action = %q, want merged", result.Action)
		}
		if got, want := readSkillFile(t, root, "reviewer.md"), "# Reviewer\none (mine)\ntwo\nthree (upstream)\n"; got != want {
			t.Errorf("agent =\n%s\nwant\n%s", got, want)
		}
		if !LoadMetadataOrNew(root).Get("reviewer").AgentLocallyModified(dest) {
			t.Error("merged agent is not reported as modified")
		}
	})

	t.Run("conflict", func(t *testing.T) {
		repo, root, _, update := setup(t)
		editSkillFile(t, root, "reviewer.md", "two", "two (mine)")
		editSkillFile(t, repo, "agents/reviewer.md", "two", "two (upstream)")
		gitAdd(t, repo, ".")
		gitCommit(t, repo, "v2")

		_, err := update()
		var conflict *MergeConflictError
		if !errors.As(err, &conflict) || !slices.Equal(conflict.Files, []string{"reviewer.md"}) {
			t.Fatalf("err = %v, want a conflict in reviewer.md", err)
		}
		if got := readSkillFile(t, root, "reviewer.md"); got != "# Reviewer\none\ntwo (mine)\nthree\n" {
			t.Errorf("agent changed on conflict:\n%s", got)
		}
	})
}
