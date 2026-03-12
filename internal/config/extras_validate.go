package config

import (
	"fmt"
	"regexp"
	"slices"
)

var extraNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

var reservedExtraNames = map[string]bool{
	"skills": true,
	"extras": true,
}

// ValidateExtraName checks that a name is safe for use as an extras directory.
// It rejects empty names, reserved words, and names that don't match the
// allowed character set (alphanumeric, hyphens, underscores; must start with
// alphanumeric).
func ValidateExtraName(name string) error {
	if name == "" {
		return fmt.Errorf("extra name cannot be empty")
	}
	if reservedExtraNames[name] {
		return fmt.Errorf("extra name %q is reserved", name)
	}
	if !extraNameRegex.MatchString(name) {
		return fmt.Errorf("extra name %q is invalid: must start with a letter or digit and contain only letters, digits, hyphens, or underscores", name)
	}
	return nil
}

// ExtraSyncModes is the authoritative list of valid extras sync modes.
var ExtraSyncModes = []string{"merge", "copy", "symlink"}

// ValidateExtraMode checks that mode is a valid sync mode.
// Empty string is allowed (defaults to "merge" at runtime).
func ValidateExtraMode(mode string) error {
	if mode == "" {
		return nil
	}
	if !slices.Contains(ExtraSyncModes, mode) {
		return fmt.Errorf("invalid mode %q: must be merge, copy, or symlink", mode)
	}
	return nil
}

// ValidateExtraNameUnique checks that the name doesn't duplicate an existing extra.
func ValidateExtraNameUnique(name string, existing []ExtraConfig) error {
	for _, e := range existing {
		if e.Name == name {
			return fmt.Errorf("extra name %q already exists", name)
		}
	}
	return nil
}
