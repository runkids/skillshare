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
