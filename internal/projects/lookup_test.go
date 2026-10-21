package projects

import (
	"errors"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"skillshare/internal/config"
)

func assertAmbiguous(t *testing.T, err error, want []string) {
	t.Helper()
	var ambiguous *AmbiguousError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("error = %v; want *AmbiguousError", err)
	}
	if !reflect.DeepEqual(ambiguous.Candidates, want) {
		t.Errorf("candidates = %q; want sorted declared keys %q", ambiguous.Candidates, want)
	}
	for _, key := range want {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("ambiguity error %q omits declared key %q", err, key)
		}
	}
}

func TestFind_DefaultNameUsesDeclaredSpelling(t *testing.T) {
	cwd := t.TempDir()
	home := t.TempDir()
	key := filepath.Join(home, "parent", "child") + string(filepath.Separator) + ".."
	entries := map[string]config.ManagedProject{key: {Targets: []string{"claude"}}}
	name := entries[key].ProjectName(key)
	got, err := Find(entries, name, cwd, home)
	if err != nil || got != key {
		t.Fatalf("displayed name %q did not resolve exact declared key: got %q, %v", name, got, err)
	}
}

func TestFindNameAndRoot(t *testing.T) {
	cwd, home := rootContext(t)
	other := filepath.Join(home, "elsewhere")
	projects := map[string]config.ManagedProject{
		"~/work/app/": {Name: "local"},
		other:         {Name: "app"},
		"~/default/":  {},
		"~":           {},
	}
	for _, tt := range []struct{ token, want string }{
		{"local", "~/work/app/"},
		{"./app", "~/work/app/"},
		{"../work/app", "~/work/app/"},
		{filepath.Join(cwd, "app"), "~/work/app/"},
		{"~/work/app///", "~/work/app/"},
		{"default", "~/default/"},
		{filepath.Base(home), "~"},
	} {
		t.Run(tt.token, func(t *testing.T) {
			got, err := Find(projects, tt.token, cwd, home)
			if err != nil || got != tt.want {
				t.Fatalf("Find(%q) = %q, %v; want %q", tt.token, got, err, tt.want)
			}
		})
	}
	_, err := Find(projects, "app", cwd, home)
	assertAmbiguous(t, err, []string{other, "~/work/app/"})
	got, err := FindRoot(projects, "app", cwd, home)
	if err != nil || got != "~/work/app/" {
		t.Fatalf("FindRoot treated app as a name: %q, %v", got, err)
	}
	if got, err := FindRoot(projects, "..", cwd, home); err != nil || got != "~" {
		t.Fatalf("FindRoot parent = %q, %v; want ~", got, err)
	}
	if _, err := FindRoot(projects, "local", cwd, home); !errors.Is(err, ErrNotFound) {
		t.Fatalf("FindRoot matched name: %v", err)
	}
	for _, find := range []func(map[string]config.ManagedProject, string, string, string) (string, error){Find, FindRoot} {
		if _, err := find(projects, "missing", cwd, home); !errors.Is(err, ErrNotFound) {
			t.Errorf("missing root: %v", err)
		}
		if _, err := find(projects, "~user/app", cwd, home); err == nil || errors.Is(err, ErrNotFound) {
			t.Errorf("invalid root must return validation error: %v", err)
		}
	}
}

func TestFindAmbiguityAndDeduplication(t *testing.T) {
	cwd, home := rootContext(t)
	key := filepath.Join(cwd, "app")
	got, err := Find(map[string]config.ManagedProject{key: {}}, "app", cwd, home)
	if err != nil || got != key {
		t.Fatalf("same name and root must select one entry: %q, %v", got, err)
	}
	projects := map[string]config.ManagedProject{key: {Name: "shared"}, "~/elsewhere": {Name: "shared"}}
	_, err = Find(projects, "shared", cwd, home)
	assertAmbiguous(t, err, []string{key, "~/elsewhere"})
	projects = map[string]config.ManagedProject{key: {Name: "absolute"}, "~/work/app/": {Name: "tilde"}}
	for i := 0; i < 20; i++ {
		_, err = FindRoot(projects, "./app", cwd, home)
		assertAmbiguous(t, err, []string{key, "~/work/app/"})
	}
	got, err = Find(projects, "tilde", cwd, home)
	if err != nil || got != "~/work/app/" {
		t.Fatalf("unique name did not disambiguate: %q, %v", got, err)
	}
}

func TestFindExplicitRootEscapes(t *testing.T) {
	cwd, home := rootContext(t)
	root := filepath.Join(cwd, "app")
	for _, token := range []string{"./app", "../work/app", root, "~/work/app"} {
		projects := map[string]config.ManagedProject{root: {}, "~/elsewhere": {Name: token}}
		got, err := Find(projects, token, cwd, home)
		if err != nil || got != root {
			t.Errorf("explicit root %q matched a name: %q, %v", token, got, err)
		}
	}
}

func TestFindWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native Windows comparisons and escapes")
	}
	projects := map[string]config.ManagedProject{`C:\Work\App`: {Name: "upper"}, `c:\work\app`: {Name: "lower"}}
	_, err := FindRoot(projects, `C:/WORK/App/`, `C:\Work`, `C:\Users\Me`)
	assertAmbiguous(t, err, []string{`C:\Work\App`, `c:\work\app`})
	got, err := Find(projects, "lower", `C:\Work`, `C:\Users\Me`)
	if err != nil || got != `c:\work\app` {
		t.Fatalf("unique name: %q, %v", got, err)
	}
	for _, token := range []string{`.\App`, `..\Work\App`, `C:\Work\App`, `\\server\share\App`} {
		root := `C:\Work\App`
		if strings.HasPrefix(token, `\\`) {
			root = token
		}
		got, err := Find(map[string]config.ManagedProject{root: {}, `D:\other`: {Name: token}}, token, `C:\Work`, `C:\Users\Me`)
		if err != nil || got != root {
			t.Errorf("root escape %q: %q, %v", token, got, err)
		}
	}
}
