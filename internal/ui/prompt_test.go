package ui

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/ansi"
)

// syncBuffer lets the test read what the program has drawn so far.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestRunForm_EscClearsThePrompt(t *testing.T) {
	const title = "Your skillshare repo"
	in, keys := io.Pipe()
	out := &syncBuffer{}
	go func() {
		deadline := time.Now().Add(5 * time.Second)
		for !strings.Contains(out.String(), title) && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		keys.Write([]byte("\x1b"))
	}()

	err := runForm(huh.NewInput().Title(title), false, in, out)

	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("err = %v, want ErrCancelled", err)
	}
	rendered := out.String()
	last := strings.LastIndex(rendered, title)
	if last < 0 {
		t.Fatalf("prompt never rendered: %q", rendered)
	}
	if !strings.Contains(rendered[last:], "\x1b[J") {
		t.Errorf("prompt was not erased after cancel; output tail = %q", rendered[last:])
	}
}

func TestRunForm_EscLeavesTheFilterFirst(t *testing.T) {
	const title = "Pick some"
	in, keys := io.Pipe()
	out := &syncBuffer{}
	go func() {
		deadline := time.Now().Add(5 * time.Second)
		for !strings.Contains(out.String(), title) && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		for _, k := range []string{"/", "x", "\x1b", "\r"} {
			keys.Write([]byte(k))
			time.Sleep(50 * time.Millisecond)
		}
	}()

	var values []string
	field := huh.NewMultiSelect[string]().Title(title).
		Options(huh.NewOptions("a", "b", "c")...).Filterable(true).Value(&values)
	err := runForm(field, true, in, out)

	if err != nil {
		t.Fatalf("esc in the filter cancelled the prompt: err = %v", err)
	}
}

func TestRunForm_EscLeavesTheFilterAfterEnterFindsNothing(t *testing.T) {
	const title = "Pick a folder"
	in, keys := io.Pipe()
	out := &syncBuffer{}
	go func() {
		deadline := time.Now().Add(5 * time.Second)
		for !strings.Contains(out.String(), title) && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		// enter with no match keeps huh in the filter, so esc must leave it.
		for _, k := range []string{"/", "z", "z", "\r", "\x1b", "\r"} {
			keys.Write([]byte(k))
			time.Sleep(50 * time.Millisecond)
		}
	}()

	value := ""
	field := huh.NewSelect[string]().Title(title).
		Options(huh.NewOptions("a", "b", "c")...).Filtering(false).Value(&value)
	if err := runForm(field, true, in, out); err != nil {
		t.Fatalf("esc in the filter cancelled the prompt: err = %v", err)
	}
	if value != "a" {
		t.Fatalf("value = %q, want the first option", value)
	}
}

func TestRunForm_TextTakesNewLinesAndSubmitsOnEnter(t *testing.T) {
	const title = "Paste server JSON"
	in, keys := io.Pipe()
	out := &syncBuffer{}
	go func() {
		deadline := time.Now().Add(5 * time.Second)
		for !strings.Contains(out.String(), title) && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		keys.Write([]byte("one\x1b\rtwo\r")) // alt+enter, then enter
	}()

	value := ""
	if err := runForm(textField(title, &value), false, in, out); err != nil {
		t.Fatalf("err = %v", err)
	}
	if value != "one\ntwo" {
		t.Fatalf("value = %q, want two lines", value)
	}
}

func TestRunForm_InputValidKeepsAskingUntilTheCheckPasses(t *testing.T) {
	const title = "Extra name"
	in, keys := io.Pipe()
	out := &syncBuffer{}
	go func() {
		deadline := time.Now().Add(5 * time.Second)
		for !strings.Contains(out.String(), title) && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		keys.Write([]byte("a/b\r"))
		for !strings.Contains(out.String(), "no slashes") && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		keys.Write([]byte("\x7f\x7f\r")) // backspace twice leaves "a"
	}()

	value := ""
	check := func(v string) error {
		if strings.Contains(v, "/") {
			return errors.New("no slashes")
		}
		return nil
	}
	if err := runForm(inputField(title, "", &value, check), false, in, out); err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(out.String(), "no slashes") {
		t.Errorf("the check's error was never shown")
	}
	if value != "a" {
		t.Fatalf("value = %q, want the corrected answer", value)
	}
}

func TestLineConfirm_ReadsOneAnswerPerLine(t *testing.T) {
	in := strings.NewReader("y\nno\n\n")
	var out bytes.Buffer

	got := []bool{
		lineConfirm("First?", false, in, &out),
		lineConfirm("Second?", true, in, &out),
		lineConfirm("Third?", true, in, &out),
	}

	if got[0] != true || got[1] != false || got[2] != true {
		t.Errorf("answers = %v, want [true false true]", got)
	}
}

func TestLineConfirm_EOFTakesTheDefault(t *testing.T) {
	var out bytes.Buffer
	if lineConfirm("Remove?", false, strings.NewReader(""), &out) {
		t.Error("empty input answered yes, want the default no")
	}
}

func TestLineConfirm_ShowsTheDefault(t *testing.T) {
	var out bytes.Buffer
	lineConfirm("Remove?", false, strings.NewReader("\n"), &out)
	if !strings.Contains(out.String(), "Remove? [y/N]") {
		t.Errorf("output = %q, want the question with [y/N]", out.String())
	}
}

func TestFitOptionLabel_KeepsALongDescriptionOnOneRow(t *testing.T) {
	label := "agent-browser  Browser automation CLI for AI agents.\nUse when the user needs to interact with websites."
	got := fitOptionLabel(label, 40)

	if strings.Contains(got, "\n") || ansi.StringWidth(got) > 40-optionPrefixWidth || !strings.HasSuffix(got, "…") {
		t.Fatalf("fitOptionLabel() = %q, want one row cut to %d columns", got, 40-optionPrefixWidth)
	}
}
