package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"skillshare/internal/theme"
	"skillshare/internal/ui"
)

// checklistItemData holds the input data for a single checklist item.
type checklistItemData struct {
	label       string
	desc        string
	preSelected bool
}

// checklistConfig configures the checklist prompt.
type checklistConfig struct {
	title        string
	header       string // optional: a note printed above the question
	items        []checklistItemData
	singleSelect bool   // true = pick exactly one
	itemName     string // names the answer line (e.g. "target", "agent")
}

// runChecklistTUI asks the question inline and leaves a ✓ line with the
// answer. It returns the chosen indices: nil when cancelled, empty when the
// user confirmed without choosing any.
func runChecklistTUI(cfg checklistConfig) ([]int, error) {
	if cfg.header != "" {
		fmt.Println(theme.Dim().Render(cfg.header))
	}
	options := checklistOptions(cfg.items)
	var values []string
	var err error
	if cfg.singleSelect {
		var v string
		v, err = ui.Select(cfg.title, options, checklistDefault(cfg.items))
		values = []string{v}
	} else {
		var pre []string
		for i, item := range cfg.items {
			if item.preSelected {
				pre = append(pre, strconv.Itoa(i))
			}
		}
		values, err = ui.MultiSelect(cfg.title, options, pre)
	}
	if errors.Is(err, ui.ErrCancelled) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	indices := make([]int, 0, len(values))
	labels := make([]string, 0, len(values))
	for _, v := range values {
		i, convErr := strconv.Atoi(v)
		if convErr != nil {
			return nil, convErr
		}
		indices = append(indices, i)
		labels = append(labels, cfg.items[i].label)
	}
	answer := strings.Join(labels, ", ")
	if answer == "" {
		answer = theme.Dim().Render("none")
	}
	ui.Answered(checklistAnswerLabel(cfg), answer)
	return indices, nil
}

// checklistOptions turns items into prompt options whose values are their
// indices, with descriptions dimmed and aligned after the labels.
func checklistOptions(items []checklistItemData) []ui.Option {
	width := 0
	for _, item := range items {
		width = max(width, len(item.label))
	}
	options := make([]ui.Option, len(items))
	for i, item := range items {
		label := item.label
		if item.desc != "" {
			label = fmt.Sprintf("%-*s  %s", width, item.label, theme.Dim().Render(item.desc))
		}
		options[i] = ui.Option{Label: label, Value: strconv.Itoa(i)}
	}
	return options
}

// checklistDefault is where a single choice starts: the first pre-selected
// item, else the first item.
func checklistDefault(items []checklistItemData) string {
	for i, item := range items {
		if item.preSelected {
			return strconv.Itoa(i)
		}
	}
	return "0"
}

// checklistAnswerLabel names the ✓ line: "Pattern", or "Targets" for a
// multi-select over targets.
func checklistAnswerLabel(cfg checklistConfig) string {
	name := cfg.itemName
	if name == "" {
		name = "choice"
	}
	if !cfg.singleSelect {
		name += "s"
	}
	return strings.ToUpper(name[:1]) + name[1:]
}
