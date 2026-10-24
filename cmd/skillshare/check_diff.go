package main

import (
	"errors"
	"fmt"
	"strings"

	"skillshare/internal/install"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/ui"
)

// runCheckDiff prints what changed in each target skill since it was installed.
func runCheckDiff(sourceDir string, targets []updateTarget, parseOpts install.ParseOptions, follow *sourcewalk.Follow) error {
	store := install.LoadMetadataOrNew(sourceDir)
	names := make([]string, len(targets))
	for i, t := range targets {
		names[i] = t.name
	}
	width := ui.RowWidth(names...)
	indent := strings.Repeat(" ", width+4)

	var changed []string
	failed := 0
	for _, t := range targets {
		if t.isRepo {
			ui.Row(ui.MarkNone, t.name, ui.DimText("tracked repo · run git diff in "+t.path), width)
			continue
		}
		spinner := ui.StartSpinner("Comparing " + t.name + " with its installed version...")
		changes, err := install.LocalChanges(store.GetExact(t.name), parseOpts, t.path, follow)
		spinner.Stop()
		if errors.Is(err, install.ErrNoInstallRecord) {
			ui.Row(ui.MarkNone, t.name, ui.DimText(err.Error()), width)
			continue
		}
		if err != nil {
			ui.Row(ui.MarkFail, t.name, err.Error(), width)
			failed++
			continue
		}
		if len(changes) == 0 {
			ui.Row(ui.MarkOK, t.name, "no local changes since install", width)
			continue
		}
		changed = append(changed, t.name)
		ui.Row(ui.MarkWarn, t.name, plural(len(changes), "file")+" changed since install", width)
		for _, c := range changes {
			fmt.Println(indent + c.Status + " " + c.Path)
			for _, line := range strings.Split(strings.TrimRight(colorizePlainDiff(c.Diff), "\n"), "\n") {
				fmt.Println(indent + "  " + line)
			}
		}
	}

	switch len(changed) {
	case 0:
	case 1:
		ui.Next("skillshare update "+changed[0]+" --force", "discard them and reinstall")
	default:
		ui.Next("skillshare update <name> --force", "discard a skill's changes and reinstall")
	}
	if failed > 0 {
		return fmt.Errorf("could not compare %s", plural(failed, "skill"))
	}
	return nil
}
