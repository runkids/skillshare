package profile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestProfileState_StrictAndAtomic(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "profiles")
	path := filepath.Join(dir, "state.yaml")
	state, err := ReadState(path)
	if err != nil || state.Version != 1 || state.Active != "" || state.Pending {
		t.Fatalf("missing state: %+v, %v", state, err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("read created state directory: %v", err)
	}
	want := State{Version: 1, Active: "frontend", Pending: true}
	if err := WriteState(path, want); err != nil {
		t.Fatal(err)
	}
	state, err = ReadState(path)
	if err != nil || state != want {
		t.Fatalf("round trip: %+v, %v", state, err)
	}
	if err := WriteState(path, State{Version: 2, Active: "bad"}); err == nil {
		t.Fatal("unsupported state written")
	}
	state, err = ReadState(path)
	if err != nil || state != want {
		t.Fatalf("failed write changed state: %+v, %v", state, err)
	}
	for _, p := range []struct{ path string; mode os.FileMode }{{dir, 0700}, {path, 0600}} {
		info, err := os.Stat(p.path)
		if err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != p.mode {
			t.Fatalf("permissions for %s: %v", p.path, info.Mode())
		}
	}
	for _, data := range []string{"version: 2\n", "version: 1\nunknown: true\n", "version: [\n", "version: 1\n---\nversion: 1\n", "active: frontend\n"} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadState(path); err == nil {
			t.Errorf("accepted invalid state %q", data)
		}
	}
	if _, err := ReadState(dir); err == nil {
		t.Fatal("directory accepted as state")
	}
}

func TestProfileLock_SerializesAndTimesOut(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles", "config.lock")
	unlock, err := Lock(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	other, err := Lock(context.Background(), filepath.Join(filepath.Dir(path), "other.lock"))
	if err != nil {
		unlock()
		t.Fatalf("independent config lock: %v", err)
	}
	other()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if release, err := Lock(ctx, path); !errors.Is(err, context.DeadlineExceeded) {
		if release != nil {
			release()
		}
		t.Fatalf("contended lock: %v", err)
	} else if !strings.Contains(err.Error(), "sync or profile switch") {
		t.Fatalf("lock error is not actionable: %v", err)
	}
	unlock()
	unlock()
	release, err := Lock(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	release()
}

func TestProfileSelector_FailsClosed(t *testing.T) {
	catalog := Catalog{Definitions: map[string]Definition{
		"frontend": {Name: "frontend", Usable: true, Skills: &SkillSelection{Include: []string{"web*"}, IncludePresent: true}},
		"all": {Name: "all", Usable: true},
		"empty": {Name: "empty", Usable: true, Skills: &SkillSelection{Include: []string{}, IncludePresent: true}},
		"future": {Name: "future", Reason: "unknown key"},
	}}
	for _, tc := range []struct {
		state State
		err error
		name string
		want, unavailable bool
	}{
		{State{Version: 1}, nil, "anything", true, false},
		{State{Version: 1, Active: "frontend"}, nil, "web-ui", true, false},
		{State{Version: 1, Active: "frontend"}, nil, "server", false, false},
		{State{Version: 1, Active: "all"}, nil, "server", true, false},
		{State{Version: 1, Active: "empty"}, nil, "server", false, false},
		{State{Version: 1, Active: "missing"}, nil, "web-ui", false, true},
		{State{Version: 1, Active: "future"}, nil, "web-ui", false, true},
		{State{}, errors.New("unreadable"), "web-ui", false, true},
	} {
		s := Resolve(catalog, tc.state, "/config.yaml", tc.err)
		got, err := s.Selects(tc.name)
		if got != tc.want || errors.Is(err, ErrProfileUnavailable) != tc.unavailable {
			t.Errorf("state %+v: got %v, %v", tc.state, got, err)
		}
	}
}

func TestProfileLock_NeverUnlinked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles", "config.lock")
	release, err := Lock(context.Background(), path)
	if err != nil { t.Fatal(err) }
	first, err := os.Stat(path)
	if err != nil { release(); t.Fatal(err) }
	release()
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(first, after) || after.Size() != 0 { t.Fatalf("lock path was replaced or removed: %v", err) }
	release, err = Lock(context.Background(), path)
	if err != nil { t.Fatal(err) }
	defer release()
	second, err := os.Stat(path)
	if err != nil || !os.SameFile(first, second) { t.Fatalf("relock changed the lock file: %v", err) }
}
