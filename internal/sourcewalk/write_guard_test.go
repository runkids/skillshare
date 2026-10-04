package sourcewalk

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// followReadUses are the functions allowed to call InFollowed directly. Each
// only classifies paths for display, audit, or update policy; none decides
// whether a new path may be created in the skills source. Every write guard
// goes through FollowSet.WriteBoundary, which fails closed on an unreadable
// declaration instead of trusting an empty snapshot.
var followReadUses = map[string]string{
	"cmd/skillshare/list.go:displayTrackedRepos":                            "display: marks a tracked repo as followed",
	"cmd/skillshare/audit.go:auditPathFollowed":                             "audit: picks the resolved-path scan",
	"cmd/skillshare/update_resolve.go:resolveGroupUpdatableWithOptions":     "update target resolution; update refuses an incomplete snapshot (followDiscoveryError)",
	"cmd/skillshare/update_handlers.go:refreshTrackedRootSkillMetadata":     "update: skips a metadata refresh for a followed repo; update refuses an incomplete snapshot",
	"cmd/skillshare/update_batch.go:auditScanFn":                            "audit: picks the resolved-path scan",
	"internal/install/install_audit.go:auditTrackedRepoUpdate":              "audit: picks the resolved-path scan",
	"internal/install/followed_update.go:PrepareFollowedUpdate":             "update policy; checks Err() first (followDiscoveryError)",
	"internal/install/followed_update.go:RefuseFollowedSkillUpdate":         "update policy; checks Err() first (followDiscoveryError)",
	"internal/install/install_queries.go:GetMissingTrackedReposWithOptions": "rehydrate query; the preceding walk returns the declaration error",
	"internal/config/reconcile_core.go:reconcileSkillsWalk":                 "metadata reconcile; the walk returns the declaration error",
	"internal/config/reconcile_core.go:pruneStaleEntries":                   "metadata reconcile; runs only after a successful walk",
	"internal/server/handler_update.go:auditGateTrackedRepo":                "audit: picks the resolved-path scan",
	"internal/server/handler_skill_content.go:handlePatchSkillSource":       "metadata edit of an existing skill; names the resolved target, and an unreadable declaration fails the lookup's discovery first",
	"internal/audit/audit_follow.go:MarkFollowedInputs":                     "audit: marks followed inputs",
}

// scanFollowCalls returns "file:function" for every InFollowed call outside
// sourcewalk itself. Closures count toward their enclosing function.
func scanFollowCalls(files map[string][]byte) ([]string, error) {
	var calls []string
	for name, data := range files {
		tree, err := parser.ParseFile(token.NewFileSet(), name, data, 0)
		if err != nil {
			return nil, err
		}
		for _, decl := range tree.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "InFollowed" {
						calls = append(calls, name+":"+fn.Name.Name)
					}
				}
				return true
			})
		}
	}
	sort.Strings(calls)
	return calls, nil
}

func compareFollowUses(calls []string, allowed map[string]string) []string {
	var failures []string
	seen := map[string]bool{}
	for _, call := range calls {
		seen[call] = true
		if _, ok := allowed[call]; !ok {
			failures = append(failures, "write guard must use FollowSet.WriteBoundary, not InFollowed: "+call)
		}
	}
	for use := range allowed {
		if !seen[use] {
			failures = append(failures, "stale read-only InFollowed use: "+use)
		}
	}
	sort.Strings(failures)
	return failures
}

func TestFollowWriteGuard(t *testing.T) {
	root := filepath.Join("..", "..")
	files := map[string][]byte{}
	for _, dir := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if strings.HasPrefix(rel, "internal/sourcewalk/") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			files[rel] = data
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	calls, err := scanFollowCalls(files)
	if err != nil {
		t.Fatal(err)
	}
	if failures := compareFollowUses(calls, followReadUses); len(failures) > 0 {
		t.Fatal(strings.Join(failures, "\n"))
	}
}

func TestFollowWriteGuardRejectsBareCheck(t *testing.T) {
	allowed := map[string]string{"cmd/example.go:show": "display"}
	for _, tc := range []struct{ name, source, want string }{
		{"bare write guard", `package p
func show(f FollowSet) { f.InFollowed("a") }
func create(f FollowSet) error { if _, ok := f.InFollowed("a"); ok { return nil }; return nil }`, "must use FollowSet.WriteBoundary"},
		{"closure in write guard", `package p
func show(f FollowSet) { f.InFollowed("a") }
func create(f FollowSet) { func() { f.InFollowed("a") }() }`, "cmd/example.go:create"},
		{"stale allowance", `package p
func show() {}`, "stale read-only InFollowed use"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls, err := scanFollowCalls(map[string][]byte{"cmd/example.go": []byte(tc.source)})
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(compareFollowUses(calls, allowed), "\n"); !strings.Contains(got, tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
