package hub

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/sourcewalk"
	ssync "skillshare/internal/sync"
)

func TestHubFollowOptions(t *testing.T) {
	source, external := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(external, "a"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(external, "a", "SKILL.md"), []byte("---\nname: a\ndescription: Followed skill\n---\n# Content\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(source, "group")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".skillfollow"), []byte("group\n"), 0644); err != nil {
		t.Fatal(err)
	}
	index, err := BuildIndex(source, true, false)
	if err != nil || len(index.Skills) != 0 {
		t.Fatalf("legacy index: %+v %v", index, err)
	}
	candidates, err := DraftCandidates(source)
	if err != nil || len(candidates) != 0 {
		t.Fatalf("legacy candidates: %+v %v", candidates, err)
	}
	follow := sourcewalk.Follow(source, sourcewalk.FollowOptions{})
	opts := ssync.DiscoveryOptions{Follow: &follow}
	index, err = BuildIndexWithOptions(source, true, false, opts)
	if err != nil || len(index.Skills) != 1 || index.Skills[0].Source != "group/a" || index.Skills[0].FlatName != "group__a" {
		t.Fatalf("followed index: %+v %v", index, err)
	}
	candidates, err = DraftCandidatesWithOptions(source, opts)
	if err != nil || len(candidates) != 1 {
		t.Fatalf("followed candidates: %+v %v", candidates, err)
	}
	// The same operation snapshot must reject a disappeared tree, not return an empty index.
	if err := os.RemoveAll(external); err != nil {
		t.Fatal(err)
	}
	if index, err := BuildIndexWithOptions(source, true, false, opts); err == nil || index != nil || follow.Err() == nil {
		t.Fatalf("partial index: %+v %v", index, err)
	}
	if candidates, err := DraftCandidatesWithOptions(source, opts); err == nil || candidates != nil {
		t.Fatalf("partial candidates: %+v %v", candidates, err)
	}
}
