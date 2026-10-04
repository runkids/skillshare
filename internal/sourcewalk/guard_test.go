package sourcewalk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The normalized expression is a readable fingerprint, independent of line
// numbers and whitespace. Counts matter: copying a call also needs an allowance.
type rawWalk struct {
	File     string `json:"file"`
	Function string `json:"function"`
	Callee   string `json:"callee"`
	Root     string `json:"root"`
}

type allowance struct {
	rawWalk
	Reason string `json:"reason"`
	Note   string `json:"note"`
}

// Reason meanings are centralized here. Non-skills resource reasons extend the
// proposal's initial vocabulary so unrelated scans are not mislabeled.
var walkReasons = map[string]string{
	"target":       "Installed target inspection or target copying",
	"backup":       "Backup storage or snapshot contents",
	"trash":        "Trash storage or entries",
	"clone":        "Downloaded or cloned repository inspection",
	"agents":       "Agent resources",
	"extras":       "Extra resources",
	"git-root":     "Physical git staging tree or repository inspection",
	"migration":    "Legacy sidecar migration; must not follow source entries",
	"skill-dir":    "Single skill or group contents; root behavior deferred to later rollout",
	"sourcewalk":   "Shared skills-source traversal implementation",
	"config":       "Configuration storage",
	"assets":       "Application assets",
	"plugin":       "Plugin packages and locks",
	"hooks":        "Hook resources and snapshots",
	"memory":       "Shared memory notes",
	"project":      "Project initialization and content detection",
	"test-fixture": "Test infrastructure compiled as a non-test package",
}

func fingerprint(node ast.Node) string {
	var b bytes.Buffer
	if err := format.Node(&b, token.NewFileSet(), node); err != nil {
		panic(err)
	}
	return b.String()
}

func importNames(file *ast.File) map[string]string {
	imports := map[string]string{}
	for _, spec := range file.Imports {
		path, _ := strconv.Unquote(spec.Path.Value)
		name := filepath.Base(path)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		imports[name] = path
	}
	return imports
}

// importedSelector honors aliases and rejects identifiers shadowing imports.
func importedSelector(expr ast.Expr, imports map[string]string) (string, string) {
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if id, ok := sel.X.(*ast.Ident); ok && id.Obj == nil {
			return imports[id.Name], sel.Sel.Name
		}
	}
	if id, ok := expr.(*ast.Ident); ok && id.Obj == nil {
		return imports["."], id.Name
	}
	return "", ""
}

func osFileType(expr ast.Expr, imports map[string]string) bool {
	ptr, ok := ast.Unparen(expr).(*ast.StarExpr)
	if !ok {
		return false
	}
	pkg, name := importedSelector(ptr.X, imports)
	return pkg == "os" && name == "File"
}

type parsedSource struct {
	name    string
	tree    *ast.File
	imports map[string]string
}

// scanRawWalks is deliberately syntactic, not a type checker. In addition to
// explicit *os.File declarations, it recognizes os file constructors and local
// functions/methods whose first result is explicitly *os.File. It does not trace
// arbitrary interfaces, imported factories, or interprocedural data flow.
func scanRawWalks(files map[string][]byte) ([]rawWalk, error) {
	var sources []parsedSource
	factories := map[string]map[string]bool{}
	for name, data := range files {
		tree, err := parser.ParseFile(token.NewFileSet(), name, data, 0)
		if err != nil {
			return nil, err
		}
		imports := importNames(tree)
		sources = append(sources, parsedSource{name, tree, imports})
		pkg := filepath.Dir(name)
		if factories[pkg] == nil {
			factories[pkg] = map[string]bool{}
		}
		for _, decl := range tree.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Type.Results != nil && len(fn.Type.Results.List) > 0 && osFileType(fn.Type.Results.List[0].Type, imports) {
				factories[pkg][fn.Name.Name] = true
			}
		}
	}
	var calls []rawWalk
	for _, source := range sources {
		imports := source.imports
		fileObjects := map[*ast.Object]bool{}
		// Parameters, named results, local/global declarations, and explicit fields.
		fileFields := map[string]bool{}
		ast.Inspect(source.tree, func(node ast.Node) bool {
			var names []*ast.Ident
			switch n := node.(type) {
			case *ast.Field:
				if osFileType(n.Type, imports) {
					names = n.Names
					for _, name := range names {
						fileFields[name.Name] = true
					}
				}
			case *ast.ValueSpec:
				if osFileType(n.Type, imports) {
					names = n.Names
				}
			}
			for _, name := range names {
				if name.Obj != nil {
					fileObjects[name.Obj] = true
				}
			}
			return true
		})
		isFile := func(expr ast.Expr) bool {
			expr = ast.Unparen(expr)
			if id, ok := expr.(*ast.Ident); ok {
				return id.Obj != nil && fileObjects[id.Obj]
			}
			if sel, ok := expr.(*ast.SelectorExpr); ok {
				return fileFields[sel.Sel.Name]
			}
			return false
		}
		returnsFile := func(expr ast.Expr) bool {
			expr = ast.Unparen(expr)
			if assertion, ok := expr.(*ast.TypeAssertExpr); ok {
				return osFileType(assertion.Type, imports)
			}
			if isFile(expr) {
				return true
			}
			call, ok := expr.(*ast.CallExpr)
			if !ok {
				return false
			}
			pkg, name := importedSelector(call.Fun, imports)
			if pkg == "os" {
				switch name {
				case "Open", "OpenFile", "Create", "CreateTemp", "NewFile":
					return true
				}
			}
			if osFileType(call.Fun, imports) {
				return true
			}
			// Local functions and methods with explicit *os.File results.
			if id, ok := call.Fun.(*ast.Ident); ok {
				return factories[filepath.Dir(source.name)][id.Name]
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && pkg == "" {
				return factories[filepath.Dir(source.name)][sel.Sel.Name]
			}
			return false
		}
		// Iterate to recognize simple local aliases without relying on file order.
		for changed := true; changed; {
			changed = false
			ast.Inspect(source.tree, func(node ast.Node) bool {
				var lhs, rhs []ast.Expr
				switch n := node.(type) {
				case *ast.AssignStmt:
					lhs, rhs = n.Lhs, n.Rhs
				case *ast.ValueSpec:
					for _, id := range n.Names {
						lhs = append(lhs, id)
					}
					rhs = n.Values
				}
				for i, expr := range rhs {
					if i >= len(lhs) || !returnsFile(expr) {
						continue
					}
					if id, ok := lhs[i].(*ast.Ident); ok && id.Obj != nil && !fileObjects[id.Obj] {
						fileObjects[id.Obj] = true
						changed = true
					}
				}
				return true
			})
		}
		for _, decl := range source.tree.Decls {
			enclosing := "<package>"
			if fn, ok := decl.(*ast.FuncDecl); ok {
				enclosing = fn.Name.Name
				if fn.Recv != nil {
					enclosing = "(" + fingerprint(fn.Recv.List[0].Type) + ")." + enclosing
				}
			}
			ast.Inspect(decl, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				pkg, name := importedSelector(call.Fun, imports)
				callee := ""
				rootIndex := 0
				switch {
				case pkg == "path/filepath" && (name == "Walk" || name == "WalkDir"):
					callee = "filepath." + name
				case pkg == "io/fs" && name == "WalkDir":
					callee = "fs.WalkDir"
					rootIndex = 1
				case pkg == "os" && name == "ReadDir":
					callee = "os.ReadDir"
				}
				var root ast.Expr
				if callee != "" && len(call.Args) > rootIndex {
					root = call.Args[rootIndex]
				}
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && (sel.Sel.Name == "ReadDir" || sel.Sel.Name == "Readdir") && returnsFile(sel.X) {
					callee = "(*os.File)." + sel.Sel.Name
					root = sel.X
				}
				if root != nil {
					calls = append(calls, rawWalk{source.name, enclosing, callee, fingerprint(root)})
				}
				return true
			})
		}
	}
	sort.Slice(calls, func(i, j int) bool { return fmt.Sprint(calls[i]) < fmt.Sprint(calls[j]) })
	return calls, nil
}

func compareAllowances(calls []rawWalk, allowed []allowance) []string {
	counts := map[rawWalk]int{}
	for _, call := range calls {
		counts[call]++
	}
	var failures []string
	for _, entry := range allowed {
		if _, ok := walkReasons[entry.Reason]; !ok {
			failures = append(failures, fmt.Sprintf("invalid reason %q: %+v", entry.Reason, entry.rawWalk))
		}
		if entry.Note == "" {
			failures = append(failures, fmt.Sprintf("missing classification note: %+v", entry.rawWalk))
		}
		if counts[entry.rawWalk] == 0 {
			failures = append(failures, fmt.Sprintf("stale allowance: %+v", entry.rawWalk))
		} else {
			counts[entry.rawWalk]--
		}
	}
	for call, count := range counts {
		if count > 0 {
			failures = append(failures, fmt.Sprintf("unclassified raw walk (%d): %+v", count, call))
		}
	}
	sort.Strings(failures)
	return failures
}

func TestRawWalkGuard(t *testing.T) {
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
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			files[filepath.ToSlash(rel)] = data
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	calls, err := scanRawWalks(files)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("allowlist.json")
	if err != nil {
		t.Fatal(err)
	}
	var allowed []allowance
	if err := json.Unmarshal(data, &allowed); err != nil {
		t.Fatal(err)
	}
	if failures := compareAllowances(calls, allowed); len(failures) > 0 {
		t.Fatal(strings.Join(failures, "\n"))
	}
}

func TestGuardRejectsNewAndStaleCalls(t *testing.T) {
	baseline := rawWalk{"cmd/example.go", "scan", "os.ReadDir", "root"}
	allowed := []allowance{{baseline, "target", "Target inspection"}}
	if got := compareAllowances([]rawWalk{baseline}, allowed); len(got) != 0 {
		t.Fatal(got)
	}
	for _, tc := range []struct{ name, source, want string }{
		{"new call in allowed function", `package p; import "os"; func scan() { os.ReadDir(root); os.ReadDir(other) }`, "unclassified"},
		{"duplicate expression in allowed function", `package p; import "os"; func scan() { os.ReadDir(root); os.ReadDir(root) }`, "unclassified"},
		{"stale allowance", `package p; func scan() {}`, "stale allowance"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls, err := scanRawWalks(map[string][]byte{"cmd/example.go": []byte(tc.source)})
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(compareAllowances(calls, allowed), "\n"); !strings.Contains(got, tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestGuardImportAndFileMatching(t *testing.T) {
	source := `package p
 import (p "path/filepath"; f "io/fs"; o "os")
 func factory() (*o.File, error) { return nil, nil }
 func scan(file *o.File, tree f.FS) {
  p.Walk(root, nil); p.WalkDir(root, nil); f.WalkDir(tree, "sub", nil); o.ReadDir(root)
  file.ReadDir(-1); file.Readdir(1)
  opened, _ := o.Open(root); opened.ReadDir(-1)
  made, _ := factory(); made.Readdir(-1)
  var explicit *o.File; explicit.ReadDir(-1)
  alias := opened; alias.ReadDir(-1)
  o.NewFile(0, "dir").ReadDir(-1)
  (*o.File)(nil).ReadDir(-1)
  value.(*o.File).Readdir(-1)
  unrelated.ReadDir(-1)
 }
 func shadow(o interface{ ReadDir(string) }) { o.ReadDir(root) }
 `
	calls, err := scanRawWalks(map[string][]byte{"cmd/example.go": []byte(source)})
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 13 {
		t.Fatalf("got %d calls: %+v", len(calls), calls)
	}
	foundFS := false
	for _, call := range calls {
		if call.Callee == "fs.WalkDir" {
			foundFS = call.Root == `"sub"`
		}
	}
	if !foundFS {
		t.Fatal("fs.WalkDir must fingerprint the root, not the filesystem")
	}
}
