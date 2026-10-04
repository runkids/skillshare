package sourcewalk

import (
	"encoding/json"
	"fmt"
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

// followScanDirs are the entry points that own an operation's FollowSet. Every
// source discovery they start must carry it.
var followScanDirs = []string{"cmd/skillshare", "internal/server"}

const sourcewalkPath = "skillshare/internal/sourcewalk"

// followGap is a syntactic candidate for a discovery that drops the snapshot.
// Expr is a whitespace-independent fingerprint, so counts matter as in rawWalk.
type followGap struct {
	File     string `json:"file"`
	Function string `json:"function"`
	Rule     string `json:"rule"`
	Expr     string `json:"expr"`
}

type followAllowance struct {
	followGap
	Reason string `json:"reason"`
}

// followRules documents what each rule flags.
var followRules = map[string]string{
	"options":   "a literal of a type with a Follow *FollowSet field that omits Follow or sets it to nil",
	"argument":  "nil, or an omitted variadic, for a *FollowSet parameter",
	"api":       "a follow-less discovery API that has no options form",
	"err-check": "a follow-aware sourcewalk Walk or WalkDir in a function that never checks FollowSet.Err()",
}

// followWhenSet narrows a type whose Follow only matters with another field: the
// literal is checked only when it sets that field. InstallOptions.Follow scopes
// tracked-repository updates; fresh installs never walk the skills source.
var followWhenSet = map[string]map[string]string{
	"skillshare/internal/install": {"InstallOptions": "Update"},
}

// followLessAPIs are discovery calls that cannot take a snapshot. The receiver
// is matched as a composite literal of the named type.
var followLessAPIs = map[string]map[string]string{
	"skillshare/internal/resource": {"SkillKind": "Discover"},
}

// followKnowledge records where the snapshot travels, per package import path.
type followKnowledge struct {
	types   map[string]map[string]int   // type → index of its Follow field
	funcs   map[string]map[string][]int // package function → *FollowSet parameter indexes
	methods map[string]map[string][]int // method name → *FollowSet parameter indexes
	varargs map[string]map[string]bool  // function or method name → last parameter is ...*FollowSet
}

func isFollowSetType(expr ast.Expr, pkgPath string, imports map[string]string) (bool, bool) {
	variadic := false
	if ellipsis, ok := expr.(*ast.Ellipsis); ok {
		expr, variadic = ellipsis.Elt, true
	}
	star, ok := expr.(*ast.StarExpr)
	if !ok {
		return false, false
	}
	if id, ok := star.X.(*ast.Ident); ok && pkgPath == sourcewalkPath {
		return id.Name == "FollowSet", variadic
	}
	pkg, name := importedSelector(star.X, imports)
	return pkg == sourcewalkPath && name == "FollowSet", variadic
}

func packagePath(file string) string { return "skillshare/" + filepath.ToSlash(filepath.Dir(file)) }

func (k *followKnowledge) learn(file string, tree *ast.File, imports map[string]string) {
	pkg := packagePath(file)
	ensure := func(m map[string]map[string][]int) map[string][]int {
		if m[pkg] == nil {
			m[pkg] = map[string][]int{}
		}
		return m[pkg]
	}
	for _, decl := range tree.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				index := 0
				for _, field := range st.Fields.List {
					for _, name := range field.Names {
						if follow, _ := isFollowSetType(field.Type, pkg, imports); follow && name.Name == "Follow" {
							if k.types[pkg] == nil {
								k.types[pkg] = map[string]int{}
							}
							k.types[pkg][ts.Name.Name] = index
						}
						index++
					}
					if len(field.Names) == 0 {
						index++
					}
				}
			}
		case *ast.FuncDecl:
			var indexes []int
			variadic := false
			index := 0
			for _, field := range d.Type.Params.List {
				count := max(len(field.Names), 1)
				follow, isVariadic := isFollowSetType(field.Type, pkg, imports)
				for range count {
					if follow {
						indexes = append(indexes, index)
						variadic = variadic || isVariadic
					}
					index++
				}
			}
			if len(indexes) == 0 {
				continue
			}
			target := ensure(k.funcs)
			if d.Recv != nil {
				target = ensure(k.methods)
			}
			target[d.Name.Name] = indexes
			if variadic {
				if k.varargs[pkg] == nil {
					k.varargs[pkg] = map[string]bool{}
				}
				k.varargs[pkg][d.Name.Name] = true
			}
		}
	}
}

// scanFollowGaps parses every file for knowledge, then checks the files under
// followScanDirs. Like scanRawWalks it is syntactic: it trusts declared types,
// matches methods by name within their own package, and does not trace data flow.
func scanFollowGaps(files map[string][]byte) ([]followGap, error) {
	k := &followKnowledge{types: map[string]map[string]int{}, funcs: map[string]map[string][]int{}, methods: map[string]map[string][]int{}, varargs: map[string]map[string]bool{}}
	var sources []parsedSource
	for name, data := range files {
		tree, err := parser.ParseFile(token.NewFileSet(), name, data, 0)
		if err != nil {
			return nil, err
		}
		imports := importNames(tree)
		k.learn(name, tree, imports)
		for _, dir := range followScanDirs {
			if filepath.ToSlash(filepath.Dir(name)) == dir {
				sources = append(sources, parsedSource{name, tree, imports})
			}
		}
	}
	var gaps []followGap
	for _, source := range sources {
		pkg := packagePath(source.name)
		for _, decl := range source.tree.Decls {
			enclosing := "<package>"
			fn, isFunc := decl.(*ast.FuncDecl)
			if isFunc {
				enclosing = fn.Name.Name
				if fn.Recv != nil {
					enclosing = "(" + fingerprint(fn.Recv.List[0].Type) + ")." + enclosing
				}
			}
			add := func(rule string, node ast.Node) {
				gaps = append(gaps, followGap{source.name, enclosing, rule, fingerprint(node)})
			}
			checksErr := false
			var followWalks []ast.Node
			ast.Inspect(decl, func(node ast.Node) bool {
				switch n := node.(type) {
				case *ast.CompositeLit:
					if optionsGap(n, pkg, source.imports, k) {
						add("options", n)
					}
				case *ast.CallExpr:
					if sel, ok := n.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Err" && len(n.Args) == 0 {
						checksErr = true
					}
					if argumentGap(n, pkg, source.imports, k) {
						add("argument", n.Fun)
					}
					if apiGap(n, source.imports) {
						add("api", n.Fun)
					}
					if callPkg, name := importedSelector(n.Fun, source.imports); callPkg == sourcewalkPath && (name == "Walk" || name == "WalkDir") && len(n.Args) > 1 {
						if lit := optionsLiteral(n.Args[1]); lit == nil || !optionsGap(lit, pkg, source.imports, k) {
							followWalks = append(followWalks, n.Fun)
						}
					}
				}
				return true
			})
			if isFunc && !checksErr {
				for _, walk := range followWalks {
					add("err-check", walk)
				}
			}
		}
	}
	sort.Slice(gaps, func(i, j int) bool { return fmt.Sprint(gaps[i]) < fmt.Sprint(gaps[j]) })
	return gaps, nil
}

func optionsLiteral(expr ast.Expr) *ast.CompositeLit {
	expr = ast.Unparen(expr)
	if unary, ok := expr.(*ast.UnaryExpr); ok && unary.Op == token.AND {
		expr = unary.X
	}
	lit, _ := expr.(*ast.CompositeLit)
	return lit
}

// setsField reports whether a keyed literal sets field to something other than
// false; an empty field name always matches.
func setsField(lit *ast.CompositeLit, field string) bool {
	if field == "" {
		return true
	}
	for _, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if key, ok := kv.Key.(*ast.Ident); ok && key.Name == field {
				value, isIdent := kv.Value.(*ast.Ident)
				return !isIdent || value.Name != "false"
			}
		}
	}
	return false
}

func isNil(expr ast.Expr) bool {
	id, ok := ast.Unparen(expr).(*ast.Ident)
	return ok && id.Name == "nil" && id.Obj == nil
}

func optionsGap(lit *ast.CompositeLit, pkg string, imports map[string]string, k *followKnowledge) bool {
	typePkg, typeName := pkg, ""
	switch t := lit.Type.(type) {
	case *ast.Ident:
		typeName = t.Name
	case *ast.SelectorExpr:
		typePkg, typeName = importedSelector(t, imports)
	}
	index, ok := k.types[typePkg][typeName]
	if !ok || !setsField(lit, followWhenSet[typePkg][typeName]) {
		return false
	}
	for i, elt := range lit.Elts {
		kv, keyed := elt.(*ast.KeyValueExpr)
		if !keyed {
			if i == index {
				return isNil(elt)
			}
			continue
		}
		if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "Follow" {
			return isNil(kv.Value)
		}
	}
	return true
}

func argumentGap(call *ast.CallExpr, pkg string, imports map[string]string, k *followKnowledge) bool {
	var indexes []int
	calleePkg, name := pkg, ""
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		name = fun.Name
		indexes = k.funcs[pkg][name]
	case *ast.SelectorExpr:
		if importPath, sel := importedSelector(fun, imports); importPath != "" {
			calleePkg, name = importPath, sel
			indexes = k.funcs[importPath][sel]
		} else {
			name = fun.Sel.Name
			indexes = k.methods[pkg][name]
		}
	}
	for _, index := range indexes {
		if index >= len(call.Args) {
			if k.varargs[calleePkg][name] && call.Ellipsis == token.NoPos {
				return true
			}
			continue
		}
		if isNil(call.Args[index]) {
			return true
		}
	}
	return false
}

func apiGap(call *ast.CallExpr, imports map[string]string) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	lit := optionsLiteral(sel.X)
	if lit == nil {
		return false
	}
	typePkg, typeName := importedSelector(lit.Type, imports)
	return followLessAPIs[typePkg][typeName] == sel.Sel.Name
}

func compareFollowAllowances(gaps []followGap, allowed []followAllowance) []string {
	counts := map[followGap]int{}
	for _, gap := range gaps {
		counts[gap]++
	}
	var failures []string
	for _, entry := range allowed {
		if _, ok := followRules[entry.Rule]; !ok {
			failures = append(failures, fmt.Sprintf("invalid rule %q: %+v", entry.Rule, entry.followGap))
		}
		if entry.Reason == "" {
			failures = append(failures, fmt.Sprintf("missing reason: %+v", entry.followGap))
		}
		if counts[entry.followGap] == 0 {
			failures = append(failures, fmt.Sprintf("stale allowance: %+v", entry.followGap))
		} else {
			counts[entry.followGap]--
		}
	}
	for gap, count := range counts {
		if count > 0 {
			failures = append(failures, fmt.Sprintf("discovery without the follow snapshot (%d, %s): %+v", count, followRules[gap.Rule], gap))
		}
	}
	sort.Strings(failures)
	return failures
}

func TestFollowSnapshotGuard(t *testing.T) {
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
	gaps, err := scanFollowGaps(files)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("follow_allowlist.json")
	if err != nil {
		t.Fatal(err)
	}
	var allowed []followAllowance
	if err := json.Unmarshal(data, &allowed); err != nil {
		t.Fatal(err)
	}
	if failures := compareFollowAllowances(gaps, allowed); len(failures) > 0 {
		t.Fatal(strings.Join(failures, "\n"))
	}
}

// followGuardStubs give the scanner the declarations it learns from real code.
var followGuardStubs = map[string]string{
	"internal/sourcewalk/stub.go": `package sourcewalk
type FollowSet struct{}
type Options struct{ Follow *FollowSet }
func Walk(root string, opts Options, fn func()) error { return nil }
func ReadDir(root string, opts Options) error { return nil }`,
	"internal/install/stub.go": `package install
import "skillshare/internal/sourcewalk"
type InstallOptions struct { Update bool; Follow *sourcewalk.FollowSet }
func Repos(dir string, opts sourcewalk.Options) {}
func Pull(dir string, follows ...*sourcewalk.FollowSet) {}`,
	"internal/resource/stub.go": `package resource
type SkillKind struct{}
func (SkillKind) Discover(dir string) {}`,
	"internal/other/stub.go": `package other
import "skillshare/internal/sourcewalk"
func scan() { sourcewalk.ReadDir("x", sourcewalk.Options{}) }`,
}

func scanFollowCommand(t *testing.T, body string) []followGap {
	t.Helper()
	files := map[string][]byte{"cmd/skillshare/example.go": []byte(`package main
import (
	"skillshare/internal/install"
	"skillshare/internal/resource"
	sw "skillshare/internal/sourcewalk"
)
var _ = resource.SkillKind{}
func list(dir string, follow *sw.FollowSet) {}
func run(dir string, follow *sw.FollowSet) {
	_ = install.InstallOptions{}
` + body + `
}`)}
	for name, source := range followGuardStubs {
		files[name] = []byte(source)
	}
	gaps, err := scanFollowGaps(files)
	if err != nil {
		t.Fatal(err)
	}
	return gaps
}

func TestFollowGuardAcceptsThreadedSnapshot(t *testing.T) {
	gaps := scanFollowCommand(t, `
	install.Repos(dir, sw.Options{Follow: follow})
	install.Pull(dir, follow)
	list(dir, follow)
	_ = install.InstallOptions{Update: true, Follow: follow}
	_ = sw.Walk(dir, sw.Options{Follow: follow}, nil)
	_ = sw.ReadDir(dir, sw.Options{Follow: follow})
	_ = follow.Err()`)
	if len(gaps) != 0 {
		t.Fatalf("unexpected gaps outside cmd/server or in threaded calls: %+v", gaps)
	}
}

func TestFollowGuardRejectsFollowLessCalls(t *testing.T) {
	for _, tc := range []struct{ name, body, rule string }{
		{"options literal without Follow", `install.Repos(dir, sw.Options{})`, "options"},
		{"options literal with nil Follow", `install.Repos(dir, sw.Options{Follow: nil})`, "options"},
		{"positional nil Follow", `install.Repos(dir, sw.Options{nil})`, "options"},
		{"update without Follow", `_ = install.InstallOptions{Update: true}`, "options"},
		{"nil snapshot argument", `list(dir, nil)`, "argument"},
		{"omitted variadic snapshot", `install.Pull(dir)`, "argument"},
		{"follow-less API", `resource.SkillKind{}.Discover(dir)`, "api"},
		{"walk without Err check", `_ = sw.Walk(dir, sw.Options{Follow: follow}, nil)`, "err-check"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gaps := scanFollowCommand(t, tc.body)
			if len(gaps) != 1 || gaps[0].Rule != tc.rule || gaps[0].Function != "run" {
				t.Fatalf("want one %s gap in run, got %+v", tc.rule, gaps)
			}
			failures := strings.Join(compareFollowAllowances(gaps, nil), "\n")
			if !strings.Contains(failures, "discovery without the follow snapshot") {
				t.Fatalf("new gap not reported: %q", failures)
			}
		})
	}
}

func TestFollowGuardRejectsStaleAndDuplicateAllowances(t *testing.T) {
	gap := followGap{"cmd/skillshare/example.go", "run", "options", "sw.Options{}"}
	allowed := []followAllowance{{gap, "Reviewed"}}
	if got := compareFollowAllowances([]followGap{gap}, allowed); len(got) != 0 {
		t.Fatal(got)
	}
	for _, tc := range []struct {
		name    string
		gaps    []followGap
		allowed []followAllowance
		want    string
	}{
		{"stale allowance", nil, allowed, "stale allowance"},
		{"duplicate call in allowed function", []followGap{gap, gap}, allowed, "discovery without the follow snapshot (1"},
		{"missing reason", []followGap{gap}, []followAllowance{{gap, ""}}, "missing reason"},
		{"unknown rule", nil, []followAllowance{{followGap{Rule: "other"}, "x"}}, "invalid rule"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := strings.Join(compareFollowAllowances(tc.gaps, tc.allowed), "\n"); !strings.Contains(got, tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
