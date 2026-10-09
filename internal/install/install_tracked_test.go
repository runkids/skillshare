package install

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/sourcewalk"
)

// makeRemote creates a bare git remote with a SKILL.md on the default branch,
// plus an optional extra branch if extraBranch != "".
func makeRemote(t *testing.T, extraBranch string) (remoteURL string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmp := t.TempDir()
	work := filepath.Join(tmp, "work")
	remote := filepath.Join(tmp, "remote.git")

	mustRunGit(t, "", "init", "-b", "main", work)
	mustRunGit(t, work, "config", "user.email", "test@test.com")
	mustRunGit(t, work, "config", "user.name", "Test")

	skillFile := filepath.Join(work, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("# test skill"), 0644); err != nil {
		t.Fatal(err)
	}
	mustRunGit(t, work, "add", ".")
	mustRunGit(t, work, "commit", "-m", "init")

	if extraBranch != "" {
		mustRunGit(t, work, "checkout", "-b", extraBranch)
		if err := os.WriteFile(skillFile, []byte("# test skill on "+extraBranch), 0644); err != nil {
			t.Fatal(err)
		}
		mustRunGit(t, work, "add", ".")
		mustRunGit(t, work, "commit", "-m", "branch commit")
		mustRunGit(t, work, "checkout", "main")
	}

	mustRunGit(t, "", "clone", "--bare", work, remote)
	return fileURL(remote)
}

// fileURL turns a local path into a file URL git accepts; Windows paths need
// a leading slash and forward slashes (file:///C:/dir).
func fileURL(p string) string {
	p = filepath.ToSlash(p)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return "file://" + p
}

// metadataInstallContext is a global-mode InstallContext over a metadata file.
type metadataInstallContext struct{ dir string }

func (c metadataInstallContext) SourcePath() string { return c.dir }
func (c metadataInstallContext) ConfigSkills() []SkillEntryDTO {
	return MetadataSkillEntries(LoadMetadataOrNew(c.dir))
}
func (metadataInstallContext) Reconcile() error              { return nil }
func (metadataInstallContext) PostInstallSkill(string) error { return nil }
func (metadataInstallContext) Mode() string                  { return "global" }
func (metadataInstallContext) GitLabHosts() []string         { return nil }
func (metadataInstallContext) AzureHosts() []string          { return nil }
func (metadataInstallContext) CNBHosts() []string            { return nil }
func (metadataInstallContext) GiteaHosts() []string          { return nil }

// TestInstallFromConfig_ReclonesMissingTrackedRepo verifies that a tracked repo
// declared in metadata but absent on disk is listed as missing and re-cloned
// under its recorded name (issue #212).
func TestInstallFromConfig_ReclonesMissingTrackedRepo(t *testing.T) {
	remoteURL := makeRemote(t, "")
	sourceDir := t.TempDir()

	// Declare the tracked repo in metadata WITHOUT cloning it (fresh machine).
	store := LoadMetadataOrNew(sourceDir)
	store.Set("_team-skills", &MetadataEntry{Source: remoteURL, Tracked: true})
	if err := store.Save(sourceDir); err != nil {
		t.Fatalf("save metadata: %v", err)
	}
	ctx := metadataInstallContext{sourceDir}
	if missing := MissingFromConfig(ctx); len(missing) != 1 || !missing[0].Tracked {
		t.Fatalf("expected the tracked repo to be missing, got %+v", missing)
	}

	result, err := InstallFromConfig(ctx, InstallOptions{SkipAudit: true, Quiet: true})
	if err != nil {
		t.Fatalf("InstallFromConfig() error = %v", err)
	}
	if result.InstalledRepos != 1 || len(result.FailedSkills) != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(sourceDir, "_team-skills", ".git")); err != nil {
		t.Fatalf("expected cloned repo at _team-skills: %v", err)
	}
	if missing := MissingFromConfig(ctx); len(missing) != 0 {
		t.Fatalf("expected nothing missing after install, got %+v", missing)
	}
}

// TestInstallTrackedRepo_NameFromOptions verifies that a name passed in
// opts.Name (--name, or the name recorded in config) wins over the
// URL-derived TrackName().
func TestInstallTrackedRepo_NameFromOptions(t *testing.T) {
	remoteURL := makeRemote(t, "")
	sourceDir := t.TempDir()

	source := &Source{
		Type:     SourceTypeGitHTTPS,
		Raw:      remoteURL,
		CloneURL: remoteURL,
	}

	result, err := InstallTrackedRepo(source, sourceDir, InstallOptions{Name: "my-custom-name"})
	if err != nil {
		t.Fatalf("InstallTrackedRepo() error = %v", err)
	}
	if result.Action != "cloned" {
		t.Fatalf("Action = %q, want %q", result.Action, "cloned")
	}

	want := filepath.Join(sourceDir, "_my-custom-name")
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("expected repo at %q but stat failed: %v", want, err)
	}
	// Confirm the URL-derived name was NOT used.
	unwanted := filepath.Join(sourceDir, "_"+source.TrackName())
	if _, err := os.Stat(unwanted); err == nil {
		t.Fatalf("repo should NOT exist at URL-derived path %q", unwanted)
	}
}

// TestInstallTrackedRepo_SameBasenameDoesNotCollide verifies that two repos
// whose URLs end in the same basename get distinct owner-repo directories,
// even though ParseSource fills source.Name with that shared basename.
func TestInstallTrackedRepo_SameBasenameDoesNotCollide(t *testing.T) {
	remoteURL := makeRemote(t, "")
	sourceDir := t.TempDir()
	seen := map[string]bool{}
	for _, owner := range []string{"alice", "bob"} {
		// Raw names the owner (TrackName reads it); the clone uses the local remote.
		source := &Source{
			Type:     SourceTypeGitSSH,
			Raw:      "git@example.com:" + owner + "/skills.git",
			CloneURL: remoteURL,
			Name:     "skills", // what ParseSource fills in from the basename
		}
		result, err := InstallTrackedRepo(source, sourceDir, InstallOptions{SkipAudit: true})
		if err != nil {
			t.Fatalf("install %s: %v", owner, err)
		}
		if result.RepoName == "_skills" || seen[result.RepoName] {
			t.Fatalf("repo %s got colliding name %q", owner, result.RepoName)
		}
		seen[result.RepoName] = true
	}
}

// TestInstallTrackedRepo_ReusesLegacyBasenameCheckout verifies that repeating
// an install whose repo was cloned under the older basename name (_skills)
// skips as the same repo instead of cloning a second _owner-skills copy.
func TestInstallTrackedRepo_ReusesLegacyBasenameCheckout(t *testing.T) {
	remoteURL := makeRemote(t, "")
	sourceDir := t.TempDir()
	source := &Source{Type: SourceTypeGitSSH, Raw: "git@example.com:alice/skills.git", CloneURL: remoteURL, Name: "skills"}
	if _, err := InstallTrackedRepo(source, sourceDir, InstallOptions{Name: "skills", SkipAudit: true}); err != nil {
		t.Fatalf("legacy install: %v", err)
	}

	_, err := InstallTrackedRepo(source, sourceDir, InstallOptions{SkipAudit: true})
	if !errors.Is(err, ErrSkipSameRepo) {
		t.Fatalf("err = %v, want ErrSkipSameRepo", err)
	}
	if _, err := os.Stat(filepath.Join(sourceDir, "_"+source.TrackName())); err == nil {
		t.Fatal("cloned a second copy under the owner-qualified name")
	}
}

// TestInstallTrackedRepo_BranchFromSource verifies that when opts.Branch is
// empty, the repo is cloned onto source.Branch rather than the remote default.
func TestInstallTrackedRepo_BranchFromSource(t *testing.T) {
	const featureBranch = "feature/my-work"
	remoteURL := makeRemote(t, featureBranch)
	sourceDir := t.TempDir()

	source := &Source{
		Type:     SourceTypeGitHTTPS,
		Raw:      remoteURL,
		CloneURL: remoteURL,
		Branch:   featureBranch, // should be used; opts.Branch is empty
	}

	result, err := InstallTrackedRepo(source, sourceDir, InstallOptions{Name: "mybranch-skill"})
	if err != nil {
		t.Fatalf("InstallTrackedRepo() error = %v", err)
	}
	if result.Action != "cloned" {
		t.Fatalf("Action = %q, want %q", result.Action, "cloned")
	}

	repoPath := filepath.Join(sourceDir, "_mybranch-skill")
	if _, err := os.Stat(repoPath); err != nil {
		t.Fatalf("expected repo at %q: %v", repoPath, err)
	}

	out, err2 := exec.Command("git", "-C", repoPath, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err2 != nil {
		t.Fatalf("git rev-parse failed: %v", err2)
	}
	got := strings.TrimSpace(string(out))
	if got != featureBranch {
		t.Errorf("cloned branch = %q, want %q", got, featureBranch)
	}
}

func TestMissingTrackedReposFollowPolicy(t *testing.T) {
	source, checkout := t.TempDir(), t.TempDir()
	mustRunGit(t, checkout, "init")
	if err := os.Symlink(checkout, filepath.Join(source, "_dev-skills")); err != nil {
		t.Fatal(err)
	}
	store := LoadMetadataOrNew(source)
	store.Set("_dev-skills", &MetadataEntry{Source: "https://example.com/repo.git", Tracked: true})
	if err := store.Save(source); err != nil {
		t.Fatal(err)
	}
	walk := sourcewalk.Options{Follow: sourcewalk.NewFollow(source, nil)}
	missing, err := GetMissingTrackedRepos(source, walk)
	if err != nil || len(missing) != 0 {
		t.Fatalf("present followed checkout reported missing: %+v %v", missing, err)
	}
	if got := MissingFromConfig(metadataInstallContext{source}); len(got) != 0 {
		t.Fatalf("present followed checkout listed for install: %+v", got)
	}
}

func TestInferTrackedKind_LocalGitPathBecomesFileURL(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo := t.TempDir()
	mustRunGit(t, "", "init", "-b", "main", repo)
	if err := os.WriteFile(filepath.Join(repo, "SKILL.md"), []byte("# s"), 0644); err != nil {
		t.Fatal(err)
	}

	source, err := ParseSource(repo)
	if err != nil {
		t.Fatal(err)
	}
	kind, err := InferTrackedKind(source, "skill")
	if err != nil {
		t.Fatalf("local git repo should be trackable: %v", err)
	}
	if kind != "skill" || source.CloneURL != fileURL(repo) {
		t.Errorf("kind=%q cloneURL=%q, want skill / %q", kind, source.CloneURL, fileURL(repo))
	}
}

func TestInferTrackedKind_PlainLocalDirHintsFileURL(t *testing.T) {
	source, err := ParseSource(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, err = InferTrackedKind(source, "skill")
	if err == nil || !strings.Contains(err.Error(), "file://") {
		t.Fatalf("want error naming file://, got %v", err)
	}
}

func TestLocalFileURL_KeepsWindowsDriveInPath(t *testing.T) {
	// "C:/repo" is what a Windows path looks like after filepath.ToSlash.
	if got := localFileURL("C:/repo"); got != "file:///C:/repo" {
		t.Errorf("localFileURL(C:/repo) = %q, want file:///C:/repo", got)
	}
	if got := localFileURL("/tmp/repo"); got != "file:///tmp/repo" {
		t.Errorf("localFileURL(/tmp/repo) = %q, want file:///tmp/repo", got)
	}
	if got := localFileURL("/tmp/my repo%20x#1"); got != "file:///tmp/my%20repo%2520x%231" {
		t.Errorf("localFileURL escaping = %q, want file:///tmp/my%%20repo%%2520x%%231", got)
	}
}

func TestParseSource_ExplicitUNCFileURLKeepsAuthority(t *testing.T) {
	source, err := ParseSource("file://server/share/repo")
	if err != nil {
		t.Fatal(err)
	}
	if source.CloneURL != "file://server/share/repo" {
		t.Errorf("CloneURL = %q, want file://server/share/repo", source.CloneURL)
	}
}

func TestInferTrackedKind_LocalBareRepoPath(t *testing.T) {
	bare := strings.TrimPrefix(makeRemote(t, ""), "file://")
	// A Windows file URL is file:///C:/dir; drop the slash before the drive.
	if len(bare) > 2 && bare[0] == '/' && bare[2] == ':' {
		bare = bare[1:]
	}

	source, err := ParseSource(bare)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := InferTrackedKind(source, "skill"); err != nil {
		t.Fatalf("local bare repo should be trackable: %v", err)
	}
}

func TestInstallTrackedRepo_ForceNeverDestroysItsLocalSource(t *testing.T) {
	cases := []struct {
		name   string
		repo   string // relative to the destination
		source func(repo string) string
		dryRun bool
	}{
		{"same path", "", func(p string) string { return p }, false},
		{"nested path", "nested", func(p string) string { return p }, false},
		{"dry run", "nested", func(p string) string { return p }, true},
		{"explicit file URL", "nested", func(p string) string { return fileURL(p) }, false},
		{"localhost file URL", "nested", func(p string) string { return "file://localhost/" + strings.TrimPrefix(filepath.ToSlash(p), "/") }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sourceDir := t.TempDir()
			dest := filepath.Join(sourceDir, "_foo")
			repo := filepath.Join(dest, tc.repo)
			mustRunGit(t, "", "init", "-b", "main", repo)
			if err := os.WriteFile(filepath.Join(repo, "SKILL.md"), []byte("# skill"), 0644); err != nil {
				t.Fatal(err)
			}
			mustRunGit(t, repo, "add", ".")
			mustRunGit(t, repo, "-c", "user.email=a@b", "-c", "user.name=n", "commit", "-m", "i")
			// Uncommitted work: a clone cannot carry it, so only refusing keeps it.
			wip := filepath.Join(repo, "wip.txt")
			if err := os.WriteFile(wip, []byte("unpushed"), 0644); err != nil {
				t.Fatal(err)
			}

			source, err := ParseSource(tc.source(repo))
			if err != nil {
				t.Fatal(err)
			}
			_, err = InstallTrackedRepo(source, sourceDir, InstallOptions{Name: "foo", Force: true, DryRun: tc.dryRun})
			if err == nil {
				t.Fatal("expected an error when the source is inside the install destination")
			}
			if _, err := os.Stat(wip); err != nil {
				t.Fatalf("source must be left intact: %v", err)
			}
		})
	}
}

func TestLocalCloneSource_UNCFileURL(t *testing.T) {
	got := localCloneSource(&Source{CloneURL: "file://server/share/repo"})
	if want := filepath.FromSlash("//server/share/repo"); got != want {
		t.Errorf("localCloneSource = %q, want %q", got, want)
	}
}

func TestInstallTrackedRepo_ForceKeepsExistingRepoWhenCloneFails(t *testing.T) {
	remoteURL := makeRemote(t, "")
	sourceDir := t.TempDir()
	source := &Source{Type: SourceTypeGitHTTPS, Raw: remoteURL, CloneURL: remoteURL}
	if _, err := InstallTrackedRepo(source, sourceDir, InstallOptions{Name: "foo"}); err != nil {
		t.Fatalf("first install: %v", err)
	}

	_, err := InstallTrackedRepo(source, sourceDir, InstallOptions{Name: "foo", Force: true, Branch: "no-such-branch"})
	if err == nil {
		t.Fatal("expected the clone of a missing branch to fail")
	}
	if _, statErr := os.Stat(filepath.Join(sourceDir, "_foo", "SKILL.md")); statErr != nil {
		t.Fatalf("the existing repo must survive a failed forced reinstall: %v", statErr)
	}
	entries, _ := os.ReadDir(sourceDir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".skillshare-clone-") {
			t.Errorf("staging dir %s was left behind", e.Name())
		}
	}
}

func TestInstallTrackedRepo_ForceReplacesExistingRepoInto(t *testing.T) {
	remoteURL := makeRemote(t, "")
	sourceDir := t.TempDir()
	source := &Source{Type: SourceTypeGitHTTPS, Raw: remoteURL, CloneURL: remoteURL}
	opts := InstallOptions{Name: "foo", Into: "team"}
	if _, err := InstallTrackedRepo(source, sourceDir, opts); err != nil {
		t.Fatalf("first install: %v", err)
	}
	stale := filepath.Join(sourceDir, "team", "_foo", "stale.txt")
	if err := os.WriteFile(stale, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	opts.Force = true
	if _, err := InstallTrackedRepo(source, sourceDir, opts); err != nil {
		t.Fatalf("forced reinstall: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("the previous checkout should be replaced, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(sourceDir, "team", "_foo", "SKILL.md")); err != nil {
		t.Errorf("the new clone should be in place: %v", err)
	}
	entries, _ := os.ReadDir(filepath.Join(sourceDir, "team"))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".skillshare-") {
			t.Errorf("leftover %s after a forced reinstall", e.Name())
		}
	}
}

func TestInstallTrackedRepo_UpdateAcceptsSourceThatIsTheDestination(t *testing.T) {
	remoteURL := makeRemote(t, "")
	sourceDir := t.TempDir()
	remote := &Source{Type: SourceTypeGitHTTPS, Raw: remoteURL, CloneURL: remoteURL}
	if _, err := InstallTrackedRepo(remote, sourceDir, InstallOptions{Name: "foo"}); err != nil {
		t.Fatalf("first install: %v", err)
	}

	source, err := ParseSource(filepath.Join(sourceDir, "_foo"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := InstallTrackedRepo(source, sourceDir, InstallOptions{Name: "foo", Update: true})
	if err != nil {
		t.Fatalf("--update pulls in place and must not be refused: %v", err)
	}
	if result.Action != "updated" {
		t.Errorf("Action = %q, want updated", result.Action)
	}
}

func TestInstallTrackedRepo_ForceReportsRootSkillUnderFinalName(t *testing.T) {
	remoteURL := makeRemote(t, "")
	sourceDir := t.TempDir()
	source := &Source{Type: SourceTypeGitHTTPS, Raw: remoteURL, CloneURL: remoteURL}
	if _, err := InstallTrackedRepo(source, sourceDir, InstallOptions{Name: "foo"}); err != nil {
		t.Fatalf("first install: %v", err)
	}

	result, err := InstallTrackedRepo(source, sourceDir, InstallOptions{Name: "foo", Force: true})
	if err != nil {
		t.Fatalf("forced reinstall: %v", err)
	}
	if len(result.Skills) != 1 || result.Skills[0] != "_foo" {
		t.Errorf("Skills = %v, want [_foo]", result.Skills)
	}
}

func TestInstallTrackedRepo_KeepsDestinationClaimedByAnotherInstaller(t *testing.T) {
	remoteURL := makeRemote(t, "")
	sourceDir := t.TempDir()
	source := &Source{Type: SourceTypeGitHTTPS, Raw: remoteURL, CloneURL: remoteURL}
	theirs := filepath.Join(sourceDir, "_foo", "theirs.txt")
	claim := func(string) {
		// Another process creates the destination while this clone runs.
		if err := os.MkdirAll(filepath.Dir(theirs), 0755); err == nil {
			_ = os.WriteFile(theirs, []byte("x"), 0644)
		}
	}

	_, err := InstallTrackedRepo(source, sourceDir, InstallOptions{Name: "foo", OnProgress: claim})
	if err == nil {
		t.Fatal("expected the install to fail when the destination was claimed meanwhile")
	}
	if _, statErr := os.Stat(theirs); statErr != nil {
		t.Fatalf("the other installer's checkout must survive: %v", statErr)
	}
}
