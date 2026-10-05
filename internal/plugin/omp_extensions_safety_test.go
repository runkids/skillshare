package plugin

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func ompPreviewOne(t *testing.T, s *Service, target string, key string, enabled bool) (*OMPExtensionsPlan, []OMPExtensionChange) {
	t.Helper()
	v, err := s.OMPExtensions(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	if key == "" {
		key = v.Rows[0].Key
	}
	changes := []OMPExtensionChange{{Key: key, Enabled: enabled}}
	plan, err := s.PreviewOMPExtensions(context.Background(), target, changes, v.Revision)
	if err != nil {
		t.Fatal(err)
	}
	return plan, changes
}

func TestOMPExtensionsProjectInheritsDisabledIDs(t *testing.T) {
	s, home, agent := ompWritableFixture(t)
	global := ompFile(t, agent, "config.yml", "disabledExtensions: [extension-module:gone, foreign:keep]\nprivate: secret\n")
	s.ProjectRoot = filepath.Join(home, "repo")
	local := ompFile(t, s.ProjectRoot, ".omp/config.yml", "# local\nunknown: 'stay byte-for-byte'\n")
	plan, changes := ompPreviewOne(t, s, "omp", "", false)
	if _, err := s.ApplyOMPExtensions(context.Background(), "omp", changes, plan.Revision); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(local)
	for _, value := range []string{"# local\nunknown: 'stay byte-for-byte'\n", "extension-module:gone", "foreign:keep", "extension-module:safe"} {
		if !strings.Contains(string(raw), value) {
			t.Fatalf("project lost %q: %s", value, raw)
		}
	}
	raw, _ = os.ReadFile(global)
	if !strings.Contains(string(raw), "private: secret") || strings.Contains(string(raw), "extension-module:safe") {
		t.Fatal("wrote global settings")
	}
}

func TestOMPExtensionsAccountWriteIsolation(t *testing.T) {
	s, home, agent := ompWritableFixture(t)
	defaultSettings := ompFile(t, agent, "config.yml", "disabledExtensions: [default:only]\n")
	account := filepath.Join(home, "profiles/work/agent")
	ompFile(t, account, "extensions/account.ts", "x")
	file := ompFile(t, account, "config.yml", "disabledExtensions: [work:only]\n")
	s.Accounts = map[string]Account{"work": {Agent: "omp", Dir: account, CLI: "never-run-custom-cli"}}
	plan, changes := ompPreviewOne(t, s, "work", "", false)
	if _, err := s.ApplyOMPExtensions(context.Background(), "work", changes, plan.Revision); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(file)
	if !strings.Contains(string(raw), "work:only") || !strings.Contains(string(raw), "extension-module:account") {
		t.Fatalf("account: %s", raw)
	}
	raw, _ = os.ReadFile(defaultSettings)
	if string(raw) != "disabledExtensions: [default:only]\n" {
		t.Fatal("account touched default")
	}
}

func TestOMPExtensionsRevisionCoversEverySelectionInput(t *testing.T) {
	for _, kind := range []string{"settings", "code", "new-entry", "version", "ownership"} {
		t.Run(kind, func(t *testing.T) {
			s, home, agent := ompWritableFixture(t)
			file := ompFile(t, agent, "config.yml", "disabledExtensions: []\n")
			plan, changes := ompPreviewOne(t, s, "omp", "", false)
			switch kind {
			case "settings":
				ompFile(t, agent, "config.yml", "disabledExtensions: []\nnew: true\n")
			case "code":
				ompFile(t, agent, "extensions/safe.ts", "changed")
			case "new-entry":
				ompFile(t, agent, "extensions/new.ts", "x")
			case "version":
				ompFile(t, home, "npm/omp/package.json", `{"name":"@oh-my-pi/pi-coding-agent","version":"18.6.2","bin":{"omp":"src/cli.ts"}}`)
			case "ownership":
				ompFile(t, s.StateDir, "hooks/state.json", `{"version":2}`)
			}
			expected, _ := os.ReadFile(file)
			if _, err := s.ApplyOMPExtensions(context.Background(), "omp", changes, plan.Revision); !errors.Is(err, ErrOMPExtensionsStale) {
				t.Fatalf("%s not stale: %v", kind, err)
			}
			after, _ := os.ReadFile(file)
			if string(after) != string(expected) {
				t.Fatal("stale apply wrote")
			}
		})
	}
}

func TestOMPExtensionsSymlinksAndUnknownRootsAreReadOnly(t *testing.T) {
	for _, kind := range []string{"settings-link", "source-link", "ancestor-link", "overlay", "profile", "env-file", "legacy"} {
		t.Run(kind, func(t *testing.T) {
			s, home, agent := ompWritableFixture(t)
			switch kind {
			case "settings-link":
				other := ompFile(t, home, "other.yml", "disabledExtensions: []\n")
				if err := os.Symlink(other, filepath.Join(agent, "config.yml")); err != nil {
					t.Fatal(err)
				}
			case "source-link":
				other := ompFile(t, home, "code.ts", "x")
				if err := os.Remove(filepath.Join(agent, "extensions/safe.ts")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(other, filepath.Join(agent, "extensions/safe.ts")); err != nil {
					t.Fatal(err)
				}
			case "ancestor-link":
				alias := filepath.Join(home, "alias")
				if err := os.Symlink(agent, alias); err != nil {
					t.Fatal(err)
				}
				t.Setenv("PI_CODING_AGENT_DIR", alias)
			case "overlay":
				t.Setenv("PI_CONFIG_FILES", "unknown.yml")
			case "profile":
				t.Setenv("OMP_PROFILE", "work")
			case "env-file":
				ompFile(t, agent, ".env", "PI_CONFIG_DIR=other\n")
			case "legacy":
				ompFile(t, agent, "settings.json", `{"disabledExtensions":[]}`)
			}
			v, err := s.OMPExtensions(context.Background(), "omp")
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range v.Rows {
				if r.Selectable {
					t.Fatalf("%s fake switch: %+v", kind, r)
				}
			}
		})
	}
}

func TestOMPExtensionsDisabledEntryEditsPreserveOpaqueBytes(t *testing.T) {
	s, _, agent := ompWritableFixture(t)
	raw := "# before\nunknown: { precise: 0x10, spelling: 'yes' } # byte exact\n\n# disabled header\ndisabledExtensions:\n  # missing entry comment\n  - extension-module:safe # safe comment\n  - gone:opaque # keep this\n\n# unrelated header\nunrelated: |-\n  body\n  # not a YAML comment\n"
	file := ompFile(t, agent, "config.yml", raw)
	plan, changes := ompPreviewOne(t, s, "omp", "", true)
	if _, err := s.ApplyOMPExtensions(context.Background(), "omp", changes, plan.Revision); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(file)
	for _, keep := range []string{"# before\nunknown: { precise: 0x10, spelling: 'yes' } # byte exact\n\n# disabled header\n", "\n# unrelated header\nunrelated: |-\n  body\n  # not a YAML comment\n", "missing entry comment", "safe comment", "gone:opaque # keep this"} {
		if !strings.Contains(string(after), keep) {
			t.Fatalf("lost %q: %s", keep, after)
		}
	}
	if strings.Contains(string(after), "extension-module:safe") {
		t.Fatal("did not enable")
	}
}

func TestOMPExtensionsOwnershipAndBypassCollision(t *testing.T) {
	s, _, agent := ompWritableFixture(t)
	path := filepath.Join(agent, "extensions/safe.ts")
	file := ompFile(t, agent, "config.yml", "extensions: ["+path+"]\n")
	v, _ := s.OMPExtensions(context.Background(), "omp")
	for _, r := range v.Rows {
		if r.Selectable {
			t.Fatal("explicit bypass alias permits fake native disable")
		}
	}
	// The explicit path is removed, but the Hooks ledger now owns its native row.
	ompFile(t, agent, "config.yml", "disabledExtensions: []\n")
	data, _ := os.ReadFile(path)
	ompFile(t, s.StateDir, "hooks/state.json", `{"version":1,"records":{"`+hash([]byte("file\x00"+path))+`":{"Owner":"`+s.ConfigPath+`","Target":"omp","Path":"`+path+`","Hash":"`+hash(data)+`"}}}`)
	v, _ = s.OMPExtensions(context.Background(), "omp")
	if v.Rows[0].Selectable || v.Rows[0].Owner != "hooks" {
		t.Fatalf("ownership: %+v", v.Rows[0])
	}
	raw, _ := os.ReadFile(file)
	if string(raw) != "disabledExtensions: []\n" {
		t.Fatal("inventory wrote")
	}
	ompFile(t, agent, "extensions/safe.ts", "drifted")
	v, _ = s.OMPExtensions(context.Background(), "omp")
	if v.Rows[0].Selectable || !strings.Contains(v.Rows[0].ReadOnlyReason, "drifted") {
		t.Fatalf("drifted owner switch: %+v", v.Rows[0])
	}
}

func TestOMPExtensionsAmbientHookBypassBlocksNativeFile(t *testing.T) {
	s, _, agent := ompWritableFixture(t)
	hook := ompFile(t, agent, "hooks/pre/hook.ts", "x")
	ompFile(t, agent, "settings.json", `{"extensions":["`+hook+`"]}`)
	ompFile(t, agent, "config.yml", "disabledExtensions: []\n")
	v, _ := s.OMPExtensions(context.Background(), "omp")
	for _, r := range v.Rows {
		if r.Path == hook && r.Selectable {
			t.Fatalf("ambient bypass switch: %+v", r)
		}
	}
}

func TestOMPExtensionsConcurrentApplyDoesNotOverwrite(t *testing.T) {
	s, _, _ := ompWritableFixture(t)
	plan, changes := ompPreviewOne(t, s, "omp", "", false)
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := s.ApplyOMPExtensions(context.Background(), "omp", changes, plan.Revision)
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, ErrOMPExtensionsBusy) && !errors.Is(err, ErrOMPExtensionsStale) {
			t.Fatalf("unexpected concurrent error: %v", err)
		}
	}
	if success != 1 {
		t.Fatalf("successful concurrent writes: %d", success)
	}
}

func TestOMPExtensionsEnablingLastIDPreservesItsComments(t *testing.T) {
	s, _, agent := ompWritableFixture(t)
	file := ompFile(t, agent, "config.yml", "disabledExtensions:\n  # keep entry heading\n  - extension-module:safe # keep entry note\nunknown: exact\n")
	plan, changes := ompPreviewOne(t, s, "omp", "", true)
	if _, err := s.ApplyOMPExtensions(context.Background(), "omp", changes, plan.Revision); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(file)
	for _, keep := range []string{"keep entry heading", "keep entry note", "unknown: exact\n"} {
		if !strings.Contains(string(raw), keep) {
			t.Fatalf("lost comment %s: %s", keep, raw)
		}
	}
	if _, _, err := ompYAML(file); err != nil {
		t.Fatalf("invalid YAML after enabling: %v: %s", err, raw)
	}
}

func TestOMPExtensionsAccountWithUnestablishedPluginRootIsReadOnly(t *testing.T) {
	s, home, _ := ompWritableFixture(t)
	account := filepath.Join(home, "accounts/work")
	ompFile(t, account, "extensions/work.ts", "x")
	ompFile(t, home, ".omp/plugins/package.json", `{"dependencies":{"other":"1"}}`)
	s.Accounts = map[string]Account{"work": {Agent: "omp", Dir: account}}
	v, _ := s.OMPExtensions(context.Background(), "work")
	if !v.ReadOnly || v.Rows[0].Selectable || !strings.Contains(v.Rows[0].ReadOnlyReason, "plugin root") {
		t.Fatalf("hidden plugin manager conflict: %+v", v)
	}
}
