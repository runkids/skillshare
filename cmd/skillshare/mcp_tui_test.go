package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"skillshare/internal/mcp"
)

type scriptedMCPPrompts struct {
	choices [][]int
	texts   []string
}

func (*scriptedMCPPrompts) review(string, *mcp.Plan) error { return nil }

func TestMCPReviewRequiresExplicitContinue(t *testing.T) {
	for _, key := range []tea.KeyType{tea.KeyEsc, tea.KeyCtrlC, tea.KeyEnter} {
		m := mcpReviewModel{viewport: viewport.New(76, 20), content: "Remove source server: docs"}
		resized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		if !strings.Contains(resized.View(), "Remove source server: docs") {
			t.Fatal("preview missing from terminal")
		}
		result, cmd := resized.Update(tea.KeyMsg{Type: key})
		if cmd == nil || result.(mcpReviewModel).accepted != (key == tea.KeyEnter) {
			t.Fatalf("unexpected preview decision for %v", key)
		}
	}
}

func (p *scriptedMCPPrompts) choose(checklistConfig) ([]int, error) {
	if len(p.choices) == 0 {
		return nil, errMCPCancelled
	}
	choice := p.choices[0]
	p.choices = p.choices[1:]
	return choice, nil
}
func (p *scriptedMCPPrompts) text(string, string) (string, error) {
	if len(p.texts) == 0 {
		return "", errMCPCancelled
	}
	value := p.texts[0]
	p.texts = p.texts[1:]
	return value, nil
}

func mcpTUIService(t *testing.T) *mcp.Service {
	t.Helper()
	home := t.TempDir()
	s := &mcp.Service{ConfigPath: filepath.Join(home, "config.yaml"), Home: home, StateDir: filepath.Join(home, "state"), Platform: "linux"}
	if err := os.WriteFile(s.ConfigPath, []byte("mcp: {targets: [claude], servers: {}}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestMCPListKeysAndPrivateDetails(t *testing.T) {
	source := &mcp.Source{Targets: []string{"claude"}, Servers: map[string]mcp.Server{"docs": {URL: "https://example.com/mcp?token=hidden-query", Headers: map[string]mcp.Value{"Authorization": {Literal: "hidden-secret"}}}}}
	m := newMCPListModel(source, nil, "global", "")
	for _, key := range []string{"a", "i", "e", "x", "s", "b", "r"} {
		updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
		if cmd == nil || updated.(mcpListModel).action == "" {
			t.Errorf("missing shortcut %s", key)
		}
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	detail := updated.(mcpListModel)
	if !detail.showDetail || strings.Contains(detail.View(), "hidden-") {
		t.Fatalf("unsafe or missing details: %s", detail.View())
	}
	updated, _ = detail.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.(mcpListModel).showDetail {
		t.Fatal("Escape did not return to list")
	}
	m.list.SetFilterState(list.Filtering)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if updated.(mcpListModel).action != "" {
		t.Fatal("typing search triggered add")
	}
}

func TestMCPEditDraftDoesNotMutateOriginal(t *testing.T) {
	s := mcpTUIService(t)
	original := mcp.Server{Command: "echo", Env: map[string]mcp.Value{"TEAM": {Literal: "before"}}}
	prompts := &scriptedMCPPrompts{choices: [][]int{{2}, {2}, {1}, {0}, {6}}, texts: []string{"after"}}
	draft, err := editMCPDraft(s, "docs", original, []string{"claude"}, prompts)
	if err != nil {
		t.Fatal(err)
	}
	if original.Env["TEAM"].Literal != "before" || draft.Env["TEAM"].Literal != "after" {
		t.Fatal("editor mutated input or lost edit")
	}
	_, err = editMCPDraft(s, "docs", original, nil, &scriptedMCPPrompts{choices: [][]int{{0}, {0}}})
	if !errors.Is(err, errMCPCancelled) {
		t.Fatalf("cancel: %v", err)
	}
}

func TestMCPBatchImportCancelAndSaveOnly(t *testing.T) {
	for _, save := range []bool{false, true} {
		s := mcpTUIService(t)
		before, _ := os.ReadFile(s.ConfigPath)
		candidates := []mcp.Candidate{{Name: "one", Server: mcp.Server{Command: "echo"}}, {Name: "two", Server: mcp.Server{URL: "https://example.com/mcp"}}}
		prompts := &scriptedMCPPrompts{choices: [][]int{{0, 1}, {0}}}
		if save {
			prompts.choices = append(prompts.choices, []int{1})
		}
		err := mcpBatchImportWizard(s, candidates, mcpOptions{}, prompts)
		if save {
			if err != nil {
				t.Fatal(err)
			}
			source, err := mcp.LoadSource(s.ConfigPath)
			if err != nil || len(source.Servers) != 2 {
				t.Fatalf("batch: %+v %v", source, err)
			}
		} else {
			if !errors.Is(err, errMCPCancelled) {
				t.Fatal(err)
			}
			after, _ := os.ReadFile(s.ConfigPath)
			if string(after) != string(before) {
				t.Fatal("cancel saved partial batch")
			}
		}
		if _, err := os.Stat(filepath.Join(s.Home, ".claude.json")); !os.IsNotExist(err) {
			t.Fatal("save-only/cancel wrote Agent config")
		}
	}
}

func TestMCPReviewDryRunNeverPromptsOrWrites(t *testing.T) {
	s := mcpTUIService(t)
	before, _ := os.ReadFile(s.ConfigPath)
	server := mcp.Server{Command: "echo", Targets: []string{"claude"}}
	err := reviewMCPMutations(s, []mcp.Mutation{{Name: "one", Server: &server}}, mcpOptions{dryRun: true}, &scriptedMCPPrompts{})
	if err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(s.ConfigPath)
	if string(before) != string(after) {
		t.Fatal("dry-run wrote source")
	}
}

func TestMCPArgumentsPreserveLiterals(t *testing.T) {
	for _, input := range []string{"--workspace\nMy Documents", `["--workspace", "My Documents", ""]`} {
		args, err := parseMCPArgumentInput(input)
		if err != nil || len(args) < 2 || args[1] != "My Documents" {
			t.Fatalf("%q: %v %v", input, args, err)
		}
	}
	if _, err := parseMCPArgumentInput(`[3]`); err == nil {
		t.Fatal("numeric argument accepted")
	}
}
