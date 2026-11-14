package profileapply

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	ssync "skillshare/internal/sync"
	"skillshare/internal/testutil"
	"skillshare/internal/utils"
)

func TestProfileApply_AfterRootModeChange(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, mode := range []string{"merge", "copy"} {
			t.Run(fmt.Sprintf("project=%v/%s", project, mode), func(t *testing.T) {
				f := newFixture(t, project, "symlink")
				v, err := Load(f.scope)
				if err != nil {
					t.Fatal(err)
				}
				discovery, err := v.Discover()
				if err != nil {
					t.Fatal(err)
				}
				seed := ssync.SyncSkillTarget(ssync.SkillTarget{Name: "custom", Target: v.Config.Targets["custom"], Mode: "symlink"}, discovery.Skills, ssync.SkillRunOptions{Source: f.source, ProjectRoot: f.scope.ProjectRoot})
				if seed.Err != nil {
					t.Fatal(seed.Err)
				}
				// `target --mode` saves only the setting. A profile switch must
				// work without requiring an intervening ordinary sync.
				write(t, f.path, strings.Replace(read(t, f.path), "mode: symlink", "mode: "+mode, 1))
				before := testutil.SnapshotTree(t, false, filepath.Dir(f.path))
				sourceBefore := testutil.SnapshotTree(t, false, f.source)
				preview, err := Apply(context.Background(), Request{Scope: f.scope, Name: "a", DryRun: true})
				if err != nil || len(preview.Blocked) != 0 || len(preview.Targets) != 1 || !reflect.DeepEqual(preview.Targets[0].Added, []string{"alpha"}) {
					t.Fatalf("preview after mode change: %+v, %v", preview, err)
				}
				if !reflect.DeepEqual(before, testutil.SnapshotTree(t, false, filepath.Dir(f.path))) || !reflect.DeepEqual(sourceBefore, testutil.SnapshotTree(t, false, f.source)) || !utils.IsSymlinkOrJunction(f.target) {
					t.Fatal("preview changed config, source or root link")
				}
				out, err := Apply(context.Background(), Request{Scope: f.scope, Name: "a"})
				if err != nil || !out.Complete || len(out.Blocked) != 0 {
					t.Fatalf("apply after mode change: %+v, %v", out, err)
				}
				if utils.IsSymlinkOrJunction(f.target) || !reflect.DeepEqual(sourceBefore, testutil.SnapshotTree(t, false, f.source)) {
					t.Fatal("apply failed to convert root or changed source")
				}
				if read(t, filepath.Join(f.target, "alpha", "SKILL.md")) != read(t, filepath.Join(f.source, "alpha", "SKILL.md")) {
					t.Fatal("selected skill not deployed")
				}
				if _, err := os.Lstat(filepath.Join(f.target, "beta")); !os.IsNotExist(err) {
					t.Fatalf("unselected skill remains: %v", err)
				}
				settled, err := Load(f.scope)
				if err != nil || settled.State.Active != "a" || settled.State.Pending {
					t.Fatalf("profile switch did not settle: %+v, %v", settled, err)
				}
			})
		}
	}
}
