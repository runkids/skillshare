package config

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"skillshare/internal/install"
)

// ValidateExtraName checks that a name is safe for use as an extras directory.
// It rejects empty names, reserved words, and names that don't match the
// allowed character set (alphanumeric, hyphens, underscores; must start with
// alphanumeric).
func ValidateExtraName(name string) error {
	return install.ValidateExtraName(name)
}

// ExtraSyncModes is the authoritative list of valid extras sync modes. It is
// its own list (not ValidSyncModes) because the managed modes are valid only
// for single-file extras and must never be accepted for skills.
var ExtraSyncModes = []string{"merge", "symlink", "copy", "import", "prepend", "append"}

// ManagedExtraMode reports a mode that keeps a managed part of the target
// file (an @path line or a content block) instead of owning the whole file.
// Several managed targets can share one file.
func ManagedExtraMode(mode string) bool {
	return mode == "import" || mode == "prepend" || mode == "append"
}

// EffectiveMode returns the target's mode after its extension: a transform
// extension only makes sense with copy mode.
func (t ExtraTargetConfig) EffectiveMode() string {
	if t.Extension != "" {
		return "copy"
	}
	return t.Mode
}

// Normalize applies EffectiveMode and drops include and exclude from a
// symlink target: one link to the whole folder cannot leave files out.
func (t *ExtraTargetConfig) Normalize() {
	t.Mode = t.EffectiveMode()
	if t.Mode == "symlink" {
		t.Include, t.Exclude = nil, nil
	}
}

// ValidateExtraMode checks that mode is a valid sync mode.
// Empty string is allowed (defaults to "merge" at runtime).
func ValidateExtraMode(mode string) error {
	if mode == "" {
		return nil
	}
	if !slices.Contains(ExtraSyncModes, mode) {
		return fmt.Errorf("invalid mode %q: must be merge, copy, symlink, import, prepend, or append", mode)
	}
	return nil
}

// ValidateExtraFlatten checks that flatten is not used with symlink mode.
func ValidateExtraFlatten(flatten bool, mode string) error {
	if flatten && mode == "symlink" {
		return fmt.Errorf("flatten cannot be used with symlink mode (symlink links the entire directory)")
	}
	return nil
}

// ValidateExtraNameUnique checks that the name doesn't duplicate an existing
// extra. Names differing only in case count as duplicates: the name is a
// directory name, and macOS and Windows file systems ignore case.
func ValidateExtraNameUnique(name string, existing []ExtraConfig) error {
	for _, e := range existing {
		if e.Name == name {
			return fmt.Errorf("extra name %q already exists", name)
		}
		if strings.EqualFold(e.Name, name) {
			return fmt.Errorf("extra name %q is too close to %q: names cannot differ only in case", name, e.Name)
		}
	}
	return nil
}

// Extras with these names get special handling: sync skips agent target
// folders for the extra named agents, and the memory notes live in the extra
// named memory.
const (
	AgentsExtraName = "agents"
	MemoryExtraName = "memory"
)

// ValidateExtraRename rejects renaming an extra to or from a name with
// special handling, ignoring case.
func ValidateExtraRename(oldName, newName string) error {
	for _, special := range []string{AgentsExtraName, MemoryExtraName} {
		if strings.EqualFold(oldName, special) || strings.EqualFold(newName, special) {
			return fmt.Errorf("an extra cannot be renamed to or from %s", special)
		}
	}
	return nil
}

// ValidateExtraConfig checks an extra's single-file settings and target modes.
// file and as must be plain filenames; as, import mode, and the absence of
// flatten are tied to file; an extension cannot transform a single file.
func ValidateExtraConfig(extra ExtraConfig) error {
	if extra.File != "" {
		if err := validateExtraFilename("file", extra.File); err != nil {
			return fmt.Errorf("extra %q: %w", extra.Name, err)
		}
	}
	for _, t := range extra.Targets {
		if err := validateExtraTarget(extra.File, t); err != nil {
			return fmt.Errorf("extra %q target %s: %w", extra.Name, t.Path, err)
		}
	}
	return nil
}

func validateExtraTarget(file string, t ExtraTargetConfig) error {
	if err := ValidateExtraMode(t.Mode); err != nil {
		return err
	}
	if err := ValidateExtraFlatten(t.Flatten, t.Mode); err != nil {
		return err
	}
	if len(t.Include) > 0 || len(t.Exclude) > 0 {
		if file != "" {
			return fmt.Errorf("include and exclude cannot be used with a single-file extra")
		}
		if t.Mode == "symlink" {
			return fmt.Errorf("include and exclude cannot be used with symlink mode (symlink links the entire directory)")
		}
		if err := validateExtraPatterns("include", t.Include); err != nil {
			return err
		}
		if err := validateExtraPatterns("exclude", t.Exclude); err != nil {
			return err
		}
	}
	if file == "" {
		if t.As != "" {
			return fmt.Errorf("as requires file")
		}
		if ManagedExtraMode(t.Mode) {
			return fmt.Errorf("%s mode requires file", t.Mode)
		}
		return nil
	}
	if t.As != "" {
		if err := validateExtraFilename("as", t.As); err != nil {
			return err
		}
	}
	if t.Flatten {
		return fmt.Errorf("flatten cannot be used with a single-file extra")
	}
	if t.Extension != "" {
		return fmt.Errorf("extension cannot be used with a single-file extra")
	}
	return nil
}

func validateExtraFilename(field, name string) error {
	if name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("%s %q must be a plain filename", field, name)
	}
	return nil
}

// validateExtraPatterns rejects .gitignore-style patterns whose path segments
// are not valid globs, so a typo fails at config time instead of matching
// nothing during sync.
func validateExtraPatterns(field string, patterns []string) error {
	for _, p := range patterns {
		for _, seg := range strings.Split(strings.TrimPrefix(strings.TrimSpace(p), "!"), "/") {
			if _, err := path.Match(seg, ""); err != nil {
				return fmt.Errorf("invalid %s pattern %q: %w", field, p, err)
			}
		}
	}
	return nil
}
