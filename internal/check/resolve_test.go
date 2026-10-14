package check

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"skillshare/internal/install"
)

// fakeRemote answers from maps keyed by "url" or "url@branch" and records
// every call.
type fakeRemote struct {
	hashes map[string]string
	trees  map[string]map[string]string

	mu    sync.Mutex
	calls []string
}

func fakeRef(repoURL, branch string) string {
	if branch == "" {
		return repoURL
	}
	return repoURL + "@" + branch
}

func (f *fakeRemote) record(call string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call)
}

func (f *fakeRemote) Hash(repoURL, branch string) (string, error) {
	ref := fakeRef(repoURL, branch)
	f.record("hash " + ref)
	hash, ok := f.hashes[ref]
	if !ok {
		return "", errors.New("unreachable")
	}
	return hash, nil
}

func (f *fakeRemote) TreeHashes(repoURL, branch string) map[string]string {
	ref := fakeRef(repoURL, branch)
	f.record("trees " + ref)
	return f.trees[ref]
}

func remoteSkill(name, version, treeHash, subdir string) Skill {
	return Skill{Name: name, Entry: &install.MetadataEntry{
		Source: "src/" + name, RepoURL: "u", Version: version, TreeHash: treeHash, Subdir: subdir,
	}}
}

func TestResolution_Statuses(t *testing.T) {
	tests := []struct {
		name   string
		skills []Skill
		hashes map[string]string
		trees  map[string]map[string]string
		want   []string // statuses, in skill order
		calls  []string
	}{
		{
			name:   "remote unreachable marks every skill an error",
			skills: []Skill{remoteSkill("a", "c1", "t1", "a"), remoteSkill("b", "c1", "", "")},
			want:   []string{"error", "error"},
			calls:  []string{"hash u"},
		},
		{
			name:   "all at the remote commit skips the tree fetch",
			skills: []Skill{remoteSkill("a", "c1", "t1", "a"), remoteSkill("b", "c1", "", "")},
			hashes: map[string]string{"u": "c1"},
			want:   []string{"up_to_date", "up_to_date"},
			calls:  []string{"hash u"},
		},
		{
			name:   "moved commit without recorded tree hashes skips the tree fetch",
			skills: []Skill{remoteSkill("a", "c1", "", "a"), remoteSkill("b", "c1", "t1", "")},
			hashes: map[string]string{"u": "c2"},
			want:   []string{"update_available", "update_available"},
			calls:  []string{"hash u"},
		},
		{
			name: "moved commit is decided per skill by tree hash",
			skills: []Skill{
				remoteSkill("unchanged", "c1", "t1", "skills/unchanged"),
				remoteSkill("slash", "c1", "t1", "/skills/unchanged"),
				remoteSkill("changed", "c1", "t2", "skills/changed"),
				remoteSkill("removed", "c1", "t3", "skills/removed"),
				remoteSkill("notree", "c1", "", "skills/unchanged"),
				remoteSkill("atremote", "c2", "t9", "skills/changed"),
			},
			hashes: map[string]string{"u": "c2"},
			trees:  map[string]map[string]string{"u": {"skills/unchanged": "t1", "skills/changed": "t2b"}},
			want:   []string{"up_to_date", "up_to_date", "update_available", "stale", "update_available", "up_to_date"},
			calls:  []string{"hash u", "trees u"},
		},
		{
			name:   "failed tree fetch falls back to the commit comparison",
			skills: []Skill{remoteSkill("a", "c1", "t1", "a"), remoteSkill("b", "c2", "t2", "b")},
			hashes: map[string]string{"u": "c2"},
			want:   []string{"update_available", "up_to_date"},
			calls:  []string{"hash u", "trees u"},
		},
		{
			name:   "empty remote tree makes a recorded subdir stale",
			skills: []Skill{remoteSkill("a", "c1", "t1", "a")},
			hashes: map[string]string{"u": "c2"},
			trees:  map[string]map[string]string{"u": {}},
			want:   []string{"stale"},
			calls:  []string{"hash u", "trees u"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			remote := &fakeRemote{hashes: tt.hashes, trees: tt.trees}
			results, err := Plan(tt.skills, "").Run(context.Background(), Options{Remote: remote})
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for i, r := range results {
				if r.Name != tt.skills[i].Name {
					t.Fatalf("result %d is %q, want %q", i, r.Name, tt.skills[i].Name)
				}
				got = append(got, r.Status)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("statuses = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(remote.calls, tt.calls) {
				t.Errorf("remote calls = %v, want %v", remote.calls, tt.calls)
			}
		})
	}
}

func TestResolution_GroupsByURLAndBranch(t *testing.T) {
	entry := func(url, branch, version string) *install.MetadataEntry {
		return &install.MetadataEntry{RepoURL: url, Branch: branch, Version: version}
	}
	skills := []Skill{
		{Name: "head-1", Entry: entry("u", "", "c1")},
		{Name: "dev", Entry: entry("u", "dev", "c1")},
		{Name: "other", Entry: entry("v", "", "c1")},
		{Name: "head-2", Entry: entry("u", "", "c1")},
	}
	remote := &fakeRemote{hashes: map[string]string{"u": "c1", "u@dev": "d1", "v": "c1"}}

	plan := Plan(skills, "")
	if plan.Remotes() != 3 || plan.RemoteSkills() != 4 {
		t.Fatalf("Remotes() = %d, RemoteSkills() = %d, want 3 and 4", plan.Remotes(), plan.RemoteSkills())
	}
	var done []int
	results, err := plan.Run(context.Background(), Options{Remote: remote, OnRemoteDone: func(skills int) {
		done = append(done, skills)
	}})
	if err != nil {
		t.Fatal(err)
	}

	got := map[string]string{}
	var order []string
	for _, r := range results {
		got[r.Name] = r.Status
		order = append(order, r.Name)
	}
	want := map[string]string{"head-1": "up_to_date", "head-2": "up_to_date", "dev": "update_available", "other": "up_to_date"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("statuses = %v, want %v", got, want)
	}
	if wantOrder := []string{"head-1", "head-2", "dev", "other"}; !reflect.DeepEqual(order, wantOrder) {
		t.Errorf("order = %v, want %v", order, wantOrder)
	}
	if wantCalls := []string{"hash u", "hash u@dev", "hash v"}; !reflect.DeepEqual(remote.calls, wantCalls) {
		t.Errorf("remote calls = %v, want %v", remote.calls, wantCalls)
	}
	if wantDone := []int{2, 1, 1}; !reflect.DeepEqual(done, wantDone) {
		t.Errorf("OnRemoteDone got %v, want %v", done, wantDone)
	}
}

func TestResolution_LocalSkillsComeFirstWithTheirMetadata(t *testing.T) {
	installed := time.Date(2024, 5, 6, 0, 0, 0, 0, time.UTC)
	skills := []Skill{
		{Name: "remote", Entry: &install.MetadataEntry{Source: "src/remote", RepoURL: "u", Version: "c1", InstalledAt: installed}},
		{Name: "bare"},
		{Name: "local", Entry: &install.MetadataEntry{Source: "src/local", Version: "v1", InstalledAt: installed}},
	}
	remote := &fakeRemote{hashes: map[string]string{"u": "c1"}}

	results, err := Plan(skills, "").Run(context.Background(), Options{Remote: remote})
	if err != nil {
		t.Fatal(err)
	}
	want := []SkillResult{
		{Name: "bare", Status: "local", Local: true},
		{Name: "local", Source: "src/local", Version: "v1", InstalledAt: "2024-05-06", Status: "local", Local: true},
		{Name: "remote", Source: "src/remote", Version: "c1", InstalledAt: "2024-05-06", Status: "up_to_date"},
	}
	if !reflect.DeepEqual(results, want) {
		t.Errorf("results:\n got %+v\nwant %+v", results, want)
	}
}

func TestResolution_ParallelKeepsPlanOrder(t *testing.T) {
	var skills []Skill
	hashes := map[string]string{}
	var want []string
	for _, url := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"} {
		skills = append(skills, Skill{Name: url, Entry: &install.MetadataEntry{RepoURL: url, Version: "c1"}})
		hashes[url] = "c1"
		want = append(want, url)
	}
	var mu sync.Mutex
	done := 0
	results, err := Plan(skills, "").Run(context.Background(), Options{
		Remote:   &fakeRemote{hashes: hashes},
		Parallel: true,
		OnRemoteDone: func(skills int) {
			mu.Lock()
			defer mu.Unlock()
			done += skills
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range results {
		got = append(got, r.Name)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
	if done != len(skills) {
		t.Errorf("OnRemoteDone counted %d skills, want %d", done, len(skills))
	}
}

func TestResolution_StopsBeforeNextRemoteWhenCancelled(t *testing.T) {
	skills := []Skill{
		{Name: "a", Entry: &install.MetadataEntry{RepoURL: "a", Version: "c1"}},
		{Name: "b", Entry: &install.MetadataEntry{RepoURL: "b", Version: "c1"}},
	}
	remote := &fakeRemote{hashes: map[string]string{"a": "c1", "b": "c1"}}
	ctx, cancel := context.WithCancel(context.Background())

	results, err := Plan(skills, "").Run(ctx, Options{Remote: remote, OnRemoteDone: func(int) { cancel() }})
	if !errors.Is(err, context.Canceled) || results != nil {
		t.Fatalf("Run = %v, %v; want nil, context.Canceled", results, err)
	}
	if want := []string{"hash a"}; !reflect.DeepEqual(remote.calls, want) {
		t.Errorf("remote calls = %v, want %v", remote.calls, want)
	}
}

// The dashboard handlers rely on net/http recovering a panic, which it only
// does on the request's goroutine: a panic on another one ends the server.
func TestResolution_SequentialRunStaysOnTheCallersGoroutine(t *testing.T) {
	skills := []Skill{{Name: "a", Entry: &install.MetadataEntry{RepoURL: "a", Version: "c1"}}}
	remote := &fakeRemote{hashes: map[string]string{"a": "c1"}}
	defer func() {
		if recover() == nil {
			t.Fatal("a panic during a sequential Run must reach the caller")
		}
	}()
	Plan(skills, "").Run(context.Background(), Options{Remote: remote, OnRemoteDone: func(int) { panic("boom") }})
}

// TestGitRemoteHash_DefaultBranchUsesAuth verifies that a skill without a
// pinned branch still probes the remote HEAD with token auth, as the
// branch-pinned probe does. The fake git answers ls-remote only when the auth
// env carries the token.
func TestGitRemoteHash_DefaultBranchUsesAuth(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a shell script named git is not found by exec on Windows")
	}
	const token = "ghp_check_default_branch_token"
	fakeBin := t.TempDir()
	script := "#!/bin/sh\n" +
		"case \"$SKILLSHARE_GIT_AUTH_PASSWORD\" in " + token + ") ;; *) echo 'fatal: Authentication failed' >&2; exit 128;; esac\n" +
		"printf '0123456789abcdef0123456789abcdef01234567\\tHEAD\\n'\n"
	if err := os.WriteFile(filepath.Join(fakeBin, "git"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GIT_CONFIG_COUNT", "")
	t.Setenv("GITHUB_TOKEN", token)

	hash, err := GitRemote{}.Hash("https://github.com/org/private-skills.git", "")
	if err != nil {
		t.Fatalf("Hash without branch failed: %v", err)
	}
	if !strings.HasPrefix("0123456789abcdef0123456789abcdef01234567", hash) || hash == "" {
		t.Fatalf("unexpected hash %q", hash)
	}
}
