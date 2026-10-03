package main

import "testing"

// keepPathsUnfolded stops shortenPath from folding the /tmp fixtures to ~,
// which it would where HOME is /tmp (as in the docker sandbox).
func keepPathsUnfolded(t *testing.T) {
	t.Helper()
	old := cachedHome
	cachedHome = "/nonexistent-home"
	t.Cleanup(func() { cachedHome = old })
}

func TestConfirmTargetLabel_SingleFileShowsFile(t *testing.T) {
	keepPathsUnfolded(t)
	entry := extrasListEntry{
		Name: "pi-prompt",
		File: "system.md",
		Targets: []extrasTargetInfo{
			{Path: "/tmp/pi/agent", As: "APPEND_SYSTEM.md"},
			{Path: "/tmp/notes"},
		},
	}
	tests := []struct {
		target string
		want   string
	}{
		{"", "all targets"},
		{"/tmp/pi/agent", "/tmp/pi/agent/APPEND_SYSTEM.md"},
		{"/tmp/notes", "/tmp/notes/system.md"},
	}
	for _, tt := range tests {
		m := extrasListTUIModel{confirmTarget: tt.target}
		if got := m.confirmTargetLabel(entry); got != tt.want {
			t.Errorf("confirmTarget %q: got %q, want %q", tt.target, got, tt.want)
		}
	}
}

func TestConfirmTargetLabel_FolderShowsDirectory(t *testing.T) {
	keepPathsUnfolded(t)
	entry := extrasListEntry{Name: "rules", Targets: []extrasTargetInfo{{Path: "/tmp/rules"}}}
	m := extrasListTUIModel{confirmTarget: "/tmp/rules"}
	if got := m.confirmTargetLabel(entry); got != "/tmp/rules" {
		t.Errorf("got %q, want /tmp/rules", got)
	}
}
