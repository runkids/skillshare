package main

import (
	"fmt"
	"strings"

	"skillshare/internal/theme"
	"skillshare/internal/ui"

	"github.com/mattn/go-runewidth"
)

// helpGroup is one titled block of a help page: commands, options or notes.
// A row with no description prints the name alone; a note group (rows with
// only a name and dim set) prints its lines dimmed.
type helpGroup struct {
	title string
	rows  []helpRow
	dim   bool
}

type helpRow struct {
	name, desc string
}

// printHelp prints a help page: the usage line, an optional intro, then
// each group with names aligned across the whole page. A description may
// hold several lines; the later ones line up under the first.
func printHelp(usage, intro string, groups ...helpGroup) {
	fmt.Println(theme.Primary().Bold(true).Render("Usage") + "  " + usage)
	if intro != "" {
		fmt.Println()
		fmt.Println(intro)
	}
	width := 0
	for _, g := range groups {
		for _, r := range g.rows {
			if r.desc != "" {
				width = max(width, runewidth.StringWidth(r.name))
			}
		}
	}
	for _, g := range groups {
		fmt.Println()
		fmt.Println(theme.Primary().Bold(true).Render(g.title))
		for _, r := range g.rows {
			if g.dim {
				fmt.Println("  " + ui.DimText(r.name))
				continue
			}
			name := theme.Accent().Render(r.name)
			if r.desc == "" {
				fmt.Println("  " + name)
				continue
			}
			pad := strings.Repeat(" ", width-runewidth.StringWidth(r.name))
			for i, line := range strings.Split(r.desc, "\n") {
				if i == 0 {
					fmt.Printf("  %s%s  %s\n", name, pad, line)
				} else {
					fmt.Printf("  %s  %s\n", strings.Repeat(" ", width), line)
				}
			}
		}
	}
}

// helpNotes builds a dimmed group of plain lines.
func helpNotes(title string, lines ...string) helpGroup {
	rows := make([]helpRow, len(lines))
	for i, l := range lines {
		rows[i] = helpRow{name: l}
	}
	return helpGroup{title: title, rows: rows, dim: true}
}

// suggestCommand returns the known command closest to name, or "" when
// none is within two edits.
func suggestCommand(name string, known []string) string {
	best, bestDist := "", 3
	for _, k := range known {
		if d := editDistance(name, k); d < bestDist {
			best, bestDist = k, d
		}
	}
	return best
}

// editDistance is the Levenshtein distance between a and b.
func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}
