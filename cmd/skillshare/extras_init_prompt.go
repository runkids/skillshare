package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/ui"
)

var syncModes = config.ValidSyncModes // folder extras; import needs a single file

// singleFileSyncModes are offered for a single-file extra. symlink is left out:
// for one file it does the same as merge.
var singleFileSyncModes = []string{"merge", "copy", "import"}

type extrasInitTarget struct {
	path    string
	mode    string
	flatten bool
	as      string // single file: target filename ("" = the source file name)
}

// extrasInitAnswers is what the extras init questions collect.
type extrasInitAnswers struct {
	name, file, source string
	singleFile         bool
	targets            []extrasInitTarget
}

func (a *extrasInitAnswers) modes() []string {
	if a.singleFile {
		return singleFileSyncModes
	}
	return syncModes
}

func (a *extrasInitAnswers) targetLabel(t extrasInitTarget) string {
	path, modeLabel := t.path, t.mode
	if a.singleFile {
		path = singleFileTargetPath(config.ExtraConfig{File: a.file}, config.ExtraTargetConfig{Path: t.path, As: t.as})
	}
	if t.flatten {
		modeLabel += ", flatten"
	}
	return fmt.Sprintf("%s (%s)", shortenPath(path), modeLabel)
}

// askExtrasInit asks for a new extra one question at a time. It returns
// nil when the user declines the final confirmation, and ui.ErrCancelled
// on esc.
func askExtrasInit() (*extrasInitAnswers, error) {
	a := &extrasInitAnswers{}
	name, err := ui.InputValid("Extra name", "rules", "", func(v string) error {
		return config.ValidateExtraName(strings.TrimSpace(v))
	})
	if err != nil {
		return nil, err
	}
	a.name = strings.TrimSpace(name)
	ui.Answered("name", a.name)

	kinds := []checklistItemData{
		{label: "Folder", desc: "every file in a folder"},
		{label: "Single file", desc: "one file, renamed per target if needed"},
	}
	kind, err := ui.Select("What do you want to sync?", checklistOptions(kinds), "0")
	if err != nil {
		return nil, err
	}
	a.singleFile = kind == "1"
	if a.singleFile {
		file, err := ui.InputValid("Source file name", "system.md", "", func(v string) error {
			if strings.TrimSpace(v) == "" {
				return errors.New("enter the file name")
			}
			return config.ValidateExtraConfig(config.ExtraConfig{Name: a.name, File: strings.TrimSpace(v)})
		})
		if err != nil {
			return nil, err
		}
		a.file = strings.TrimSpace(file)
		ui.Answered("file", a.file)
	} else {
		ui.Answered("kind", "folder")
	}

	source, err := ui.Input("Source folder (optional)", "Enter for the default", "")
	if err != nil {
		return nil, err
	}
	a.source = strings.TrimSpace(source)
	if a.source != "" {
		ui.Answered("source", a.source)
	}

	for {
		t, err := askExtrasInitTarget(a)
		if err != nil {
			return nil, err
		}
		a.targets = append(a.targets, t)
		ui.Answered("target", a.targetLabel(t))
		more, err := ui.Confirm("Add another target?", false)
		if err != nil {
			return nil, err
		}
		if !more {
			break
		}
	}

	ok, err := ui.ConfirmAction("Create this extra?", true)
	if err != nil || !ok {
		return nil, err
	}
	return a, nil
}

// askExtrasInitTarget asks where one target is and how it is synced.
func askExtrasInitTarget(a *extrasInitAnswers) (extrasInitTarget, error) {
	var t extrasInitTarget
	path, err := ui.InputValid(fmt.Sprintf("Target %d folder", len(a.targets)+1), targetPlaceholder(len(a.targets)), "", func(v string) error {
		if strings.TrimSpace(v) == "" {
			return errors.New("enter the folder the files go to")
		}
		return nil
	})
	if err != nil {
		return t, err
	}
	t.path = strings.TrimSpace(path)

	if a.singleFile {
		as, err := ui.InputValid("File name in this target", "Enter keeps "+a.file, "", func(v string) error {
			tc := config.ExtraTargetConfig{Path: t.path, As: strings.TrimSpace(v)}
			return config.ValidateExtraConfig(config.ExtraConfig{Name: a.name, File: a.file, Targets: []config.ExtraTargetConfig{tc}})
		})
		if err != nil {
			return t, err
		}
		if t.as = strings.TrimSpace(as); t.as == a.file {
			t.as = ""
		}
	}

	modes := a.modes()
	items := make([]checklistItemData, len(modes))
	for i, mode := range modes {
		items[i] = checklistItemData{label: mode, desc: extrasModeDesc(mode, a.singleFile)}
	}
	choice, err := ui.Select("Sync mode", checklistOptions(items), "0")
	if err != nil {
		return t, err
	}
	i, err := strconv.Atoi(choice)
	if err != nil {
		return t, err
	}
	t.mode = modes[i]

	// One file, or a folder linked whole, has nothing to flatten.
	if !a.singleFile && t.mode != "symlink" {
		if t.flatten, err = ui.Confirm("Flatten files into the target root?", false); err != nil {
			return t, err
		}
	}
	return t, nil
}

// extrasModeDesc says what a sync mode does for this kind of extra.
func extrasModeDesc(mode string, singleFile bool) string {
	switch {
	case singleFile && mode == "merge":
		return "file symlink, default"
	case singleFile && mode == "copy":
		return "file copy"
	case mode == "import":
		return "@path line in the target file"
	case mode == "merge":
		return "per-file symlinks, default"
	case mode == "copy":
		return "file copies"
	case mode == "symlink":
		return "directory symlink"
	}
	return ""
}

// targetPlaceholder returns a contextual placeholder for the target input.
func targetPlaceholder(n int) string {
	placeholders := []string{
		"~/.claude/rules",
		"~/.cursor/rules",
		"~/.codex/rules",
	}
	if n < len(placeholders) {
		return placeholders[n]
	}
	return "~/.<tool>/rules"
}

// cmdExtrasInitPrompt asks for a new extra when no arguments are provided.
func cmdExtrasInitPrompt(mode runMode, cwd string) error {
	result, err := askExtrasInit()
	if errors.Is(err, ui.ErrCancelled) || (err == nil && result == nil) {
		ui.Cancelled("created")
		return nil
	}
	if err != nil {
		return err
	}

	start := time.Now()
	opts := extrasInitOptions{name: result.name, source: result.source, file: result.file}
	if result.singleFile {
		// Each target keeps its own file name and mode; merge is the default.
		for _, t := range result.targets {
			if t.mode == "merge" {
				t.mode = ""
			}
			opts.targets = append(opts.targets, t)
		}
	} else {
		// Collect targets and the first non-empty mode (mode applies globally)
		syncMode := ""
		flatten := false
		for _, t := range result.targets {
			if syncMode == "" && t.mode != "" && t.mode != "merge" {
				syncMode = t.mode
			}
			if t.flatten {
				flatten = true
			}
		}
		for _, t := range result.targets {
			opts.targets = append(opts.targets, extrasInitTarget{path: t.path, mode: syncMode, flatten: flatten})
		}
	}
	if err := validateExtrasInit(opts); err != nil {
		return err
	}

	if mode == modeProject {
		if err := config.ValidateProjectExtraSource(opts.source); err != nil {
			return err
		}
		return extrasInitProject(cwd, opts, start)
	}
	return extrasInitGlobal(opts, start)
}
