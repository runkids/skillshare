package main

import (
	"fmt"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/ui"
)

type checkExtraResult struct {
	Name    string `json:"name"`
	Remote  string `json:"remote"`
	Branch  string `json:"branch,omitempty"`
	Commit  string `json:"commit,omitempty"`
	State   string `json:"state"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

func inspectRemoteExtras(entries []config.ExtraConfig, root string) []checkExtraResult {
	var results []checkExtraResult
	for _, entry := range entries {
		if entry.Remote == "" {
			continue
		}
		state := install.InspectRemoteExtra(root, entry.Name)
		out := checkExtraResult{Name: entry.Name, Remote: entry.Remote, Branch: entry.Branch, Commit: state.Commit, State: state.State(), Message: "remote revision not checked"}
		if state.Err != nil {
			out.Error = state.Err.Error()
		}
		results = append(results, out)
	}
	return append(results, undeclaredExtraPublications(entries, root)...)
}

func undeclaredExtraPublications(entries []config.ExtraConfig, root string) []checkExtraResult {
	names, err := install.InterruptedExtraNames(root)
	if err != nil {
		return []checkExtraResult{{Name: ".skillshare-staging", State: "error", Error: err.Error(), Message: "publication artifacts could not be inspected"}}
	}
	declared := map[string]bool{}
	for _, entry := range entries {
		if entry.Remote != "" {
			declared[entry.Name] = true
		}
	}
	var out []checkExtraResult
	for _, name := range names {
		if !declared[name] {
			out = append(out, checkExtraResult{Name: name, State: "interrupted", Message: "publication interrupted before remote declaration; rerun the original install with the same --name to repair; remote revision not checked"})
		}
	}
	return out
}

func remoteExtraSummary(remote, branch, commit, state string) string {
	if len(commit) > 7 {
		commit = commit[:7]
	}
	parts := []string{remote}
	if branch != "" {
		parts = append(parts, branch)
	}
	if commit != "" {
		parts = append(parts, commit)
	}
	return strings.Join(append(parts, state), " · ")
}

func cmdCheckExtras(mode runMode, cwd string, args []string) error {
	opts, help, err := parseCheckArgs(args)
	if help {
		printCheckHelp()
		return nil
	}
	if err != nil {
		return err
	}
	if opts.diff || len(opts.groups) != 0 {
		return fmt.Errorf("check extras supports names, --json and --exit-code; remote revision not checked")
	}
	var entries []config.ExtraConfig
	var root string
	if mode == modeProject {
		cfg, err := config.LoadProjectReadOnly(cwd)
		if err != nil {
			return err
		}
		entries, root = cfg.Extras, cfg.EffectiveExtrasSource(cwd)
	} else {
		cfg, err := config.LoadReadOnly()
		if err != nil {
			return err
		}
		entries, root = cfg.Extras, cfg.EffectiveExtrasSource()
	}
	requested := make(map[string]bool, len(opts.names))
	for _, name := range opts.names {
		requested[name] = true
	}
	results := make([]checkExtraResult, 0)
	tally := checkTally{}
	for _, out := range inspectRemoteExtras(entries, root) {
		if len(opts.names) > 0 && !requested[out.Name] {
			continue
		}
		delete(requested, out.Name)
		if out.Error != "" {
			tally.failed++
		} else if out.State != "intact" {
			tally.outdated++
		}
		results = append(results, out)
	}
	for _, name := range opts.names {
		if requested[name] {
			results = append(results, checkExtraResult{Name: name, State: "error", Message: "remote revision not checked", Error: "remote extra not found"})
			tally.failed++
		}
	}
	err = tally.err(opts.exitCode)
	if opts.json {
		return writeJSONResult(results, err)
	}
	for _, result := range results {
		mark := ui.MarkOK
		if result.State != "intact" {
			mark = ui.MarkWarn
		}
		ui.Row(mark, result.Name, remoteExtraSummary(result.Remote, result.Branch, result.Commit, result.State), ui.RowWidth(result.Name))
		if result.Error != "" {
			ui.Warning("%s", result.Error)
		} else if result.Remote == "" && result.State == "interrupted" {
			ui.Note(result.Message)
		}
	}
	ui.Note("Local integrity only; remote revision not checked")
	return err
}
