package install

import (
	"errors"
	"strings"
	"testing"
)

func TestRejectSourceCredentials(t *testing.T) {
	cases := []struct{ source, rule string }{
		{"https://user:secret@example.com/repo", "userinfo"},
		{"https://example.com/repo?token=secret", "query"},
		{"https://example.com/repo?", "query"},
		{"https://example.com/repo#secret", "fragment"},
		{"ssh://git:secret@example.com/repo", "password"},
		{"https://user@%zz/repo", "userinfo"},
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
}
