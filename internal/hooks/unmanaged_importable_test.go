package hooks

import (
	"path/filepath"
	"strings"
	"testing"
)

// Each unmanaged source says whether Import takes its hooks over, so the dashboard
// offers Review only for hooks it can import.
func TestUnmanagedImportableMatchesImport(t *testing.T) {
	stop := `{"Stop":[{"hooks":[{"type":"command","command":"echo x"}]}]}`
	for _, tc := range []struct {
		name, target, file, content string
		project, want               bool
	}{
		{"claude settings", "claude", ".claude/settings.json", `{"hooks":` + stop + `}`, false, true},
		{"claude project local settings", "claude", ".claude/settings.local.json", `{"hooks":` + stop + `}`, true, false},
		{"codex hooks.json", "codex", ".codex/hooks.json", `{"hooks":` + stop + `}`, false, true},
		{"codex inline toml", "codex", ".codex/config.toml", "[[hooks.Stop]]\n[[hooks.Stop.hooks]]\ntype = \"command\"\ncommand = \"echo x\"\n", false, false},
		{"droid inline settings", "droid", ".factory/settings.json", `{"hooks":` + stop + `}`, false, true},
		{"droid legacy file", "droid", ".factory/hooks/hooks.json", stop, false, false},
		{"omp extension", "omp", ".omp/agent/extensions/mine.ts", ompCode, false, true},
		{"omp pre factory", "omp", ".omp/agent/hooks/pre/guard.ts", ompCode, false, false},
		{"git config", "git", ".gitconfig", "[hook \"guard\"]\n\tcommand = echo x\n\tevent = pre-commit\n", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := gitEnv(t)
			s, base := e.service, e.home
			if tc.project {
				base = filepath.Join(t.TempDir(), "app")
				s = e.project(base)
			}
			path := filepath.Join(base, tc.file)
			write(t, path, tc.content)
			inv, err := s.List()
			must(t, err)
			var listed []Unmanaged
			for _, u := range inv.Unmanaged {
				if u.Target == tc.target && strings.HasPrefix(path, u.Path) {
					listed = append(listed, u)
				}
			}
			if len(listed) != 1 {
				t.Fatalf("%s is not listed once: %+v", path, inv.Unmanaged)
			}
			candidates, err := s.Import(ImportRequest{From: tc.target})
			imported := err == nil && len(candidates) > 0
			if listed[0].Importable != tc.want || imported != tc.want {
				t.Fatalf("importable=%v, Import found hooks=%v, want %v", listed[0].Importable, imported, tc.want)
			}
		})
	}
}

// One Agent can hold both kinds of source; each entry keeps its own flag.
func TestUnmanagedImportableIsPerSource(t *testing.T) {
	e := newEnv(t)
	codex := filepath.Join(e.home, ".codex")
	write(t, filepath.Join(codex, "hooks.json"), `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"echo x"}]}]}}`)
	write(t, filepath.Join(codex, "config.toml"), "[[hooks.SessionStart]]\n")
	inv, err := e.service.List()
	must(t, err)
	got := map[string]bool{}
	for _, u := range inv.Unmanaged {
		if u.Target == "codex" {
			got[filepath.Base(u.Path)] = u.Importable
		}
	}
	if len(got) != 2 || !got["hooks.json"] || got["config.toml"] {
		t.Fatalf("codex sources: %v", got)
	}
}
