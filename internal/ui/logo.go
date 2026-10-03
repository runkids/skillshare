package ui

import (
	"fmt"
	"io"
	"math"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"skillshare/internal/theme"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

const (
	logoSteps    = 16
	logoFrameGap = 55 * time.Millisecond
	logoTagline  = "Your AI coding setup, everywhere."
)

// wordmarkFont draws each letter of SKILLSHARE on a 5-pixel-high grid.
var wordmarkFont = map[rune][]string{
	'S': {".###", "#...", ".##.", "...#", "###."},
	'K': {"#..#", "#.#.", "##..", "#.#.", "#..#"},
	'I': {"###", ".#.", ".#.", ".#.", "###"},
	'L': {"#...", "#...", "#...", "#...", "####"},
	'H': {"#..#", "#..#", "####", "#..#", "#..#"},
	'A': {".##.", "#..#", "####", "#..#", "#..#"},
	'R': {"###.", "#..#", "###.", "#.#.", "#..#"},
	'E': {"####", "#...", "###.", "#...", "####"},
}

// wordmark is SKILLSHARE as pixel rows: 0 is empty, 1 belongs to SKILL and
// 2 to SHARE, which take different colors.
var wordmark = buildWordmark("SKILL", "SHARE")

type rgb struct{ r, g, b float64 }

// LogoBanner prints the SKILLSHARE wordmark, the version and tagline, then
// the given lines. On a terminal the wordmark first lights up from left to
// right. Without a terminal, without color, with only 16 colors, or on a
// narrow terminal it prints a plain "skillshare vX" line instead.
func LogoBanner(version string, lines []string, animate bool) {
	if !logoFits() {
		fmt.Println(theme.Primary().Bold(true).Render("skillshare") + " " + theme.Dim().Render("v"+version))
		fmt.Println(theme.Muted().Render(logoTagline))
		printLines(lines)
		return
	}
	if animate {
		playWordmark()
	} else {
		fmt.Print(strings.Join(wordmarkFrame(1), "\n") + "\n")
	}
	fmt.Println()
	fmt.Println(theme.Dim().Render("v" + version + " · " + logoTagline))
	printLines(lines)
}

func printLines(lines []string) {
	for _, line := range lines {
		fmt.Println(line)
	}
}

// playWordmark plays the animation, making sure Ctrl+C mid-animation does
// not leave the shell with a hidden cursor.
func playWordmark() {
	interrupted := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(interrupted, os.Interrupt, syscall.SIGTERM)
	defer func() {
		signal.Stop(interrupted)
		close(done)
	}()
	go func() {
		select {
		case <-interrupted:
			fmt.Print(showCursor + "\n")
			os.Exit(130)
		case <-done:
		}
	}()
	playLogo(os.Stdout, time.Sleep)
}

// playLogo writes the "light up" animation. The cursor is hidden while frames
// redraw in place, and each frame goes out in a single write.
func playLogo(w io.Writer, pause func(time.Duration)) {
	fmt.Fprint(w, hideCursor)
	for step := 0; step <= logoSteps; step++ {
		var b strings.Builder
		frame := wordmarkFrame(easeOut(float64(step) / logoSteps))
		if step > 0 {
			fmt.Fprintf(&b, "\x1b[%dA", len(frame))
		}
		for _, line := range frame {
			b.WriteString(line + "\x1b[K\n")
		}
		fmt.Fprint(w, b.String())
		if step < logoSteps {
			pause(logoFrameGap)
		}
	}
	fmt.Fprint(w, showCursor)
}

func logoFits() bool {
	t := theme.Get()
	if t.NoColor || t.Plain || !logoColorsSupported() {
		return false
	}
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	return err == nil && width > len(wordmark[0])
}

// logoColorsSupported reports whether the terminal has at least 256 colors;
// 16 colors flatten the animation's in-between shades.
func logoColorsSupported() bool {
	return lipgloss.ColorProfile() <= termenv.ANSI256
}

func buildWordmark(words ...string) [][]int {
	rows := make([][]int, 5)
	for part, word := range words {
		for _, letter := range word {
			for y, line := range wordmarkFont[letter] {
				for _, c := range line {
					v := 0
					if c == '#' {
						v = part + 1
					}
					rows[y] = append(rows[y], v)
				}
				rows[y] = append(rows[y], 0)
			}
		}
	}
	for y := range rows {
		rows[y] = rows[y][:len(rows[y])-1]
	}
	return rows
}

// wordmarkSplit is the first column of SHARE.
func wordmarkSplit() int {
	for x := range wordmark[0] {
		for y := range wordmark {
			if wordmark[y][x] == 2 {
				return x
			}
		}
	}
	return 0
}

func wordmarkSkillColor() rgb {
	if theme.Get().Mode == theme.ModeLight {
		return rgb{0x1d, 0x6f, 0xd8}
	}
	return rgb{0x2e, 0x8b, 0xf0}
}

func wordmarkShareColor() rgb {
	if theme.Get().Mode == theme.ModeLight {
		return rgb{0xc8, 0x8a, 0x00}
	}
	return rgb{0xf5, 0xb8, 0x1c}
}

// wordmarkFrame renders the wordmark as half blocks at the given animation
// progress (0..1): two pixel rows per line.
func wordmarkFrame(progress float64) []string {
	colors := wordmarkColors(progress)
	var out []string
	for y := 0; y < len(colors); y += 2 {
		var b strings.Builder
		for x := range colors[y] {
			var lower *rgb
			if y+1 < len(colors) {
				lower = colors[y+1][x]
			}
			b.WriteString(halfBlock(colors[y][x], lower))
		}
		out = append(out, strings.TrimRight(b.String(), " "))
	}
	return out
}

// wordmarkColors returns each pixel's color at the given progress, nil for
// empty pixels. Letters start as a faint grey and take on their color from
// left to right.
func wordmarkColors(progress float64) [][]*rgb {
	bg := rgb{18, 19, 22}
	if theme.Get().Mode == theme.ModeLight {
		bg = rgb{250, 250, 250}
	}
	width := float64(len(wordmark[0]) - 1)
	out := make([][]*rgb, len(wordmark))
	for y, row := range wordmark {
		out[y] = make([]*rgb, len(row))
		for x, part := range row {
			if part == 0 {
				continue
			}
			c := wordmarkSkillColor()
			if part == 2 {
				c = wordmarkShareColor()
			}
			lum := 0.3*c.r + 0.59*c.g + 0.11*c.b
			grey := mix(bg, rgb{lum, lum, lum}, 0.4)
			k := math.Max(0, math.Min(1, (progress*1.3-float64(x)/width)/0.3))
			mixed := mix(grey, c, k)
			out[y][x] = &mixed
		}
	}
	return out
}

func halfBlock(upper, lower *rgb) string {
	switch {
	case upper != nil && lower != nil:
		return lipgloss.NewStyle().Foreground(upper.color()).Background(lower.color()).Render("▀")
	case upper != nil:
		return lipgloss.NewStyle().Foreground(upper.color()).Render("▀")
	case lower != nil:
		return lipgloss.NewStyle().Foreground(lower.color()).Render("▄")
	default:
		return " "
	}
}

func mix(a, b rgb, k float64) rgb {
	return rgb{a.r + (b.r-a.r)*k, a.g + (b.g-a.g)*k, a.b + (b.b-a.b)*k}
}

func (c rgb) color() lipgloss.Color {
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", int(c.r), int(c.g), int(c.b)))
}

func easeOut(t float64) float64 { return 1 - math.Pow(1-t, 3) }
