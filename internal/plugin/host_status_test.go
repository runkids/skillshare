package plugin

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"testing"
)

// The dashboard groups Agents by remedy, so a missing CLI has to stay distinguishable
// from a reachable Agent that failed for its own reasons, without reading the message.
func TestHostStatusSeparatesAMissingCLIFromOtherFailures(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want string
		key  string
	}{
		{"cli not on path", agentError{cause: ErrCLIMissing, key: "plugins.error.cliMissing", message: "claude CLI is not installed"}, HostMissing, "plugins.error.cliMissing"},
		{"agent refused", errors.New("claude command failed; open the native client"), HostBlocked, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var h Host
			h.fail(tc.err)
			if h.Status != tc.want {
				t.Fatalf("status = %q, want %q", h.Status, tc.want)
			}
			if h.Error != tc.err.Error() {
				t.Fatalf("message was rewritten: %q", h.Error)
			}
			if h.ErrorKey != tc.key {
				t.Fatalf("key = %q, want %q", h.ErrorKey, tc.key)
			}
		})
	}
}

func TestRunCommandMarksAnAbsentBinary(t *testing.T) {
	_, err := runCommand(context.Background(), t.TempDir(), "skillshare-no-such-binary", "--version")
	if !errors.Is(err, exec.ErrNotFound) && !errors.Is(err, ErrCLIMissing) {
		t.Fatalf("err = %v, want it to match ErrCLIMissing", err)
	}
	if !errors.Is(err, ErrCLIMissing) {
		t.Fatalf("err = %v, want ErrCLIMissing", err)
	}
}

func TestHostReportsReadyWhenTheCLIAnswers(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	s.Run = func(_ context.Context, _, _ string, args ...string) ([]byte, error) {
		if args[0] == "--version" {
			return []byte("2.1.276"), nil
		}
		return []byte(`[]`), nil
	}
	if h := s.host(context.Background(), "claude"); h.Status != HostReady {
		t.Fatalf("status = %q (error %q), want %q", h.Status, h.Error, HostReady)
	}
}

// A fixed sentence without a key silently stays English in the dashboard, so every
// message that is a constant has to carry one.
func TestFixedMessagesCarryATranslationKey(t *testing.T) {
	for _, d := range TargetDefinitions() {
		if d.Reason != "" && d.ReasonKey == "" {
			t.Errorf("%s: reason has no key", d.Target)
		}
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	s.Run = func(_ context.Context, _, _ string, args ...string) ([]byte, error) {
		if args[0] == "--version" {
			return []byte("1.0.0"), nil
		}
		return []byte(`[]`), nil
	}
	for _, target := range Targets {
		h := s.host(context.Background(), target)
		if h.Note != "" && h.NoteKey == "" {
			t.Errorf("%s: note has no key", target)
		}
		if automationProblem(target) != "" && h.ErrorKey == "" {
			t.Errorf("%s: automation problem has no key", target)
		}
	}
}
