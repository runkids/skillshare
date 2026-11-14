package skillserve

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"skillshare/internal/config"
	"skillshare/internal/profile"
)

func TestSkillServe_ProfileRecovery(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "alpha/SKILL.md"), skillMD("alpha"))
	writeFile(t, filepath.Join(src, "beta/SKILL.md"), skillMD("beta"))
	catalog := profile.Catalog{Definitions: map[string]profile.Definition{
		"a": {Name: "a", Usable: true, Skills: &profile.SkillSelection{Include: []string{"alpha"}, IncludePresent: true}},
		"b": {Name: "b", Usable: true, Skills: &profile.SkillSelection{Include: []string{"beta"}, IncludePresent: true}},
	}}
	active := "a"
	b := &Builder{Source: src, Target: "claude", Skills: &config.ResourceTargetConfig{}, ResolveProfile: func() (profile.Selector, error) {
		return profile.Resolve(catalog, profile.State{Version: 1, Active: active}, "config.yaml", nil), nil
	}}
	for _, tc := range []struct {
		name        string
		count       int
		uri         string
		unavailable bool
	}{
		{"a", 1, "skill://alpha/SKILL.md", false},
		{"b", 1, "skill://beta/SKILL.md", false},
		{"", 2, "", false},
		{"missing", 0, "", true},
		{"b", 1, "skill://beta/SKILL.md", false},
	} {
		active = tc.name
		got, err := b.Build()
		if tc.unavailable {
			if !errors.Is(err, profile.ErrProfileUnavailable) {
				t.Fatalf("unavailable profile: %v", err)
			}
			continue
		}
		if err != nil || len(got.Skills) != tc.count {
			t.Fatalf("profile %q: %+v, %v", active, got, err)
		}
		if tc.uri != "" && find(got, tc.uri) == nil {
			t.Fatalf("profile %q omitted %s", active, tc.uri)
		}
	}
	untargeted := &Builder{Source: src, ResolveProfile: func() (profile.Selector, error) {
		return profile.Selector{}, errors.New("should not resolve without a target")
	}}
	if got, err := untargeted.Build(); err != nil || len(got.Skills) != 2 {
		t.Fatalf("untargeted catalog: %+v, %v", got, err)
	}
	active = "a"
	svc := &service{b: b, opts: Options{Warn: io.Discard, Refresh: -time.Nanosecond}, warned: map[string]bool{}}
	if _, err := svc.catalog(); err != nil {
		t.Fatal(err)
	}
	active = "missing"
	if _, err := svc.catalog(); !errors.Is(err, profile.ErrProfileUnavailable) || svc.cat != nil {
		t.Fatalf("stale catalog survived unavailable profile: %v", err)
	}
	active = "b"
	if got, err := svc.catalog(); err != nil || find(got, "skill://beta/SKILL.md") == nil {
		t.Fatalf("catalog did not recover: %+v, %v", got, err)
	}
}

// Exercise the protocol on one live connection, not just a new builder per state.
func TestSkillServe_ProfileLiveRecovery(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	src := filepath.Join(root, "skills")
	for _, name := range []string{"alpha", "beta", "excluded"} {
		writeFile(t, filepath.Join(src, name, "SKILL.md"), skillMD(name))
	}
	path := filepath.Join(root, "config.yaml")
	definitions := "profiles:\n  a:\n    skills:\n      include: [alpha]\n  b:\n    skills:\n      include: [beta]\n  future:\n    unknown: true\n"
	writeFile(t, path, definitions+"targets: {}\n")
	statePath, _, err := config.ProfilePaths(path)
	if err != nil {
		t.Fatal(err)
	}
	set := func(active string) {
		t.Helper()
		if err := profile.WriteState(statePath, profile.State{Version: 1, Active: active}); err != nil {
			t.Fatal(err)
		}
	}
	set("a")
	b := &Builder{Source: src, Target: "claude", Skills: &config.ResourceTargetConfig{Exclude: []string{"excluded"}}, ResolveProfile: func() (profile.Selector, error) {
		cfg, err := config.LoadReadOnlyPath(path)
		if err != nil {
			return profile.Selector{}, err
		}
		return cfg.ProfileSelector(), nil
	}}
	srv, err := NewServer(b, Options{Warn: io.Discard, Refresh: time.Millisecond, Edit: &EditOptions{Skills: []string{"alpha"}}})
	if err != nil {
		t.Fatal(err)
	}
	ct, st := mcp.NewInMemoryTransports()
	ss, err := srv.Connect(context.Background(), st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ss.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "profile-test", Version: "test"}, nil)
	if err := mcp.AddSendingCustomMethod[*listSkillsParams, *listSkillsResult](client, methodList); err != nil {
		t.Fatal(err)
	}
	cs, err := client.Connect(context.Background(), ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	for _, active := range []string{"a", "b", "", "missing", "future", "unreadable", "b"} {
		set(active)
		if active == "unreadable" {
			if err := os.Remove(statePath); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(statePath, 0700); err != nil {
				t.Fatal(err)
			}
		}
		// Let the configured refresh interval expire, then require this connection to see the new state.
		time.Sleep(2 * time.Millisecond)
		got, err := mcp.CallCustomMethod[*listSkillsParams, *listSkillsResult](context.Background(), cs, methodList, &listSkillsParams{})
		if active == "missing" || active == "future" || active == "unreadable" {
			if err == nil || !strings.Contains(err.Error(), "profile use") {
				t.Fatalf("%s served stale data: %+v, %v", active, got, err)
			}
			if active == "unreadable" {
				if err := os.Remove(statePath); err != nil {
					t.Fatal(err)
				}
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		want := 1
		if active == "" {
			want = 2
		}
		if len(got.Skills) != want {
			t.Fatalf("%s: %+v", active, got.Skills)
		}
		wantNames := []string{"alpha"}
		if active == "b" {
			wantNames = []string{"beta"}
		} else if active == "" {
			wantNames = []string{"alpha", "beta"}
		}
		for _, name := range wantNames {
			found := false
			for _, skill := range got.Skills {
				found = found || skill.URI == "skill://"+name+"/SKILL.md"
			}
			if !found {
				t.Fatalf("%s omitted %s: %+v", active, name, got.Skills)
			}
		}
		if len(srv.svc.edit.Skills) != 1 || srv.svc.edit.Skills[0] != "alpha" || len(b.Skills.Exclude) != 1 {
			t.Fatal("refresh changed startup settings or edit authorization")
		}
	}
}
