package ui

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"skillshare/internal/theme"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// ErrCancelled is returned when the user leaves a prompt with esc or ctrl+c.
var ErrCancelled = errors.New("cancelled")

// promptVisibleRows is how many options a list shows before it scrolls.
const promptVisibleRows = 9

// answerLabelWidth aligns the values of collapsed answers.
const answerLabelWidth = 8

// Option is one choice in Select or MultiSelect.
type Option struct {
	Label string
	Value string
}

// Select asks for one option and returns its value. The prompt runs inline
// and erases itself when answered; call Answered to leave a summary line.
// A list longer than the visible rows can be narrowed with /.
func Select(title string, options []Option, selected string) (string, error) {
	value := selected
	field := huh.NewSelect[string]().
		Title(promptTitle(title)).
		Options(huhOptions(options, nil)...).
		Value(&value)
	long := len(options) > promptVisibleRows
	if long {
		field = field.Height(promptVisibleRows + 1)
	}
	err := runPrompt(field, long)
	return value, err
}

// MultiSelect asks for any number of options and returns the chosen values
// in option order. Values in selected start checked. Like Select, a list
// longer than the visible rows can be narrowed with /.
func MultiSelect(title string, options []Option, selected []string) ([]string, error) {
	checked := map[string]bool{}
	for _, v := range selected {
		checked[v] = true
	}
	var values []string
	field := huh.NewMultiSelect[string]().
		Title(promptTitle(title)).
		Options(huhOptions(options, checked)...).
		Value(&values)
	long := len(options) > promptVisibleRows
	field = field.Filterable(long)
	if long {
		field = field.Height(promptVisibleRows + 1)
	}
	err := runPrompt(field, long)
	return values, err
}

// Confirm asks a yes/no question; def is the answer Enter gives. Without a
// terminal it reads one line instead, so piped answers such as
// `echo y | skillshare uninstall x` keep working.
func Confirm(title string, def bool) (bool, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return lineConfirm(title, def, os.Stdin, os.Stdout), nil
	}
	value := def
	field := huh.NewConfirm().
		Title(promptTitle(title)).
		Affirmative("Yes").
		Negative("No").
		Inline(true).
		Value(&value)
	err := runPrompt(field, false)
	return value, err
}

// ConfirmAction is Confirm for a question that guards one action: esc
// answers no instead of returning ErrCancelled.
func ConfirmAction(title string, def bool) (bool, error) {
	ok, err := Confirm(title, def)
	if errors.Is(err, ErrCancelled) {
		return false, nil
	}
	return ok, err
}

// Cancelled reports a declined confirmation, e.g. Cancelled("removed").
func Cancelled(nothingWas string) {
	fmt.Println(theme.Dim().Render("Cancelled. Nothing was " + nothingWas + "."))
}

// lineConfirm asks a yes/no question as plain text: y/yes or n/no, and
// anything else, including end of input, takes def.
func lineConfirm(title string, def bool, in io.Reader, out io.Writer) bool {
	hint := "[y/N]"
	if def {
		hint = "[Y/n]"
	}
	fmt.Fprintf(out, "%s %s ", promptTitle(title), theme.Dim().Render(hint))
	answer := strings.ToLower(strings.TrimSpace(readLine(in)))
	fmt.Fprintln(out)
	switch answer {
	case "y", "yes":
		return true
	case "n", "no":
		return false
	}
	return def
}

// readLine reads up to the next newline one byte at a time, so it never
// takes input meant for a later question.
func readLine(in io.Reader) string {
	var line []byte
	b := make([]byte, 1)
	for {
		n, err := in.Read(b)
		if n == 1 {
			if b[0] == '\n' {
				break
			}
			line = append(line, b[0])
		}
		if err != nil {
			break
		}
	}
	return string(line)
}

// Input asks for one line of text. placeholder is shown while it is empty.
func Input(title, placeholder, value string) (string, error) {
	return InputValid(title, placeholder, value, func(string) error { return nil })
}

// InputValid is Input that only accepts an answer check passes; otherwise
// the error shows under the question and it stays open.
func InputValid(title, placeholder, value string, check func(string) error) (string, error) {
	err := runPrompt(inputField(title, placeholder, &value, check), false)
	return value, err
}

func inputField(title, placeholder string, value *string, check func(string) error) *huh.Input {
	return huh.NewInput().
		Title(promptTitle(title)).
		Prompt("› ").
		Placeholder(placeholder).
		Validate(check).
		Value(value)
}

// Text asks for text that may span lines, such as pasted JSON: enter
// submits and alt+enter starts a new line. The box grows with value, from 3
// to 10 lines.
func Text(title, value string) (string, error) {
	err := runPrompt(textField(title, &value), false)
	return value, err
}

func textField(title string, value *string) *huh.Text {
	lines := min(max(strings.Count(*value, "\n")+1, 3), 10)
	return huh.NewText().
		Title(promptTitle(title)).
		Lines(lines).
		CharLimit(1 << 20).
		ExternalEditor(false).
		Value(value)
}

// Answered prints the one-line record an answered prompt collapses into.
func Answered(label, value string) {
	fmt.Printf("%s %-*s %s\n", theme.Success().Render("✓"), answerLabelWidth, label, value)
}

func runPrompt(field huh.Field, filterable bool) error {
	return runForm(field, filterable, os.Stdin, os.Stdout)
}

// runForm runs the form itself instead of huh's Run, which cancels through
// tea.Interrupt: Bubble Tea treats that as a kill and skips the final empty
// frame, leaving the question on screen after esc.
func runForm(field huh.Field, filterable bool, in io.Reader, out io.Writer) error {
	keys := huh.NewDefaultKeyMap()
	keys.Quit = key.NewBinding(key.WithKeys("ctrl+c"))
	keys.Select.Filter.SetEnabled(filterable)
	form := huh.NewForm(huh.NewGroup(field)).
		WithTheme(promptTheme()).
		WithKeyMap(keys).
		WithShowHelp(false)
	form.SubmitCmd = tea.Quit
	form.CancelCmd = tea.Quit
	m := &promptModel{form: form, field: field, hint: promptHint(field, filterable)}
	if _, err := tea.NewProgram(m, tea.WithInput(in), tea.WithOutput(out)).Run(); err != nil {
		if errors.Is(err, tea.ErrInterrupted) {
			return ErrCancelled
		}
		return err
	}
	if m.cancelled || form.State == huh.StateAborted {
		return ErrCancelled
	}
	return nil
}

// promptModel decides what esc means. huh's form quits on its Quit key
// before the field sees it, so binding esc there would cancel the question
// while the user only meant to leave the filter.
type promptModel struct {
	form      *huh.Form
	field     huh.Field
	cancelled bool
	hint      string
}

// filtering asks the field itself: enter with no match keeps huh's Select
// in the filter, which key presses alone cannot tell.
func (m *promptModel) filtering() bool {
	f, ok := m.field.(interface{ GetFiltering() bool })
	return ok && f.GetFiltering()
}

func (m *promptModel) Init() tea.Cmd { return m.form.Init() }

func (m *promptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok && k.String() == "esc" && !m.filtering() {
		m.cancelled = true
		return m, tea.Quit
	}
	form, cmd := m.form.Update(msg)
	m.form = form.(*huh.Form)
	return m, cmd
}

// View draws nothing once answered or cancelled, so the question is erased.
func (m *promptModel) View() string {
	if m.cancelled || m.form.State != huh.StateNormal {
		return ""
	}
	return m.form.View() + "\n\n" + m.hint
}

// promptHint is the key line under a question, in the same words the
// full-screen TUIs use.
func promptHint(field huh.Field, filterable bool) string {
	var pairs [][2]string
	switch field.(type) {
	case *huh.Select[string]:
		pairs = [][2]string{{"↑↓", "move"}, {"enter", "choose"}}
	case *huh.MultiSelect[string]:
		pairs = [][2]string{{"↑↓", "move"}, {"space", "toggle"}, {"ctrl+a", "all"}, {"enter", "confirm"}}
	case *huh.Confirm:
		pairs = [][2]string{{"y", "yes"}, {"n", "no"}}
	case *huh.Text:
		pairs = [][2]string{{"enter", "confirm"}, {"alt+enter", "new line"}}
	default:
		pairs = [][2]string{{"enter", "confirm"}}
	}
	if filterable {
		pairs = append(pairs, [2]string{"/", "filter"})
	}
	pairs = append(pairs, [2]string{"esc", "cancel"})
	parts := make([]string, len(pairs))
	for i, p := range pairs {
		parts[i] = theme.Primary().Render(p[0]) + " " + theme.Dim().Render(p[1])
	}
	return "  " + strings.Join(parts, theme.Dim().Render(" · "))
}

// focusedButton fills the chosen Yes/No button with the accent color.
func focusedButton(button, accent lipgloss.Style) lipgloss.Style {
	t := theme.Get()
	if t.NoColor {
		return button.Reverse(true)
	}
	text := lipgloss.Color("0")
	if t.Mode == theme.ModeLight {
		text = lipgloss.Color("15")
	}
	return button.Background(accent.GetForeground()).Foreground(text).Bold(true)
}

// promptTitle marks a question with an accent "?", matching the "›" cursor.
func promptTitle(title string) string {
	return theme.Accent().Render("?") + " " + theme.Primary().Render(title)
}

func huhOptions(options []Option, checked map[string]bool) []huh.Option[string] {
	out := make([]huh.Option[string], len(options))
	for i, o := range options {
		out[i] = huh.NewOption(o.Label, o.Value).Selected(checked[o.Value])
	}
	return out
}

// promptTheme maps the skillshare palette onto huh's styles: no side bar,
// "›" cursor, ◉/○ checkboxes, accent-colored focus.
func promptTheme() *huh.Theme {
	t := huh.ThemeBase()
	primary, accent := theme.Primary(), theme.Accent()
	success, dim := theme.Success(), theme.Dim()

	f := &t.Focused
	f.Base = lipgloss.NewStyle()
	f.Card = f.Base
	f.Title = lipgloss.NewStyle()
	f.Description = dim
	f.ErrorIndicator = theme.Danger().SetString(" *")
	f.ErrorMessage = theme.Danger()
	f.SelectSelector = accent.SetString("› ")
	f.Option = primary
	f.SelectedOption = success
	f.MultiSelectSelector = accent.SetString("› ")
	f.SelectedPrefix = success.SetString("◉ ")
	f.UnselectedPrefix = dim.SetString("○ ")
	f.UnselectedOption = primary
	f.NextIndicator = dim.MarginLeft(1).SetString("›")
	f.PrevIndicator = dim.MarginRight(1).SetString("‹")
	button := lipgloss.NewStyle().Padding(0, 2).MarginLeft(1)
	f.FocusedButton = focusedButton(button, accent)
	f.BlurredButton = button.Inherit(dim)
	f.TextInput.Cursor = accent
	f.TextInput.Placeholder = dim
	f.TextInput.Prompt = accent
	f.TextInput.Text = primary

	t.Blurred = t.Focused
	t.Help.ShortKey = dim
	t.Help.ShortDesc = dim
	t.Help.ShortSeparator = dim
	return t
}
