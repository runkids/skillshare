package memory

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestInstructionsProjectPathIsRelativeToProjectRoot(t *testing.T) {
	project := filepath.Join(t.TempDir(), "repo")
	got := Instructions(filepath.Join(project, ".skillshare", "extras", "memory"), project, ModePassive)
	if !strings.Contains(got, "`.skillshare/extras/memory` (relative to the project root)") || strings.Contains(got, project) {
		t.Fatalf("project guidance = %q", got)
	}
	outside := filepath.Join(t.TempDir(), "notes")
	if got := Instructions(outside, project, ModePassive); !strings.Contains(got, "`"+filepath.ToSlash(outside)+"`") || strings.Contains(got, "relative") {
		t.Fatalf("outside guidance = %q", got)
	}
	if got := Instructions(outside, "", ModePassive); !strings.Contains(got, "scope=global") || !strings.Contains(got, "## Shared memory") {
		t.Fatalf("global guidance = %q", got)
	}
}

func TestApplyKeepsOutsideContentAndIsIdempotent(t *testing.T) {
	want := Instructions("/notes", "", ModePassive)
	before := "# Mine\n\nKeep this.\n"
	after, err := Apply(before, want)
	if err != nil || !strings.HasPrefix(after, before) || Inspect(after, want) != StateConfigured {
		t.Fatalf("after = %q, %v", after, err)
	}
	if again, err := Apply(after, want); err != nil || again != after {
		t.Fatalf("second apply changed the file: %q, %v", again, err)
	}
}

func TestApplyReplacesOnlyOutdatedBlock(t *testing.T) {
	content := "top\r\n\r\n" + strings.ReplaceAll(Instructions("/old", "", ModePassive), "\n", "\r\n") + "bottom\r\n"
	want := Instructions("/new", "", ModePassive)
	if Inspect(content, want) != StateOutdated {
		t.Fatalf("state = %s", Inspect(content, want))
	}
	after, err := Apply(content, want)
	if err != nil || !strings.HasPrefix(after, "top\r\n\r\n") || !strings.HasSuffix(after, "-->\r\nbottom\r\n") || !strings.Contains(after, "`/new`") || strings.Contains(after, "/old") {
		t.Fatalf("after = %q, %v", after, err)
	}
	if Inspect(after, want) != StateConfigured {
		t.Fatalf("state after = %s", Inspect(after, want))
	}
}

func TestApplyRefusesUserChangedBlocks(t *testing.T) {
	want := Instructions("/notes", "", ModePassive)
	modified := strings.Replace(want, "Open only", "Always open", 1)
	unclosed := strings.Replace(want, blockEnd+"\n", "", 1)
	twice := want + "\n" + want
	for name, content := range map[string]string{StateModified: modified, StateMalformed: unclosed, StateMalformed + "-twice": twice} {
		state := strings.TrimSuffix(name, "-twice")
		if got := Inspect(content, want); got != state {
			t.Errorf("%s: state = %s", name, got)
		}
		if _, err := Apply(content, want); err == nil {
			t.Errorf("%s: apply succeeded", name)
		}
	}
}

func TestInspectOwnsOnlyMarkedBlocksOfItsScope(t *testing.T) {
	want := Instructions("/notes", "", ModePassive)
	manual := "## Shared memory\n\nShared notes directory: `/notes`\n"
	project := Instructions("notes", "/repo", ModePassive)
	fenced := "```\n" + want + "```\n"
	for _, content := range []string{manual, project, fenced} {
		if got := Inspect(content, want); got != StateUnconfigured {
			t.Errorf("Inspect(%q) = %s", content, got)
		}
	}
}

func TestInspectIgnoresMarkersInCodeExamples(t *testing.T) {
	want := Instructions("/notes", "", ModePassive)
	indented := "    " + strings.ReplaceAll(strings.TrimSuffix(want, "\n"), "\n", "\n    ") + "\n"
	for name, content := range map[string]string{
		"tilde":         "~~~markdown\n" + want + "~~~\n",
		"long backtick": "````\n```\n" + want + "```\n````\n",
		"tilde in ```":  "```\n~~~\n```\n" + "text\n",
		"indented":      "Example:\n\n" + indented,
		"tab":           "\t" + strings.SplitN(want, "\n", 2)[0] + "\n",
	} {
		if got := Inspect(content, want); got != StateUnconfigured {
			t.Errorf("%s: state = %s", name, got)
		}
	}
	after := "~~~\n" + want + "~~~\n\n" + want
	if got := Inspect(after, want); got != StateConfigured {
		t.Errorf("block after a closed fence: state = %s", got)
	}
	if out, err := Apply("~~~\n"+want+"~~~\n", want); err != nil || !strings.HasPrefix(out, "~~~\n"+want+"~~~\n\n<!--") {
		t.Errorf("apply beside example = %q, %v", out, err)
	}
}

func TestInstructionsModes(t *testing.T) {
	passive := Instructions("/notes", "", ModePassive)
	if strings.Contains(passive, "mode=") || !strings.Contains(passive, "only when the user requests it") {
		t.Errorf("passive = %q", passive)
	}
	active := Instructions("/notes", "", ModeActive)
	if !strings.Contains(active, "scope=global mode=active sha256=") || !strings.Contains(active, "propose the note to the user") || strings.Contains(active, "only when the user requests it") {
		t.Errorf("active = %q", active)
	}
}

func TestApplySwitchesModeInPlace(t *testing.T) {
	content := "top\n\n" + Instructions("/notes", "", ModePassive) + "bottom\n"
	active := Instructions("/notes", "", ModeActive)
	if got := Inspect(content, active); got != StateOutdated {
		t.Fatalf("state = %s", got)
	}
	after, err := Apply(content, active)
	if err != nil || !strings.HasPrefix(after, "top\n\n") || !strings.HasSuffix(after, "-->\nbottom\n") || Inspect(after, active) != StateConfigured {
		t.Fatalf("after = %q, %v", after, err)
	}
	if Mode(after, ScopeGlobal) != ModeActive || Mode(content, ScopeGlobal) != ModePassive {
		t.Errorf("modes = %s, %s", Mode(after, ScopeGlobal), Mode(content, ScopeGlobal))
	}
}
