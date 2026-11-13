package install

import (
	"errors"
	"strings"
	"testing"
)

func TestRejectSourceCredentials(t *testing.T) {
	t.Run("baseline", func(t *testing.T) {
		cases := []struct{ source, rule string }{
			{"https://user:secret@example.com/repo", "userinfo"},
			{"https://example.com/repo?token=secret", "query"},
			{"https://example.com/repo?", "query"},
			{"https://example.com/repo#secret", "fragment"},
			{"ssh://git:secret@example.com/repo", "password"},
			{"https://user@%zz/repo", "userinfo"},
			{"https://example.com/%zz?token=secret", "invalid URL"},
			{"https://example.com/%zz#secret", "invalid URL"},
			{"https://example.com/repo?token=%zzsecret", "query"},
			{"https://example.com/repo#%zzsecret", "invalid URL"},
			{"file://user:secret@localhost/repo", "userinfo"},
			{"file://secret@localhost/repo", "userinfo"},
			{"file://:secret@localhost/repo", "userinfo"},
			{"file://user:secret@/tmp/repo", "userinfo"},
			{"file://user:secret@%zz/repo", "userinfo"},
			{"file://user:secret@localhost/%zz", "userinfo"},
			{"file://user:%73ecret@localhost/repo", "userinfo"},
			{"file://%73ecret@localhost/repo", "userinfo"},
			{" FILE://user:secret@localhost/repo ", "userinfo"},
			{"FiLe://user:secret@localhost/repo", "userinfo"},
			{"ssh://git@example.com/repo", ""},
			{"git@example.com:org/repo", ""},
			{"file:///tmp/fixture", ""},
		}
		for _, tc := range cases {
			t.Run(tc.source, func(t *testing.T) {
				err := RejectSourceCredentials(tc.source)
				if tc.rule == "" {
					if err != nil {
						t.Fatal(err)
					}
					return
				}
				if !errors.Is(err, ErrSourceCredentials) || !strings.Contains(err.Error(), tc.rule) {
					t.Fatalf("got %v; want credential rule %s", err, tc.rule)
				}
				if strings.Contains(err.Error(), tc.source) || strings.Contains(err.Error(), "secret") {
					t.Fatalf("credential disclosed: %v", err)
				}
			})
		}
	})
	t.Run("TestRejectSourceCredentials_ShorthandClass", func(t *testing.T) {
		for _, source := range []string{
			"https:/example.com/repo?token=SYNTHETIC", "https:example.com/repo?token=SYNTHETIC", `https:\example.com\repo?token=SYNTHETIC`,
			"https//example.com/repo?token=SYNTHETIC", "example.com/repo?token=SYNTHETIC", "example.com/repo#SYNTHETIC", "owner/repo/sub?token=SYNTHETIC",
			"ado:org/project/repo?token=SYNTHETIC", "user:SYNTHETIC@example.com/org/repo", "user:SYNTHETIC@example.com:org/repo", "user@example.com/org/repo",
			" HTTPS://user:SYNTHETIC@example.com/repo", "git://user:SYNTHETIC@example.com/repo",
		} {
			err := RejectSourceCredentials(source)
			if !errors.Is(err, ErrSourceCredentials) || strings.Contains(err.Error(), "SYNTHETIC") || strings.Contains(err.Error(), source) {
				t.Errorf("unsafe admission: %v", err)
			}
		}
	})
	t.Run("TestRejectSourceCredentials_ValidFormsAccepted", func(t *testing.T) {
		for _, source := range []string{"owner/repo", "owner/repo/sub/dir", "owner/repo/packages/@scope/pkg", "github.com/owner/repo", "example.com/org/repo", "example.com:8443/org/repo", "https://example.com:8443/org/repo", "ssh://git@example.com:2222/org/repo", "git@example.com:org/repo.git//sub", "ado:org/project/repo", "https://dev.azure.com/org/project/_git/repo", "file:///tmp/fixture//sub",
			"file://localhost/tmp/fixture", "file:///tmp/fixture@local", "file:///tmp/literal%value", "file:///tmp/fixture?draft#notes",
			"/tmp/fixture@local", "./fixture@local", "../literal%value", `C:\fixture@local`,
		} {
			if err := RejectSourceCredentials(source); err != nil {
				t.Errorf("valid %q: %v", source, err)
			}
		}
	})
}
