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

func TestFindRoot(t *testing.T) {
	cwd, home := rootContext(t)
	other := filepath.Join(home, "elsewhere")
	projects := map[string]config.ManagedProject{
		"~/work/app/": {Name: "local"},
		other:         {Name: "app"},
		"~":           {},
	}
	for _, tt := range []struct{ token, want string }{
		{"app", "~/work/app/"},
		{"./app", "~/work/app/"},
		{"../work/app", "~/work/app/"},
		{filepath.Join(cwd, "app"), "~/work/app/"},
		{"~/work/app///", "~/work/app/"},
		{"..", "~"},
	} {
		t.Run(tt.token, func(t *testing.T) {
			got, err := FindRoot(projects, tt.token, cwd, home)
			if err != nil || got != tt.want {
				t.Fatalf("FindRoot(%q) = %q, %v; want %q", tt.token, got, err, tt.want)
			}
		})
	}
	if _, err := FindRoot(projects, "local", cwd, home); !errors.Is(err, ErrNotFound) {
		t.Fatalf("FindRoot matched name: %v", err)
	}
	if _, err := FindRoot(projects, "missing", cwd, home); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing root: %v", err)
	}
	if _, err := FindRoot(projects, "~user/app", cwd, home); err == nil || errors.Is(err, ErrNotFound) {
		t.Errorf("invalid root must return validation error: %v", err)
	}
}

func TestFindRootAmbiguity(t *testing.T) {
	cwd, home := rootContext(t)
	key := filepath.Join(cwd, "app")
	projects := map[string]config.ManagedProject{key: {Name: "absolute"}, "~/work/app/": {Name: "tilde"}}
	for i := 0; i < 20; i++ {
		_, err := FindRoot(projects, "./app", cwd, home)
		assertAmbiguous(t, err, []string{key, "~/work/app/"})
	}
}

func TestFindRootWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native Windows comparisons")
	}
	projects := map[string]config.ManagedProject{`C:\Work\App`: {Name: "upper"}, `c:\work\app`: {Name: "lower"}}
	_, err := FindRoot(projects, `C:/WORK/App/`, `C:\Work`, `C:\Users\Me`)
	assertAmbiguous(t, err, []string{`C:\Work\App`, `c:\work\app`})
}

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
