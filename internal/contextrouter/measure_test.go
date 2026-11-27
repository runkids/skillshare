package contextrouter

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func measure(t *testing.T, dir string, lim Limits) Report {
	t.Helper()
	report, err := Measure(context.Background(), dir, lim)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func topicNamed(t *testing.T, report Report, name string) Topic {
	t.Helper()
	for _, topic := range report.Topics {
		if topic.Name == name {
			return topic
		}
	}
	t.Fatalf("no topic %q in %+v", name, report.Topics)
	return Topic{}
}

// Every size equals what the reference script printed for the same folder.
func TestMeasureGoldens(t *testing.T) {
	for _, c := range loadCases(t) {
		t.Run(c.Name, func(t *testing.T) {
			report := measure(t, c.materialize(t), DefaultLimits)
			if report.Error != nil || report.Reference != ReferenceCommit {
				t.Fatalf("report: %+v", report)
			}
			if !sameSize(report.Kernel.Bytes, c.Expect.Kernel) {
				t.Errorf("kernel: got %v, want %v", show(report.Kernel.Bytes), show(c.Expect.Kernel))
			}
			if len(report.Topics) != len(c.Expect.Topics) {
				t.Fatalf("topics: got %d, want %d", len(report.Topics), len(c.Expect.Topics))
			}
			for _, topic := range report.Topics {
				want, ok := c.Expect.Topics[topic.Name]
				if !ok || !sameSize(topic.Bytes, want) {
					t.Errorf("%s: got %v, want %v", topic.Name, show(topic.Bytes), show(want))
				}
			}
		})
	}
}

func TestMeasureStates(t *testing.T) {
	report := measure(t, goldenNamed(t, "cap-edge").materialize(t), DefaultLimits)
	if equal := topicNamed(t, report, "equal"); *equal.Bytes != equal.Cap {
		t.Errorf("equal: %d bytes, cap %d", *equal.Bytes, equal.Cap)
	}
	if over := topicNamed(t, report, "over"); *over.Bytes != over.Cap+1 {
		t.Errorf("over: %d bytes, cap %d", *over.Bytes, over.Cap)
	}

	// A failed source leaves the topic without a size, as the reference does;
	// the sources that did load keep theirs.
	broken := topicNamed(t, measure(t, goldenNamed(t, "good-then-bad").materialize(t), DefaultLimits), "broken")
	if broken.Bytes != nil || broken.Reason != ReasonSource || len(broken.Sources) != 2 {
		t.Fatalf("broken: %+v", broken)
	}
	if first, second := broken.Sources[0], broken.Sources[1]; first.Bytes == nil || first.Error != "" ||
		second.Bytes != nil || second.Error != ErrHeading || second.Count == nil || *second.Count != 0 || second.Heading != "Gone" {
		t.Errorf("sources: %+v", broken.Sources)
	}

	twice := topicNamed(t, measure(t, goldenNamed(t, "headings").materialize(t), DefaultLimits), "twice")
	if twice.Sources[0].Count == nil || *twice.Sources[0].Count != 2 {
		t.Errorf("duplicate heading: %+v", twice.Sources)
	}
	if repeat := topicNamed(t, measure(t, goldenNamed(t, "repeat").materialize(t), DefaultLimits), "twice"); len(repeat.Sources) != 2 {
		t.Errorf("repeat: %+v", repeat.Sources)
	}

	report = measure(t, goldenNamed(t, "unlisted").materialize(t), DefaultLimits)
	if hidden, named := topicNamed(t, report, "hidden"), topicNamed(t, report, "named"); hidden.Listed == nil || *hidden.Listed || named.Listed == nil || !*named.Listed {
		t.Errorf("listed: hidden %v, named %v", hidden.Listed, named.Listed)
	}

	report = measure(t, goldenNamed(t, "missing-kernel").materialize(t), DefaultLimits)
	if report.Kernel.Error != ErrMissing || report.Kernel.Bytes != nil || report.Topics[0].Listed != nil || report.Topics[0].Bytes == nil {
		t.Errorf("missing kernel: %+v %+v", report.Kernel, report.Topics[0])
	}

	// A declared mount is listed and nothing under it is measured.
	report = measure(t, goldenNamed(t, "mounts").materialize(t), DefaultLimits)
	if len(report.Mounts) != 1 || report.Mounts[0] != (Mount{Name: "kb", Root: "modules/kb"}) || len(report.Topics) != 1 {
		t.Errorf("mounts: %+v, topics %+v", report.Mounts, report.Topics)
	}

	// A router that compacts path links is sized like any other.
	report = measure(t, goldenNamed(t, "compact-basic").materialize(t), DefaultLimits)
	for _, topic := range report.Topics {
		if topic.Bytes == nil || topic.Reason != "" {
			t.Errorf("compacted %s: %+v", topic.Name, topic)
		}
	}
}

func TestMeasureConfigProblems(t *testing.T) {
	empty := measure(t, t.TempDir(), DefaultLimits)
	if empty.Error == nil || empty.Error.Code != ConfigUnreadable || empty.Error.Detail != ErrMissing || empty.Kernel != nil || empty.Topics == nil || len(empty.Topics) != 0 {
		t.Fatalf("no config: %+v", empty)
	}
	broken := measure(t, writeTree(t, map[string]string{ConfigRel: "{"}), DefaultLimits)
	if broken.Error == nil || broken.Error.Code != ConfigInvalid || !strings.Contains(broken.Error.Detail, "invalid JSON") {
		t.Fatalf("broken config: %+v", broken.Error)
	}
	large := measure(t, writeTree(t, map[string]string{ConfigRel: `{"topics":{}}`}), Limits{MaxConfigBytes: 5})
	if large.Error == nil || large.Error.Detail != ErrTooLarge {
		t.Fatalf("large config: %+v", large.Error)
	}
	if runtime.GOOS == "windows" {
		return
	}
	// A config that is a symlink out of the folder is not read.
	outside := writeTree(t, map[string]string{"real.json": `{"topics":{}}`})
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "wiki"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "real.json"), filepath.Join(dir, "wiki", "ai-context.json")); err != nil {
		t.Fatal(err)
	}
	linked := measure(t, dir, DefaultLimits)
	if linked.Error == nil || linked.Error.Code != ConfigUnreadable || linked.Error.Detail != ErrOutside {
		t.Fatalf("linked config: %+v", linked.Error)
	}
}

func TestMeasureOutside(t *testing.T) {
	base := t.TempDir()
	writeFile(t, filepath.Join(base, "secret.md"), "secret")
	dir := filepath.Join(base, "root")
	writeFile(t, filepath.Join(dir, "AGENTS.md"), "`t`")
	writeFile(t, filepath.Join(dir, ConfigRel), `{"rootInstructions":"../secret.md","topics":{"t":{"description":"","sources":[{"path":"../secret.md"},{"path":"/etc/hosts"}]}}}`)
	report := measure(t, dir, DefaultLimits)
	if report.Kernel.Error != ErrOutside || report.Kernel.Bytes != nil {
		t.Errorf("kernel: %+v", report.Kernel)
	}
	for _, source := range report.Topics[0].Sources {
		if source.Error != ErrOutside || source.Bytes != nil {
			t.Errorf("source: %+v", source)
		}
	}
}

func TestMeasureLimits(t *testing.T) {
	page := strings.Repeat("x", 100)
	source := `{"path":"wiki/a.md"}`
	three := writeTree(t, map[string]string{
		"AGENTS.md": "`a` `b` `c`", "wiki/a.md": page, "wiki/b.md": page,
		ConfigRel: `{"topics":{
		  "a":{"description":"","sources":[` + source + `]},
		  "b":{"description":"","sources":[{"path":"wiki/b.md"}]},
		  "c":{"description":"","sources":[` + source + `]}}}`,
	})
	limited := func(change func(*Limits)) Report {
		lim := DefaultLimits
		change(&lim)
		return measure(t, three, lim)
	}
	sized := func(report Report) string {
		var out []string
		for _, topic := range report.Topics {
			if topic.Bytes != nil {
				out = append(out, topic.Name)
			} else if topic.Reason != ReasonLimit {
				t.Errorf("%s: reason %q", topic.Name, topic.Reason)
			}
		}
		return strings.Join(out, ",")
	}

	if report := limited(func(l *Limits) {}); sized(report) != "a,b,c" || report.Truncated {
		t.Fatalf("defaults: %s truncated=%v", sized(report), report.Truncated)
	}
	if report := limited(func(l *Limits) { l.MaxTopics = 2 }); sized(report) != "a,b" || !report.Truncated {
		t.Errorf("MaxTopics: %s truncated=%v", sized(report), report.Truncated)
	}
	if report := limited(func(l *Limits) { l.MaxSources = 1 }); sized(report) != "a" || !report.Truncated {
		t.Errorf("MaxSources: %s truncated=%v", sized(report), report.Truncated)
	}
	// One body fits; the second distinct page does not. Topic c reuses the first body.
	if report := limited(func(l *Limits) { l.MaxScanBytes = 150 }); sized(report) != "a,c" || !report.Truncated {
		t.Errorf("MaxScanBytes: %s truncated=%v", sized(report), report.Truncated)
	}

	// The same source fifty times is read and scanned once.
	repeated := writeTree(t, map[string]string{
		"AGENTS.md": "`a`", "wiki/a.md": page,
		ConfigRel: `{"topics":{"a":{"description":"","sources":[` + strings.TrimSuffix(strings.Repeat(source+",", 50), ",") + `]}}}`,
	})
	lim := DefaultLimits
	lim.MaxScanBytes = 100
	topic := measure(t, repeated, lim).Topics[0]
	want := 50*(len("<!-- wiki/a.md -->\n")+100) + 49*2
	if topic.Bytes == nil || *topic.Bytes != want {
		t.Errorf("repeated source: got %v, want %d", show(topic.Bytes), want)
	}
}

func TestMeasureCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Measure(ctx, goldenNamed(t, "repeat").materialize(t), DefaultLimits); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
	if _, err := Measure(context.Background(), filepath.Join(t.TempDir(), "gone"), DefaultLimits); err == nil {
		t.Fatal("a folder that does not exist measured")
	}
}
