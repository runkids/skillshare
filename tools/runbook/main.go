package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	var (
		reportFmt string
		dryRun    bool
		timeout   time.Duration
		noTUI     bool
	)

	flag.StringVar(&reportFmt, "report", "", "output format: json")
	flag.BoolVar(&dryRun, "dry-run", false, "parse and classify only, don't execute")
	flag.DurationVar(&timeout, "timeout", 2*time.Minute, "per-step timeout")
	flag.BoolVar(&noTUI, "no-tui", false, "disable TUI, use plain text output")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: runbook [flags] <file.md|directory>")
		flag.PrintDefaults()
		os.Exit(1)
	}

	target := args[0]
	files, err := resolveFiles(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "no runbook files found")
		os.Exit(1)
	}

	useTUI := !noTUI && !dryRun && reportFmt == ""
	exitCode := 0
	var reports []Report

	for _, file := range files {
		name := filepath.Base(file)
		var report Report
		var runErr error

		if useTUI && len(files) == 1 {
			report, runErr = runWithTUI(file, name, timeout)
		} else {
			report, runErr = runPlain(file, name, dryRun, timeout)
		}

		if runErr != nil {
			fmt.Fprintf(os.Stderr, "error running %s: %v\n", file, runErr)
			exitCode = 1
			continue
		}

		reports = append(reports, report)

		if reportFmt == "json" {
			WriteJSONReport(os.Stdout, report)
		}

		if report.Summary.Failed > 0 {
			exitCode = 1
		}
	}

	// Print summary for non-JSON, non-TUI modes.
	if reportFmt != "json" {
		if len(reports) > 1 {
			WritePlainSummary(os.Stdout, reports)
		} else if len(reports) == 1 && !useTUI {
			WriteSingleReport(os.Stdout, reports[0])
		}
	}

	os.Exit(exitCode)
}

// runPlain runs a runbook without TUI.
func runPlain(path, name string, dryRun bool, timeout time.Duration) (Report, error) {
	f, err := os.Open(path)
	if err != nil {
		return Report{}, err
	}
	defer f.Close()

	return RunRunbook(f, name, RunOptions{
		DryRun:  dryRun,
		Timeout: timeout,
	})
}

// runWithTUI runs a runbook with bubbletea TUI.
func runWithTUI(path, name string, timeout time.Duration) (Report, error) {
	f, err := os.Open(path)
	if err != nil {
		return Report{}, err
	}
	defer f.Close()

	rb, err := ParseRunbook(f)
	if err != nil {
		return Report{}, err
	}

	steps := ClassifyAll(rb.Steps)

	// Separate auto and manual steps.
	var autoSteps []Step
	var skippedResults []StepResult
	for _, s := range steps {
		if s.Executor == ExecutorAuto {
			autoSteps = append(autoSteps, s)
		} else {
			skippedResults = append(skippedResults, StepResult{
				Step:   s,
				Status: StatusSkipped,
				Error:  "manual step",
			})
		}
	}

	if timeout == 0 {
		timeout = 2 * time.Minute
	}

	start := time.Now()

	execFn := func(s Step) StepResult {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		result := Execute(ctx, s)
		checkAssertions(&result, s)
		return result
	}

	model := newTUIModel(name, autoSteps, execFn)
	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		return Report{}, err
	}

	m := finalModel.(tuiModel)
	allResults := append(m.results, skippedResults...)

	report := Report{
		Version:    "1",
		Runbook:    name,
		DurationMs: msDuration(time.Since(start)),
		Steps:      allResults,
		Summary:    computeSummary(allResults),
	}

	return report, nil
}

// resolveFiles finds runbook files from a path (file or directory).
func resolveFiles(target string) ([]string, error) {
	info, err := os.Stat(target)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{target}, nil
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), "_runbook.md") || strings.HasSuffix(e.Name(), "-runbook.md") {
			files = append(files, filepath.Join(target, e.Name()))
		}
	}
	return files, nil
}
