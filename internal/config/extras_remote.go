package config

import (
	"fmt"
	"strings"
	"unicode"

	"skillshare/internal/install"
)

// ValidateExtraRemote checks remote-extra origin fields without exposing credentials.
func ValidateExtraRemote(e ExtraConfig) error {
	if e.Remote == "" {
		if e.Branch != "" {
			return fmt.Errorf("extra %q: branch requires remote", e.Name)
		}
		return nil
	}
	if err := install.RejectSourceCredentials(e.Remote); err != nil {
		return fmt.Errorf("extra %q: %w", e.Name, err)
	}
	if e.Source != "" {
		return fmt.Errorf("extra %q: remote and source are mutually exclusive", e.Name)
	}
	r := e.Remote
	if !strings.HasPrefix(strings.ToLower(r), "file://") && (strings.HasPrefix(r, "/") || strings.HasPrefix(r, "~") || strings.HasPrefix(r, ".") || strings.HasPrefix(r, `\`) || len(r) >= 2 && unicode.IsLetter(rune(r[0])) && r[1] == ':') {
		return fmt.Errorf("extra %q: remote must not be a filesystem path", e.Name)
	}
	return nil
}

// ValidateRemoteExtraEdits prevents bypassing the remote lifecycle through raw config.
func ValidateRemoteExtraEdits(before, after []ExtraConfig) error {
	for _, old := range before {
		if old.Remote == "" {
			continue
		}
		found := false
		for _, next := range after {
			if next.Name == old.Name && next.Remote == old.Remote && next.Branch == old.Branch && next.Source == "" {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%q comes from %s; to rename or move it run: skillshare extras remove %s && skillshare install %s --kind extra --name <new>", old.Name, old.Remote, old.Name, old.Remote)
		}
	}
	return nil
}

func validateExtraRemotes(extras []ExtraConfig) error {
	for _, extra := range extras {
		if err := ValidateExtraRemote(extra); err != nil {
			return err
		}
	}
	return nil
}
