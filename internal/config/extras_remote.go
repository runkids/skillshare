package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
	"skillshare/internal/install"
)

var ErrRawExtrasUnreadable = errors.New("extras section is not readable")

// RawExtras decodes only extras. Only a numeric syntax line can leave the
// parser: yaml.v3 errors can include scalars, duplicate keys and anchor names.
func RawExtras(data []byte) ([]ExtraConfig, error) {
	var document struct {
		Extras []ExtraConfig `yaml:"extras"`
	}
	if err := yaml.Unmarshal(data, &document); err != nil {
		var typeErr *yaml.TypeError
		if errors.As(err, &typeErr) {
			return nil, ErrRawExtrasUnreadable
		}
		// yaml.v3 exposes no structured syntax error. Keep only its bounded
		// numeric location prefix; its remaining text can contain input values.
		if rest, ok := strings.CutPrefix(err.Error(), "yaml: line "); ok {
			if digits, _, ok := strings.Cut(rest, ":"); ok && len(digits) <= 10 {
				if line, err := strconv.ParseUint(digits, 10, 32); err == nil && line > 0 {
					return nil, fmt.Errorf("invalid YAML syntax at line %d", line)
				}
			}
		}
		return nil, errors.New("invalid YAML syntax")
	}
	return document.Extras, nil
}

// RawExtrasRemoteError withholds parseable credentials and unreadable extras.
// Syntax-broken raw documents stay readable in the recovery editor by design.
func RawExtrasRemoteError(data []byte) error {
	extras, err := RawExtras(data)
	if err != nil {
		if errors.Is(err, ErrRawExtrasUnreadable) {
			return err
		}
		return nil
	}
	for _, extra := range extras {
		if err := install.RejectSourceCredentials(extra.Remote); err != nil {
			return fmt.Errorf("extra %q: %w", extra.Name, err)
		}
	}
	return nil
}

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
	r := strings.TrimSpace(e.Remote)
	if r == "" || !strings.HasPrefix(strings.ToLower(r), "file://") && (strings.HasPrefix(r, "/") || strings.HasPrefix(r, "~") || strings.HasPrefix(r, ".") || strings.HasPrefix(r, `\`) || len(r) >= 2 && unicode.IsLetter(rune(r[0])) && r[1] == ':') {
		return fmt.Errorf("extra %q: remote must not be a filesystem path", e.Name)
	}
	source, err := install.ParseSource(r)
	if err != nil || source == nil || !source.IsGit() {
		return fmt.Errorf("extra %q: unsupported remote source form", e.Name)
	}
	return nil
}

// ValidateRemoteExtraEdits prevents bypassing the remote lifecycle through raw config.
func ValidateRemoteExtraEdits(before, after []ExtraConfig) error {
	for _, old := range before {
		if old.Remote == "" || ValidateExtraRemote(old) != nil {
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

// ExtraRestoreCommand names the explicit slice-1 restore, with shell arguments
// quoted when needed. A bare install does not restore remote extras yet.
func ExtraRestoreCommand(extra ExtraConfig, project bool) string {
	word := func(s string) string {
		if strings.IndexFunc(s, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune("_./:@%+=,-", r)
		}) < 0 && s != "" {
			return s
		}
		return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
	}
	command := "skillshare install " + word(extra.Remote) + " --kind extra --name " + word(extra.Name)
	if extra.Branch != "" {
		command += " --branch " + word(extra.Branch)
	}
	if project {
		return command + " -p"
	}
	return command + " -g"
}
