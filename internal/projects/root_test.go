package projects

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func rootContext(t *testing.T) (cwd, home string) {
	t.Helper()
	home = filepath.Join(t.TempDir(), "home")
	return filepath.Join(home, "work"), home
}

func TestRootIdentity(t *testing.T) {
	cwd, home := rootContext(t)
	sep := string(filepath.Separator)
	volumeRoot := filepath.VolumeName(cwd) + sep
	absTyped := home + sep + "work" + sep + ".." + sep + "app"
	for _, tt := range []struct{ name, arg, identity string }{
		{"home", "~", home},
		{"home separators", "~/", home},
		{"tilde clean", "~/work/../app///", filepath.Join(home, "app")},
		{"relative", "app", filepath.Join(cwd, "app")},
		{"dot", ".", cwd},
		{"parent", "../app/", filepath.Join(home, "app")},
		{"absolute spelling", absTyped + sep + sep, filepath.Join(home, "app")},
		{"absolute home", home, home},
		{"relative home", "..", home},
		{"outside home", "../../outside", filepath.Join(home, "..", "outside")},
		{"volume root", volumeRoot, volumeRoot},
		{"volume root separators", volumeRoot + sep, volumeRoot},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Identity(tt.arg, cwd, home)
			if err != nil || got != tt.identity {
				t.Errorf("Identity(%q) = %q, %v; want %q", tt.arg, got, err, tt.identity)
			}
		})
	}
}

func TestRootInvalidInputs(t *testing.T) {
	cwd, home := rootContext(t)
	for _, arg := range []string{"", "~user", "~user/app", "a\x00b"} {
		t.Run(arg, func(t *testing.T) {
			if _, err := Identity(arg, cwd, home); err == nil {
				t.Errorf("Identity(%q) accepted invalid root", arg)
			}
		})
	}
}

func TestRootIdentityUsesExplicitHome(t *testing.T) {
	cwd, home := rootContext(t)
	t.Setenv("HOME", filepath.Join(t.TempDir(), "different-home"))
	t.Setenv("USERPROFILE", filepath.Join(t.TempDir(), "different-profile"))
	got, err := Identity("~/app", cwd, home)
	if err != nil || got != filepath.Join(home, "app") {
		t.Fatalf("Identity uses process home: %q, %v", got, err)
	}
}

func TestRootIdentityKeepsSymlinkAliases(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("lexical alias behavior is platform independent; symlink privileges vary on Windows")
	}
	base := t.TempDir()
	real, alias := filepath.Join(base, "real"), filepath.Join(base, "alias")
	if err := os.Mkdir(real, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, alias); err != nil {
		t.Fatal(err)
	}
	got, err := Identity(alias+"/child/..", base, base)
	if err != nil || got != alias {
		t.Fatalf("Identity resolved symlink: %q, %v; want %q", got, err, alias)
	}
}

func TestRootWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native Windows path semantics")
	}
	for _, tt := range []struct{ arg, identity string }{
		{`C:\Work\App\\`, `C:\Work\App`},
		{`c:/Work/../App//`, `c:\App`},
		{`C:\`, `C:\`},
		{`C:/`, `C:\`},
		{`\\server\share\App\\`, `\\server\share\App`},
		{`\\server\share\`, `\\server\share`},
		{`\\server\share`, `\\server\share`},
		{`~\App\`, `C:\Users\Me\App`},
		{`.\App`, `C:\Work\App`},
		{`\App`, `C:\App`},
		{`/App`, `C:\App`},
	} {
		got, err := Identity(tt.arg, `C:\Work`, `C:\Users\Me`)
		if err != nil || got != tt.identity {
			t.Errorf("Identity(%q) = %q, %v; want %q", tt.arg, got, err, tt.identity)
		}
	}
	for _, arg := range []string{`C:`, `C:App`, `C:..\App`, `~user\App`} {
		if _, err := Identity(arg, `C:\Work`, `C:\Users\Me`); err == nil {
			t.Errorf("Identity accepted %q", arg)
		}
	}
}

func TestStoredKey(t *testing.T) {
	cwd, home := rootContext(t)
	sep := string(filepath.Separator)
	volumeRoot := filepath.VolumeName(cwd) + sep
	absTyped := home + sep + "work" + sep + ".." + sep + "app"
	for _, tt := range []struct{ name, arg, stored string }{
		{"home", "~", "~"},
		{"home separators", "~/", "~"},
		{"tilde clean", "~/work/../app///", "~/work/../app"},
		{"relative", "app", filepath.Join("~", "work", "app")},
		{"dot", ".", filepath.Join("~", "work")},
		{"parent", "../app/", filepath.Join("~", "app")},
		{"absolute spelling", absTyped + sep + sep, absTyped},
		{"absolute home", home, home},
		{"relative home", "..", "~"},
		{"outside home", "../../outside", filepath.Join(home, "..", "outside")},
		{"volume root", volumeRoot, volumeRoot},
		{"volume root separators", volumeRoot + sep, volumeRoot},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := StoredKey(tt.arg, cwd, home)
			if err != nil || got != tt.stored {
				t.Errorf("StoredKey(%q) = %q, %v; want %q", tt.arg, got, err, tt.stored)
			}
		})
	}
	for _, arg := range []string{"", "~user/app"} {
		if _, err := StoredKey(arg, cwd, home); err == nil {
			t.Errorf("StoredKey(%q) accepted invalid root", arg)
		}
	}
}

func TestStoredKeyHomeBoundaries(t *testing.T) {
	cwd, home := rootContext(t)
	sep := string(filepath.Separator)
	volumeRoot := filepath.VolumeName(cwd) + sep
	for _, tt := range []struct{ name, arg, cwd, home, want string }{
		{"home trailing separators", "app", cwd, home + sep + sep, filepath.Join("~", "work", "app")},
		{"home prefix sibling", ".", home + "-other", home, home + "-other"},
		{"home at volume root", "app", volumeRoot, volumeRoot, "~" + sep + "app"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := StoredKey(tt.arg, tt.cwd, tt.home)
			if err != nil || got != tt.want {
				t.Fatalf("StoredKey(%q) = %q, %v; want %q", tt.arg, got, err, tt.want)
			}
		})
	}
}

func TestStoredKeyWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native Windows path semantics")
	}
	for _, tt := range []struct{ arg, stored string }{
		{`C:\Work\App\\`, `C:\Work\App`},
		{`c:/Work/../App//`, `c:/Work/../App`},
		{`C:\`, `C:\`},
		{`C:/`, `C:/`},
		{`\\server\share\App\\`, `\\server\share\App`},
		{`\\server\share\`, `\\server\share`},
		{`\\server\share`, `\\server\share`},
		{`~\App\`, `~\App`},
		{`.\App`, `C:\Work\App`},
		{`\App`, `C:\App`},
		{`/App`, `C:\App`},
	} {
		got, err := StoredKey(tt.arg, `C:\Work`, `C:\Users\Me`)
		if err != nil || got != tt.stored {
			t.Errorf("StoredKey(%q) = %q, %v; want %q", tt.arg, got, err, tt.stored)
		}
	}
}
