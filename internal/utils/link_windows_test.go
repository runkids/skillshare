//go:build windows

package utils

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestCreateJunctionPathWithCmdMetacharacters(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "target&more%TEMP%")
	os.MkdirAll(target, 0755)
	link := filepath.Join(base, "link&echo%TEMP%")

	if err := CreateJunction(link, target); err != nil {
		t.Fatal(err)
	}
	if !IsJunction(link) {
		t.Fatal("link is not a junction")
	}
	if got, err := ResolveLinkTarget(link); err != nil || got != target {
		t.Fatalf("target = %q, %v; want %q", got, err, target)
	}
}

func TestCreateJunctionExtendedPathTarget(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "target")
	os.MkdirAll(target, 0755)
	os.WriteFile(filepath.Join(target, "marker"), []byte("x"), 0644)
	link := filepath.Join(base, "link")

	if err := CreateJunction(link, `\\?\`+target); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(link, "marker")); err != nil {
		t.Fatalf("cannot read through the junction: %v", err)
	}
}

func TestCreateJunctionRefusesNetworkPath(t *testing.T) {
	link := filepath.Join(t.TempDir(), "link")

	if err := CreateJunction(link, `\\localhost\C$\Users`); err == nil {
		t.Fatal("expected an error for a UNC target")
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatalf("link was left behind: %v", err)
	}
}

func TestCanonicalPathShortAndLongNames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "directory with a long name")
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}
	long := canonicalSpelling(path)
	name, err := syscall.UTF16PtrFromString(long)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]uint16, 260)
	var short string
	for {
		n, err := syscall.GetShortPathName(name, &buf[0], uint32(len(buf)))
		if err != nil {
			t.Fatal(err)
		}
		if n < uint32(len(buf)) {
			short = syscall.UTF16ToString(buf[:n])
			break
		}
		buf = make([]uint16, n+1)
	}
	if PathsEqual(short, long) {
		t.Skip("8.3 aliases are unavailable on this volume")
	}
	for _, tail := range []string{"", "not-created/child"} {
		a, b := filepath.Join(short, tail), filepath.Join(long, tail)
		if !PathsEqual(CanonicalPath(a), CanonicalPath(b)) || !SamePhysicalPath(a, b) {
			t.Fatalf("short/long identities differ: %q -> %q, %q -> %q", a, CanonicalPath(a), b, CanonicalPath(b))
		}
	}
}
