package config

import (
	"fmt"
	"path/filepath"
)

// ExtraTargetConflict identifies another shared file holding the target.
type ExtraTargetConflict struct {
	Name   string
	Target string
}

func (e *ExtraTargetConflict) Error() string {
	return fmt.Sprintf("%s uses shared file %q; only import can hold more than one shared file", e.Target, e.Name)
}

// ValidateImportMode is shared by CLI configuration and instruction API changes.
func ValidateImportMode(mode, name string, supportsImport bool) error {
	if mode == "import" && !supportsImport {
		return fmt.Errorf("%s does not read @import lines", name)
	}
	return nil
}

// ValidateExtraConnections checks ownership before any target is written. A
// link/copy owns the whole file; imports may share only with other imports.
func ValidateExtraConnections(extras []ExtraConfig, sourceDir func(ExtraConfig) string, targetDir func(string) string, focus ...string) error {
	if err := validateExtraRemotes(extras); err != nil {
		return err
	}
	type owner struct{ name, mode string }
	owners := map[string]owner{}
	for _, extra := range extras {
		if !needsExtraValidation(extra) {
			continue
		}
		if err := ValidateExtraConfig(extra); err != nil {
			return err
		}
		if extra.File == "" {
			continue
		}
		for _, target := range extra.Targets {
			as := target.As
			if as == "" {
				as = extra.File
			}
			dir := targetDir(target.Path)
			if real, err := filepath.EvalSymlinks(dir); err == nil {
				dir = real
			}
			file := filepath.Clean(filepath.Join(dir, as))
			if old, ok := owners[file]; ok && old.name != extra.Name && (!ManagedExtraMode(old.mode) || !ManagedExtraMode(target.Mode)) {
				name := old.name
				if len(focus) > 0 && name == focus[0] {
					name = extra.Name
				}
				return &ExtraTargetConflict{Name: name, Target: file}
			}
			owners[file] = owner{extra.Name, target.Mode}
			// Modes that edit the target in place would write through a link into
			// another extra's source.
			if ManagedExtraMode(target.Mode) {
				if name := ExtraSourceAt(extras, file, sourceDir); name != "" && name != extra.Name {
					return &ExtraTargetConflict{Name: name, Target: file}
				}
			}
		}
	}
	return nil
}

// ExtraSourceAt names the shared source a target resolves to, including chains
// of user-created links and custom extra sources.
func ExtraSourceAt(extras []ExtraConfig, file string, sourceDir func(ExtraConfig) string) string {
	dest, err := filepath.EvalSymlinks(file)
	if err != nil {
		return ""
	}
	for _, extra := range extras {
		if extra.File == "" {
			continue
		}
		src, err := filepath.EvalSymlinks(filepath.Join(sourceDir(extra), extra.File))
		if err == nil && filepath.Clean(src) == filepath.Clean(dest) {
			return extra.Name
		}
	}
	return ""
}

// ValidateExtras checks file ownership and import support in global mode.
func (c *Config) ValidateExtras(focus ...string) error {
	source := func(e ExtraConfig) string {
		return ResolveExtrasSourceDir(e, c.EffectiveExtrasSource(), c.EffectiveSkillsSource())
	}
	if err := ValidateExtraConnections(c.Extras, source, ExpandPath, focus...); err != nil {
		return err
	}
	return validateExtrasImportSupport(c.Extras, c.Targets, false, "", ExpandPath)
}

// ValidateExtras checks file ownership and import support in project mode.
func (c *ProjectConfig) ValidateExtras(root string, focus ...string) error {
	for _, e := range c.Extras {
		if err := ValidateProjectExtraSource(e.Source); err != nil {
			return fmt.Errorf("extra %q: %w", e.Name, err)
		}
	}
	source := func(e ExtraConfig) string {
		return ResolveExtrasSourceDirProject(e, c.EffectiveExtrasSource(root), root)
	}
	target := func(p string) string {
		if filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(root, p)
	}
	if err := ValidateExtraConnections(c.Extras, source, target, focus...); err != nil {
		return err
	}
	targets := map[string]TargetConfig{}
	for _, t := range c.Targets {
		targets[t.Name] = TargetConfig{Instructions: t.Instructions}
	}
	return validateExtrasImportSupport(c.Extras, targets, true, root, target)
}

func validateExtrasImportSupport(extras []ExtraConfig, targets map[string]TargetConfig, project bool, root string, targetDir func(string) string) error {
	// Include known tools even when their skills target is not configured.
	known := DefaultTargets()
	for name, tc := range targets {
		known[name] = tc
	}
	for name, tc := range known {
		it, ok := TargetInstructions(name, tc, project)
		if !ok {
			continue
		}
		file := it.Path
		if project && !filepath.IsAbs(file) {
			file = filepath.Join(root, file)
		}
		for _, extra := range extras {
			if extra.File == "" {
				continue
			}
			for _, target := range extra.Targets {
				as := target.As
				if as == "" {
					as = extra.File
				}
				if filepath.Clean(filepath.Join(targetDir(target.Path), as)) == filepath.Clean(file) {
					if err := ValidateImportMode(target.Mode, name, it.Import); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// needsExtraValidation reports whether an extra sets anything beyond a plain
// folder sync (single-file settings or file filters), which ValidateExtraConfig
// must then check before sync.
func needsExtraValidation(extra ExtraConfig) bool {
	if extra.File != "" {
		return true
	}
	for _, target := range extra.Targets {
		if target.As != "" || ManagedExtraMode(target.Mode) || len(target.Include) > 0 || len(target.Exclude) > 0 {
			return true
		}
	}
	return false
}
