package update

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/audit"
	"skillshare/internal/install"
	"skillshare/internal/testutil"
)

// Confirmation is a deterministic barrier: the scan has finished, but no
// candidate may be published until the callback approves it.
func TestTrackedRepo_CandidateRemainsPrivate(t *testing.T) {
	for _, approve := range []bool{false, true} {
		name := "reject"
		if approve {
			name = "approve-exact-revision"
		}
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, maliciousSkill)
			approved := testutil.RunGit(t, f.seed, "rev-parse", "HEAD")
			link := filepath.Join(t.TempDir(), "published")
			if err := os.Symlink(f.repo, link); err != nil {
				t.Skip(err)
			}
			store := install.NewMetadataStore()
			store.Set("_team", &install.MetadataEntry{Source: "local", Tracked: true})
			if err := store.Save(f.source); err != nil {
				t.Fatal(err)
			}
			metadata := filepath.Join(f.source, install.MetadataFileName)
			beforeMetadata, err := os.ReadFile(metadata)
			if err != nil {
				t.Fatal(err)
			}
			assertOld := func() {
				t.Helper()
				content, err := os.ReadFile(filepath.Join(link, "SKILL.md"))
				if err != nil || string(content) != cleanSkill {
					t.Errorf("published link exposed candidate: %q, %v", content, err)
				}
				if f.head(t) != f.before {
					t.Error("active HEAD changed before approval")
				}
				if got, err := os.ReadFile(metadata); err != nil || string(got) != string(beforeMetadata) {
					t.Errorf("metadata changed before publication: %q, %v", got, err)
				}
			}
			opts := f.opts()
			asked := false
			opts.Confirm = func(pending *TrackedRepoResult) (bool, error) {
				if len(pending.Changes) != 1 || pending.Changes[0].Path != "SKILL.md" {
					t.Errorf("candidate diff unavailable during confirmation: %+v", pending.Changes)
				}
				asked = true
				assertOld()
				// A second network pull during publication would publish this
				// unscanned revision instead of the approved candidate.
				f.push(t, maliciousSkill+"A later, unscanned revision.\n")
				return approve, nil
			}
			_, err = TrackedRepo(f.repo, opts)
			if !asked {
				t.Fatal("confirmation barrier was not reached")
			}
			if approve {
				if err != nil {
					t.Fatal(err)
				}
				if f.head(t) != approved {
					t.Error("published revision differs from the audited revision")
				}
				return
			}
			if !errors.Is(err, audit.ErrBlocked) {
				t.Fatalf("expected audit rejection, got %v", err)
			}
			assertOld()
		})
	}
}

func TestTrackedRepo_CandidateSymlinkTargets(t *testing.T) {
	for _, absolute := range []bool{false, true} {
		t.Run(map[bool]string{false: "relative", true: "absolute"}[absolute], func(t *testing.T) {
			f := newFixture(t, cleanSkill+"Safe update.\n")
			if err := os.WriteFile(filepath.Join(f.seed, "payload.dat"), []byte(maliciousSkill), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(filepath.Join(f.seed, "SKILL.md")); err != nil {
				t.Fatal(err)
			}
			target := "../_team/payload.dat"
			if absolute {
				target = filepath.Join(f.repo, "payload.dat")
			}
			if err := os.Symlink(target, filepath.Join(f.seed, "SKILL.md")); err != nil {
				t.Skip(err)
			}
			testutil.RunGit(t, f.seed, "add", "-A")
			testutil.RunGit(t, f.seed, "commit", "-m", "symlink skill to payload")
			testutil.RunGit(t, f.seed, "push", "origin", "HEAD:main")
			_, err := TrackedRepo(f.repo, f.opts())
			if err == nil || f.head(t) != f.before {
				t.Fatalf("symlink payload was published: %v", err)
			}
		})
	}
}

func TestTrackedRepo_CandidatePreservesIntegrityHashes(t *testing.T) {
	f := newFixture(t, cleanSkill+"Changed pinned content.\n")
	store := install.NewMetadataStore()
	store.Set("_team", &install.MetadataEntry{Source: "local", Tracked: true})
	if _, err := store.RefreshTrackedRootSkillHashes("_team", f.repo); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(f.source); err != nil {
		t.Fatal(err)
	}
	opts := f.opts()
	opts.Threshold = "medium"
	res, err := TrackedRepo(f.repo, opts)
	if !errors.Is(err, audit.ErrBlocked) || f.head(t) != f.before {
		t.Fatalf("pinned change was published: %+v, %v", res, err)
	}
	found := false
	for _, finding := range res.Audit.Findings {
		found = found || finding.RuleID == "content-tampered"
	}
	if !found {
		t.Fatal("candidate did not retain content integrity findings")
	}
}

func TestTrackedRepo_CandidateFromShallowClone(t *testing.T) {
	f := newFixture(t, cleanSkill+"Initial help.\n")
	remote := testutil.RunGit(t, f.repo, "remote", "get-url", "origin")
	shallow := filepath.Join(f.source, "_shallow")
	testutil.RunGit(t, "", "clone", "--depth=1", "file://"+remote, shallow)
	f.push(t, cleanSkill+"Later help.\n")
	res, err := TrackedRepo(shallow, f.opts())
	if err != nil || res.Status != StatusUpdated {
		t.Fatalf("shallow update = %+v, %v", res, err)
	}
	if got := testutil.RunGit(t, shallow, "rev-parse", "HEAD"); got != res.Info.AfterHash {
		t.Error("published shallow revision differs from candidate")
	}
}

// Partial clones lack intermediate blobs, which git >= 2.45.1 will not
// lazy-fetch through a local upload-pack by default.
func TestTrackedRepo_CandidateFromPartialClone(t *testing.T) {
	f := newFixture(t, cleanSkill+"Initial help.\n")
	remote := testutil.RunGit(t, f.repo, "remote", "get-url", "origin")
	testutil.RunGit(t, remote, "config", "uploadpack.allowFilter", "true")
	partial := filepath.Join(f.source, "_partial")
	testutil.RunGit(t, "", "clone", "--depth=1", "--filter=blob:none", "file://"+remote, partial)
	// A plain pull, as before 0.27.0, leaves the intermediate blob missing.
	f.push(t, cleanSkill+"First help.\n")
	f.push(t, cleanSkill+"Second help.\n")
	testutil.RunGit(t, partial, "pull", "--quiet")
	f.push(t, cleanSkill+"Third help.\n")
	f.push(t, cleanSkill+"Fourth help.\n")
	res, err := TrackedRepo(partial, f.opts())
	if err != nil || res.Status != StatusUpdated {
		t.Fatalf("partial update = %+v, %v", res, err)
	}
}

func TestTrackedRepo_CandidateMergesLocalCommit(t *testing.T) {
	f := newFixture(t, cleanSkill+"Upstream help.\n")
	testutil.ConfigureGitUser(t, f.repo)
	if err := os.WriteFile(filepath.Join(f.repo, "local.txt"), []byte("local commit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testutil.RunGit(t, f.repo, "add", "local.txt")
	testutil.RunGit(t, f.repo, "commit", "-m", "local work")
	local := f.head(t)
	res, err := TrackedRepo(f.repo, f.opts())
	if err != nil || res.Status != StatusUpdated {
		t.Fatalf("merge update = %+v, %v", res, err)
	}
	if got := testutil.RunGit(t, f.repo, "rev-parse", "HEAD^1"); got != local {
		t.Errorf("local commit was not preserved as merge parent: %s", got)
	}
	if f.head(t) != res.Info.AfterHash {
		t.Error("published merge differs from audited merge")
	}
}

func TestTrackedRepo_ForceScanErrorPreservesLocalEdits(t *testing.T) {
	f := newFixture(t, cleanSkill+"Upstream help.\n")
	local := filepath.Join(f.repo, "SKILL.md")
	if err := os.WriteFile(local, []byte("local edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	project := t.TempDir()
	rules := filepath.Join(project, ".skillshare", "audit-rules.yaml")
	if err := os.MkdirAll(filepath.Dir(rules), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rules, []byte("rules: [unterminated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := f.opts()
	opts.Force, opts.ProjectRoot = true, project
	res, err := TrackedRepo(f.repo, opts)
	gate := blocked(t, err, install.CheckoutUnchanged)
	if gate.ScanErr == nil || res.Discarded {
		t.Fatalf("scan failure discarded edits: %+v, %v", res, err)
	}
	if got, err := os.ReadFile(local); err != nil || string(got) != "local edit\n" || f.head(t) != f.before {
		t.Errorf("scan failure changed live checkout: %q, %v", got, err)
	}
}

func TestTrackedRepo_CheckoutChangedDuringConfirmation(t *testing.T) {
	f := newFixture(t, maliciousSkill)
	opts := f.opts()
	opts.Confirm = func(*TrackedRepoResult) (bool, error) {
		return true, os.WriteFile(filepath.Join(f.repo, "SKILL.md"), []byte("new local edit\n"), 0o644)
	}
	res, err := TrackedRepo(f.repo, opts)
	if err == nil || !strings.Contains(err.Error(), "checkout changed while auditing") || res.Recorded != 0 {
		t.Fatalf("unexpected publication result: %+v, %v", res, err)
	}
	if got, err := os.ReadFile(filepath.Join(f.repo, "SKILL.md")); err != nil || string(got) != "new local edit\n" || f.head(t) != f.before {
		t.Errorf("concurrent edit was overwritten: %q, %v", got, err)
	}
}
