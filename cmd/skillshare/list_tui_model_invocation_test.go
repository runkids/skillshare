package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

const flagLine = "disable-model-invocation: true"

// modelInvocationTUI builds a one-item list TUI over a real SKILL.md and returns its path.
func modelInvocationTUI(t *testing.T, entry skillEntry, skillMD string) (listTUIModel, string) {
	t.Helper()
	source := t.TempDir()
	path := filepath.Join(source, entry.RelPath, "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(skillMD), 0644); err != nil {
		t.Fatal(err)
	}
	m := newListTUIModel(nil, []skillItem{{entry: entry}}, 1, "global", source, "", nil, kindAll)
	return m, path
}

func pressKey(t *testing.T, m listTUIModel, key string) listTUIModel {
	t.Helper()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return next.(listTUIModel)
}

func skillMDHasFlag(t *testing.T, path string) bool {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Contains(string(data), flagLine)
}

func TestModelInvocationKey_LocalSkillTogglesDirectly(t *testing.T) {
	m, path := modelInvocationTUI(t, skillEntry{Name: "mine", RelPath: "mine"}, "---\nname: mine\n---\n# Body\n")

	m = pressKey(t, m, "m")

	if m.confirming {
		t.Error("a local skill is yours to edit, so m should not ask")
	}
	if !skillMDHasFlag(t, path) {
		t.Error("SKILL.md should carry the flag after m")
	}
}

func TestModelInvocationKey_TrackedSkillAsksFirst(t *testing.T) {
	m, path := modelInvocationTUI(t, skillEntry{Name: "theirs", RelPath: "_team/theirs", RepoName: "_team"}, "---\nname: theirs\n---\n# Body\n")

	m = pressKey(t, m, "m")

	if !m.confirming {
		t.Fatal("editing a tracked repo blocks its updates, so m should ask")
	}
	if skillMDHasFlag(t, path) {
		t.Fatal("SKILL.md must stay untouched until confirmed")
	}

	m = pressKey(t, m, "y")

	if m.confirming {
		t.Error("confirming should close the prompt and stay in the TUI")
	}
	if !skillMDHasFlag(t, path) {
		t.Error("SKILL.md should carry the flag after confirming")
	}
}

func TestModelInvocationKey_ShowsRefusal(t *testing.T) {
	local := skillEntry{Name: "demo", RelPath: "demo"}
	tracked := skillEntry{Name: "demo", RelPath: "_team/demo", RepoName: "_team"}
	for _, tc := range []struct {
		name, content, reason string
		entry                 skillEntry
	}{
		{"local alias", "---\n{name: demo, fallback: &f false, disable-model-invocation: *f}\n---\n", "cannot safely edit flow flag", local},
		{"local padded opener", "--- \n{name: demo}\n---\n", "exact --- delimiters starting on line 1", local},
		{"tracked leading blank", "\n---\n{name: demo}\n---\n", "exact --- delimiters starting on line 1", tracked},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, path := modelInvocationTUI(t, tc.entry, tc.content)
			next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			m = pressKey(t, next.(listTUIModel), "m")
			if tc.entry.RepoName != "" {
				if !m.confirming || strings.Contains(m.View(), tc.reason) {
					t.Fatal("expected confirmation before attempting the edit")
				}
				m = pressKey(t, m, "y")
			}
			if m.confirming || !strings.Contains(m.View(), tc.reason) {
				t.Fatalf("expected the list to show the refusal:\n%s", m.View())
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != tc.content {
				t.Fatalf("refused edit did not preserve SKILL.md: %q, %v", got, err)
			}
			if m = pressKey(t, m, "?"); strings.Contains(m.View(), tc.reason) {
				t.Error("the next key should clear the old refusal")
			}
		})
	}
}

func TestModelInvocationKey_TurningBackOnNeverAsks(t *testing.T) {
	m, path := modelInvocationTUI(t, skillEntry{Name: "theirs", RelPath: "_team/theirs", RepoName: "_team"}, "---\nname: theirs\n"+flagLine+"\n---\n# Body\n")

	m = pressKey(t, m, "m")

	if m.confirming {
		t.Error("removing the flag restores the file, so there is nothing to warn about")
	}
	if skillMDHasFlag(t, path) {
		t.Error("the flag should be gone")
	}
}

func TestModelInvocationKey_IgnoresAgents(t *testing.T) {
	agents := t.TempDir()
	path := filepath.Join(agents, "helper.md")
	orig := "---\nname: helper\n---\n# Agent\n"
	os.WriteFile(path, []byte(orig), 0644)
	entry := skillEntry{Name: "helper", RelPath: "helper.md", Kind: "agent"}
	m := newListTUIModel(nil, []skillItem{{entry: entry}}, 1, "global", t.TempDir(), agents, nil, kindAll)

	m = pressKey(t, m, "m")

	got, _ := os.ReadFile(path)
	if m.confirming || string(got) != orig {
		t.Error("agents do not read this key, so m should do nothing")
	}
}

func TestRenderDetailHeader_ShowsManualOnlyChip(t *testing.T) {
	e := skillEntry{Name: "mine", RelPath: "mine"}

	off := renderDetailHeader(e, &detailData{ModelInvocationOff: true}, 60)
	on := renderDetailHeader(e, &detailData{}, 60)

	if !strings.Contains(off, "manual only") || strings.Contains(on, "manual only") {
		t.Errorf("chip should appear only while model invocation is off\n off: %q\n on: %q", off, on)
	}
}
