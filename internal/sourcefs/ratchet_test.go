package sourcefs

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The ratchet keeps raw os write calls a deliberate, reviewed choice. Every
// call in a non-test file under cmd/ and internal/ must have an allowance in
// testdata/raw_writes.tsv with a reason that says why it does not write the
// skills source, or source-unmigrated when it does and still has to move onto
// this package. The check is syntactic: it cannot follow data flow.
//
// To regenerate the list after adding or removing calls, run
//
//	SOURCEFS_RATCHET_UPDATE=1 go test ./internal/sourcefs -run TestRawWriteRatchet
//
// It keeps the reasons of existing entries and marks new ones unclassified,
// which still fails until a reviewer gives each one a reason.

const allowlistPath = "testdata/raw_writes.tsv"

// rawWriteFuncs are the os functions that write the filesystem.
var rawWriteFuncs = map[string]bool{
	"Create": true, "CreateTemp": true, "MkdirTemp": true, "OpenFile": true,
	"WriteFile": true, "Mkdir": true, "MkdirAll": true, "Remove": true,
	"RemoveAll": true, "Rename": true, "Symlink": true, "Link": true,
	"Truncate": true, "Chmod": true,
}

// validReasons are the tags an allowance may carry.
var validReasons = map[string]string{
	"sourcefs":          "this package: the handle's own root creation and edge-crossing rename",
	"source-checked":    "writes the source after the in-source side was checked through sourcefs",
	"source-unmigrated": "writes the skills source and still has to move onto sourcefs",
	"source-root":       "creates or moves the skills source root itself, before a handle can be opened on it",
	"target":            "an AI tool's skills, agents, or rules directory",
	"backup":            "the backup directory",
	"trash":             "the trash directory",
	"config":            "skillshare's own config, state, registry, cache, or log files",
	"agents":            "the agents source",
	"extras":            "an extras source or its targets",
	"clone":             "a fresh clone or download directory outside the source",
	"plugin":            "plugin, MCP, hooks, or instructions files of a tool",
	"tmp":               "a temporary file or directory",
	"project":           "a project's own files outside the skills source",
	"shell":             "a shell completion script",
	"binary":            "the skillshare executable during upgrade",
	"devnull":           "os.DevNull",
	"test-helper":       "test support code that writes sandboxes",
}

type rawWrite struct {
	File, Func, Callee, Arg string
}

type allowance struct {
	rawWrite
	Reason string
}

func (w rawWrite) String() string {
	return fmt.Sprintf("%s\t%s\t%s\t%s", w.File, w.Func, w.Callee, w.Arg)
}

// scanRawWrites parses the non-test .go files under each dir, relative to
// root, and returns every call or reference to a raw os write function.
func scanRawWrites(root string, dirs ...string) ([]rawWrite, error) {
	var out []rawWrite
	fset := token.NewFileSet()
	for _, dir := range dirs {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" || d.Name() == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			out = append(out, fileRawWrites(fset, filepath.ToSlash(rel), file)...)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// fileRawWrites finds raw writes in one file. Matching follows the file's
// import of "os", so an aliased import counts too.
func fileRawWrites(fset *token.FileSet, rel string, file *ast.File) []rawWrite {
	osName := ""
	for _, imp := range file.Imports {
		if p, _ := strconv.Unquote(imp.Path.Value); p == "os" {
			osName = "os"
			if imp.Name != nil {
				osName = imp.Name.Name
			}
		}
	}
	if osName == "" || osName == "_" || osName == "." {
		return nil
	}
	isRawWrite := func(sel *ast.SelectorExpr) bool {
		id, ok := sel.X.(*ast.Ident)
		return ok && id.Name == osName && rawWriteFuncs[sel.Sel.Name]
	}

	var out []rawWrite
	record := func(fn string, sel *ast.SelectorExpr, args []ast.Expr) {
		out = append(out, rawWrite{File: rel, Func: fn, Callee: "os." + sel.Sel.Name, Arg: argFingerprint(fset, sel.Sel.Name, args)})
	}
	visit := func(fn string, node ast.Node) {
		called := map[*ast.SelectorExpr]bool{}
		ast.Inspect(node, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CallExpr:
				if sel, ok := n.Fun.(*ast.SelectorExpr); ok && isRawWrite(sel) {
					called[sel] = true
					record(fn, sel, n.Args)
				}
			case *ast.SelectorExpr:
				// A function value such as `var removeAll = os.RemoveAll`
				// is a raw write too.
				if isRawWrite(n) && !called[n] {
					record(fn, n, nil)
				}
			}
			return true
		})
	}
	for _, decl := range file.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok {
			visit(funcName(fd), fd)
		} else {
			visit("<package>", decl)
		}
	}
	return out
}

func funcName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fd.Name.Name
	}
	t := fd.Recv.List[0].Type
	star := ""
	if s, ok := t.(*ast.StarExpr); ok {
		star, t = "*", s.X
	}
	if idx, ok := t.(*ast.IndexExpr); ok {
		t = idx.X
	}
	if id, ok := t.(*ast.Ident); ok {
		return "(" + star + id.Name + ")." + fd.Name.Name
	}
	return fd.Name.Name
}

// argFingerprint is the source of the path arguments, with whitespace
// collapsed. Rename, Link, and Symlink write both of theirs.
func argFingerprint(fset *token.FileSet, callee string, args []ast.Expr) string {
	if args == nil {
		return "<ref>"
	}
	n := 1
	switch callee {
	case "Rename", "Link", "Symlink":
		n = 2
	}
	var parts []string
	for i := 0; i < n && i < len(args); i++ {
		var buf bytes.Buffer
		_ = printer.Fprint(&buf, fset, args[i])
		parts = append(parts, strings.Join(strings.Fields(buf.String()), " "))
	}
	s := strings.Join(parts, ", ")
	if len(s) > 100 {
		sum := sha256.Sum256([]byte(s))
		s = s[:80] + "…" + hex.EncodeToString(sum[:4])
	}
	return s
}

func loadAllowlist(path string) ([]allowance, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []allowance
	sc := bufio.NewScanner(f)
	for line := 1; sc.Scan(); line++ {
		text := sc.Text()
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		cols := strings.Split(text, "\t")
		if len(cols) != 5 {
			return nil, fmt.Errorf("%s:%d: want 5 tab-separated columns, got %d", path, line, len(cols))
		}
		out = append(out, allowance{rawWrite{cols[0], cols[1], cols[2], cols[3]}, cols[4]})
	}
	return out, sc.Err()
}

// diffAllowlist matches found calls against allowances as a multiset: each
// allowance covers one call, so a second identical call in the same function
// is new.
func diffAllowlist(found []rawWrite, allowed []allowance) (unlisted []rawWrite, stale []allowance) {
	left := map[rawWrite][]allowance{}
	for _, a := range allowed {
		left[a.rawWrite] = append(left[a.rawWrite], a)
	}
	for _, w := range found {
		if as := left[w]; len(as) > 0 {
			left[w] = as[1:]
			continue
		}
		unlisted = append(unlisted, w)
	}
	for _, as := range left {
		stale = append(stale, as...)
	}
	sort.Slice(stale, func(i, j int) bool { return stale[i].String() < stale[j].String() })
	return unlisted, stale
}

func writeAllowlist(path string, found []rawWrite, allowed []allowance) error {
	reasons := map[rawWrite][]string{}
	for _, a := range allowed {
		reasons[a.rawWrite] = append(reasons[a.rawWrite], a.Reason)
	}
	sorted := append([]rawWrite(nil), found...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].String() < sorted[j].String() })
	var buf bytes.Buffer
	buf.WriteString("# Raw os write calls allowed outside sourcefs. See ratchet_test.go.\n")
	buf.WriteString("# file\tfunction\tcallee\tpath argument\treason\n")
	for _, w := range sorted {
		reason := "unclassified"
		if rs := reasons[w]; len(rs) > 0 {
			reason, reasons[w] = rs[0], rs[1:]
		}
		fmt.Fprintf(&buf, "%s\t%s\n", w, reason)
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func TestRawWriteRatchet(t *testing.T) {
	found, err := scanRawWrites(filepath.Join("..", ".."), "cmd", "internal")
	if err != nil {
		t.Fatal(err)
	}
	allowed, err := loadAllowlist(allowlistPath)
	if err != nil && !(os.Getenv("SOURCEFS_RATCHET_UPDATE") != "" && os.IsNotExist(err)) {
		t.Fatal(err)
	}
	if os.Getenv("SOURCEFS_RATCHET_UPDATE") != "" {
		if err := writeAllowlist(allowlistPath, found, allowed); err != nil {
			t.Fatal(err)
		}
		if allowed, err = loadAllowlist(allowlistPath); err != nil {
			t.Fatal(err)
		}
	}

	for _, a := range allowed {
		if _, ok := validReasons[a.Reason]; !ok {
			t.Errorf("%s: reason %q is not one of the valid reasons in ratchet_test.go", a.rawWrite, a.Reason)
		}
	}
	unlisted, stale := diffAllowlist(found, allowed)
	for _, w := range unlisted {
		t.Errorf("new raw os write call: %s\n\twrite the skills source through sourcefs, or add an allowance with a truthful reason to %s", w, allowlistPath)
	}
	for _, a := range stale {
		t.Errorf("stale allowance, the call no longer exists: %s", a.rawWrite)
	}
}

func TestRatchetFlagsNewCallsAndStaleAllowances(t *testing.T) {
	root := t.TempDir()
	src := `package p

import xos "os"

func allowed() {
	_ = xos.WriteFile("a", nil, 0o644)
	_ = xos.WriteFile("a", nil, 0o644)
}

var remove = xos.Remove
`
	if err := os.MkdirAll(filepath.Join(root, "internal", "p"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "p", "p.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "p", "p_test.go"), []byte("package p\n\nimport \"os\"\n\nvar _ = os.Remove\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	found, err := scanRawWrites(root, "internal")
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 3 {
		t.Fatalf("found %v, want two aliased calls and one reference", found)
	}

	write := rawWrite{"internal/p/p.go", "allowed", "os.WriteFile", `"a"`}
	ref := rawWrite{"internal/p/p.go", "<package>", "os.Remove", "<ref>"}
	gone := rawWrite{"internal/p/p.go", "allowed", "os.Rename", `"a", "b"`}

	// One allowance for two identical calls: the second one is new, even
	// inside a function that is already on the list.
	unlisted, stale := diffAllowlist(found, []allowance{{write, "tmp"}, {ref, "tmp"}})
	if len(unlisted) != 1 || unlisted[0] != write || len(stale) != 0 {
		t.Fatalf("unlisted = %v, stale = %v; want the second WriteFile only", unlisted, stale)
	}

	// An allowance whose call no longer exists is stale.
	unlisted, stale = diffAllowlist(found, []allowance{{write, "tmp"}, {write, "tmp"}, {ref, "tmp"}, {gone, "tmp"}})
	if len(unlisted) != 0 || len(stale) != 1 || stale[0].rawWrite != gone {
		t.Fatalf("unlisted = %v, stale = %v; want the Rename allowance stale", unlisted, stale)
	}
}
