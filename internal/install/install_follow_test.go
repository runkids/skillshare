package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/sourcewalk"
)

func TestGetTrackedReposWithFollow(t *testing.T) {
	root, ext := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(ext, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(ext, filepath.Join(root, "_repo")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".skillfollow"), []byte("_repo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	legacy, err := GetTrackedRepos(root)
	if err != nil || len(legacy) != 0 {
		t.Fatalf("%v %v", legacy, err)
	}
	set := sourcewalk.Follow(root, sourcewalk.FollowOptions{})
	repos, err := GetTrackedReposWithOptions(root, sourcewalk.Options{Follow: &set})
	if err != nil || len(repos) != 1 || repos[0] != "_repo" {
		t.Fatalf("%v %v", repos, err)
	}
}

// Metadata below a declared entry belongs to the entry's external owner, so it
// is never offered for rehydration, whether the link is present or offline.
func TestMissingTrackedReposSkipFollowedEntries(t *testing.T) {
	remoteURL := makeRemote(t, "")
	for _, linked := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing", true: "followed"}[linked], func(t *testing.T) {
			root := t.TempDir()
			if linked {
				if err := os.Symlink(t.TempDir(), filepath.Join(root, "group")); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(root, ".skillfollow"), []byte("group\n"), 0644); err != nil {
				t.Fatal(err)
			}
			store := NewMetadataStore()
			store.Set("group/_repo", &MetadataEntry{Source: remoteURL, Tracked: true})
			store.Set("_plain", &MetadataEntry{Source: remoteURL, Tracked: true})
			if err := store.Save(root); err != nil {
				t.Fatal(err)
			}
			set := sourcewalk.Follow(root, sourcewalk.FollowOptions{})
			missing, err := GetMissingTrackedReposWithOptions(root, sourcewalk.Options{Follow: &set})
			if err != nil || len(missing) != 1 || missing[0].Name != "_plain" {
				t.Errorf("missing = %+v, %v; want only _plain", missing, err)
			}
			results, err := RehydrateMissingTrackedRepos(root, ParseOptions{}, InstallOptions{Follow: &set, SkipAudit: true})
			if err != nil || len(results) != 1 || results[0].Name != "_plain" {
				t.Errorf("results = %+v, %v; want only _plain", results, err)
			}
			if _, err := os.Stat(filepath.Join(root, "group", "_repo")); !os.IsNotExist(err) {
				t.Fatalf("rehydrate wrote below the followed entry: %v", err)
			}
			if info, err := os.Lstat(filepath.Join(root, "group")); linked && (err != nil || info.Mode()&os.ModeSymlink == 0) {
				t.Fatalf("group link replaced: %v", err)
			} else if !linked && !os.IsNotExist(err) {
				t.Fatalf("rehydrate created the declared entry: %v", err)
			}
		})
	}
}

// A tracked install never creates a repo inside a declared entry, or the entry
// itself while its link is offline; only an update of an existing repo proceeds.
func TestInstallTrackedRepoRefusesDeclaredEntry(t *testing.T) {
	remoteURL := makeRemote(t, "")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".skillfollow"), []byte("group\n_repo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	set := sourcewalk.Follow(root, sourcewalk.FollowOptions{})
	source := &Source{Type: SourceTypeGitHTTPS, Raw: remoteURL, CloneURL: remoteURL}
	for _, opts := range []InstallOptions{{Name: "repo"}, {Name: "other", Into: "group"}, {Name: "other", Into: "group", Update: true}} {
		opts.Follow, opts.SkipAudit = &set, true
		if _, err := InstallTrackedRepo(source, root, opts); err == nil || !strings.Contains(err.Error(), "is a link") {
			t.Errorf("%+v: err = %v, want link refusal", opts, err)
		}
	}
	for _, name := range []string{"group", "_repo"} {
		if _, err := os.Lstat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Errorf("tracked install created declared entry %s: %v", name, err)
		}
	}
}

type configCtx struct {
	source string
	skills []SkillEntryDTO
}

func (c configCtx) SourcePath() string            { return c.source }
func (c configCtx) ConfigSkills() []SkillEntryDTO { return c.skills }
func (c configCtx) Reconcile() error              { return nil }
func (c configCtx) PostInstallSkill(string) error { return nil }
func (c configCtx) Mode() string                  { return "global" }
func (c configCtx) GitLabHosts() []string         { return nil }
func (c configCtx) AzureHosts() []string          { return nil }
func (c configCtx) CNBHosts() []string            { return nil }
func (c configCtx) GiteaHosts() []string          { return nil }

// An unreadable declaration may hide any entry, so every install into the
// source is refused with the read error instead of trusting an empty snapshot.
func TestInstallRefusesUnreadableDeclaration(t *testing.T) {
	remoteURL := makeRemote(t, "")
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".skillfollow"), 0755); err != nil {
		t.Fatal(err)
	}
	set := sourcewalk.Follow(root, sourcewalk.FollowOptions{})
	local := filepath.Join(t.TempDir(), "team")
	if err := os.MkdirAll(local, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(local, "SKILL.md"), []byte("---\nname: team\n---\n# team\n"), 0644); err != nil {
		t.Fatal(err)
	}
	assertRefused := func(t *testing.T, err error) {
		t.Helper()
		if err == nil || !strings.Contains(err.Error(), "read skillfollow declaration") {
			t.Errorf("err = %v, want the declaration read error", err)
		}
		if _, err := os.Lstat(filepath.Join(root, "team")); !os.IsNotExist(err) {
			t.Errorf("install created a possibly declared entry: %v", err)
		}
	}
	t.Run("plain", func(t *testing.T) {
		source, err := ParseSource(local)
		if err != nil {
			t.Fatal(err)
		}
		for _, follow := range []*sourcewalk.FollowSet{nil, &set} {
			_, err = Install(source, filepath.Join(root, "team"), InstallOptions{SourceDir: root, Follow: follow, SkipAudit: true})
			assertRefused(t, err)
		}
	})
	t.Run("tracked", func(t *testing.T) {
		source := &Source{Type: SourceTypeGitHTTPS, Raw: remoteURL, CloneURL: remoteURL}
		_, err := InstallTrackedRepo(source, root, InstallOptions{Name: "repo", Into: "team", Follow: &set, SkipAudit: true})
		assertRefused(t, err)
	})
	t.Run("config", func(t *testing.T) {
		ctx := configCtx{source: root, skills: []SkillEntryDTO{{Name: "demo", Source: remoteURL, Group: "team"}}}
		result, err := InstallFromConfig(ctx, InstallOptions{Follow: &set, SkipAudit: true, Quiet: true})
		if err != nil {
			t.Fatal(err)
		}
		if len(result.FailedSkills) != 1 || result.FailedSkills[0] != "team/demo" {
			t.Errorf("failed = %v, want [team/demo]", result.FailedSkills)
		}
		if _, err := os.Lstat(filepath.Join(root, "team")); !os.IsNotExist(err) {
			t.Errorf("install created a possibly declared entry: %v", err)
		}
	})
}
