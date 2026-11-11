package gitenv

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestEnvironDropsRepositorySelection(t *testing.T) {
	t.Setenv("GIT_DIR", "/caller/.git")
	t.Setenv("GIT_WORK_TREE", "/caller")
	t.Setenv("GIT_INDEX_FILE", "/caller/.git/index")
	t.Setenv("git_common_dir", "/caller/.git") // Windows names are case-insensitive.

	for _, item := range Environ() {
		key, _, _ := strings.Cut(item, "=")
		if slices.Contains([]string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR"}, strings.ToUpper(key)) {
			t.Fatalf("Environ() kept %s", item)
		}
	}
}

// Users pass credentials through GIT_CONFIG_COUNT, and install's token auth
// appends to it.
func TestEnvironKeepsConfigVariables(t *testing.T) {
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "http.proxy")
	t.Setenv("GIT_CONFIG_VALUE_0", "http://proxy")

	env := Environ()
	for _, want := range []string{"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=http.proxy", "GIT_CONFIG_VALUE_0=http://proxy"} {
		if !slices.Contains(env, want) {
			t.Errorf("Environ() dropped %s", want)
		}
	}
}

// rawGitAllowed lists the only places that may start git with os/exec.
var rawGitAllowed = map[string]string{
	"internal/gitenv/":   "the wrapper itself",
	"internal/hooks/":    "gitcmd.go builds its own sanitized environment",
	"internal/testutil/": "test fixtures",
}

// A git command started with os/exec inherits a hook's GIT_DIR and runs
// against the hook's repository (#852), so skillshare's own git goes through
// Command. The check is syntactic: it finds exec.Command and
// exec.CommandContext calls whose program is the literal "git".
func TestNoRawGitCommands(t *testing.T) {
	root := filepath.Join("..", "..")
	fset := token.NewFileSet()
	for _, dir := range []string{"cmd", "internal"} {
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
			rel = filepath.ToSlash(rel)
			for prefix := range rawGitAllowed {
				if strings.HasPrefix(rel, prefix) {
					return nil
				}
			}
			file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			for _, pos := range rawGitCalls(file) {
				t.Errorf("%s:%d runs git with os/exec; use gitenv.Command", rel, fset.Position(pos).Line)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// rawGitCalls follows the file's import of os/exec, so an aliased import counts.
func rawGitCalls(file *ast.File) []token.Pos {
	execName := ""
	for _, imp := range file.Imports {
		if p, _ := strconv.Unquote(imp.Path.Value); p == "os/exec" {
			execName = "exec"
			if imp.Name != nil {
				execName = imp.Name.Name
			}
		}
	}
	if execName == "" {
		return nil
	}
	var out []token.Pos
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != execName {
			return true
		}
		prog := 0
		switch sel.Sel.Name {
		case "Command":
		case "CommandContext":
			prog = 1
		default:
			return true
		}
		if len(call.Args) > prog {
			if lit, ok := call.Args[prog].(*ast.BasicLit); ok && lit.Value == `"git"` {
				out = append(out, call.Pos())
			}
		}
		return true
	})
	return out
}
