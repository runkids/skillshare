package contextrouter

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
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
			if report.Error != nil || report.SemanticsVersion != SemanticsVersion {
				t.Fatalf("report: %+v", report)
			}
			if !sameSize(report.Kernel.Bytes, c.Expect.Kernel) {
				t.Errorf("kernel: got %v, want %v", show(report.Kernel.Bytes), show(c.Expect.Kernel))
			}
			if len(report.Topics) != len(c.Expect.Topics) {
				t.Fatalf("topics: got %d, want %d", len(report.Topics), len(c.Expect.Topics))
			}
			for i, topic := range report.Topics {
				want, ok := c.Expect.Topics[topic.Name]
				if !ok || !sameSize(topic.Bytes, want) {
					t.Errorf("%s: got %v, want %v", topic.Name, show(topic.Bytes), show(want))
				}
				// Topics come in the config's order, as the reference prints them.
				if topic.Name != c.Expect.Order[i] {
					t.Errorf("topic %d: got %s, want %s", i, topic.Name, c.Expect.Order[i])
				}
				if description := c.Expect.Descriptions[topic.Name]; description == nil || topic.Description != *description {
					t.Errorf("%s description: got %q, want %v", topic.Name, topic.Description, description)
				}
			}
		})
	}
}

// A folder whose own script predates an option gets the sizes that script
// prints: it ignores the option, so the report does too.
func TestMeasureLegacyScripts(t *testing.T) {
	differs := 0
	for _, c := range loadCases(t) {
		for version, want := range c.Legacy {
			if c.Name == "missing-kernel" && version == "0.2.1" {
				continue // that script stops at a missing kernel and prints nothing
			}
			report := measure(t, c.withScript(t, version), DefaultLimits)
			if report.VersionStatus != VersionLegacy || report.ScriptVersion != version || report.SemanticsVersion != version {
				t.Fatalf("%s %s: status %q, script %q, semantics %q", c.Name, version, report.VersionStatus, report.ScriptVersion, report.SemanticsVersion)
			}
			if !sameSize(report.Kernel.Bytes, want.Kernel) {
				t.Errorf("%s %s kernel: got %v, want %v", c.Name, version, show(report.Kernel.Bytes), show(want.Kernel))
			}
			for _, topic := range report.Topics {
				if !sameSize(topic.Bytes, want.Topics[topic.Name]) {
					t.Errorf("%s %s %s: got %v, want %v", c.Name, version, topic.Name, show(topic.Bytes), show(want.Topics[topic.Name]))
				}
				if !sameSize(want.Topics[topic.Name], c.Expect.Topics[topic.Name]) {
					differs++
				}
			}
		}
	}
	if differs < 4 {
		t.Fatalf("only %d legacy sizes differ from the current rule; the fixtures no longer exercise it", differs)
	}
}

// The script's version picks the rule and is reported; whatever it is, the folder is measured.
func TestMeasureScriptVersions(t *testing.T) {
	c := goldenNamed(t, "compact-basic") // sets stripFrontmatter and compactPathLinks
	sizes := func(report Report) string {
		out := fmt.Sprint(show(report.Kernel.Bytes))
		for _, topic := range report.Topics {
			out += fmt.Sprintf(" %s=%v", topic.Name, show(topic.Bytes))
		}
		return out
	}
	unknown := measure(t, c.materialize(t), DefaultLimits)
	if unknown.VersionStatus != VersionUnknown || unknown.ScriptVersion != "" || unknown.SemanticsVersion != SemanticsVersion {
		t.Fatalf("no script: %+v", unknown)
	}
	current := sizes(unknown)
	for version, status := range map[string]string{
		"0.5.0": VersionOK, "0.6.0": VersionOK, "0.6.9": VersionOK,
		"0.7.0": VersionNewer, "1.0.0": VersionNewer, "99.0.0": VersionNewer,
	} {
		report := measure(t, c.withScript(t, version), DefaultLimits)
		if report.VersionStatus != status || report.ScriptVersion != version || report.SemanticsVersion != SemanticsVersion {
			t.Errorf("%s: status %q, script %q, semantics %q", version, report.VersionStatus, report.ScriptVersion, report.SemanticsVersion)
		}
		if got := sizes(report); got != current || report.Topics[0].Bytes == nil {
			t.Errorf("%s: sizes %s, want %s", version, got, current)
		}
	}
	legacy := measure(t, c.withScript(t, "0.2.1"), DefaultLimits)
	if legacy.VersionStatus != VersionLegacy || sizes(legacy) == current {
		t.Errorf("0.2.1 with both options set: status %q, sizes %s", legacy.VersionStatus, sizes(legacy))
	}

	// The script is text to read. One that would exit when run is still only read.
	dir := c.materialize(t)
	writeFile(t, filepath.Join(dir, filepath.FromSlash(ScriptRel)), "raise SystemExit('never run')\nVERSION = \"0.6.0\"\n")
	if report := measure(t, dir, DefaultLimits); report.VersionStatus != VersionOK || sizes(report) != current {
		t.Errorf("script with a raise: %+v", report)
	}
	// A version cannot be read from a script past the file limit.
	writeFile(t, filepath.Join(dir, filepath.FromSlash(ScriptRel)), "VERSION = '0.2.1'\n"+strings.Repeat("#", 1<<20))
	if report := measure(t, dir, DefaultLimits); report.VersionStatus != VersionUnknown || sizes(report) != current {
		t.Errorf("oversized script: status %q", report.VersionStatus)
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
	// A folder that vanished after it was listed is a report, so the caller can still offer the others.
	gone, err := Measure(context.Background(), filepath.Join(t.TempDir(), "gone"), DefaultLimits)
	if err != nil || gone.Error == nil || gone.Error.Code != ConfigUnreadable || gone.Error.Detail != ErrMissing {
		t.Fatalf("gone folder: %+v %v", gone.Error, err)
	}
}

// A hostile folder cannot make one report read without bound, scan a body
// built to stress the link scanner, or run past its time.
func TestMeasureHostile(t *testing.T) {
	if runtime.GOOS != "windows" {
		// Twenty names for one page: each is read and kept, so the read budget must stop it.
		files := map[string]string{"AGENTS.md": "", "wiki/page.md": strings.Repeat("x", 1000)}
		topics := []string{}
		for i := 0; i < 20; i++ {
			topics = append(topics, fmt.Sprintf(`"t%02d":{"description":"","sources":[{"path":"wiki/link%02d.md"}]}`, i, i))
		}
		files[ConfigRel] = `{"topics":{` + strings.Join(topics, ",") + `}}`
		dir := writeTree(t, files)
		for i := 0; i < 20; i++ {
			if err := os.Symlink("page.md", filepath.Join(dir, "wiki", fmt.Sprintf("link%02d.md", i))); err != nil {
				t.Fatal(err)
			}
		}
		lim := DefaultLimits
		lim.MaxScanBytes = 3000
		report := measure(t, dir, lim)
		sized := 0
		for _, topic := range report.Topics {
			if topic.Bytes != nil {
				sized++
			} else if topic.Reason != ReasonLimit {
				t.Errorf("%s: reason %q", topic.Name, topic.Reason)
			}
		}
		if sized != 3 || !report.Truncated {
			t.Errorf("read budget: %d topics sized, truncated=%v", sized, report.Truncated)
		}
	}

	// The scanner takes minutes on a megabyte of unclosed links; such a body is not scanned.
	hostile := writeTree(t, map[string]string{
		"AGENTS.md": "`a`", "wiki/a.md": strings.Repeat("[a](", 200_000),
		ConfigRel: `{"compactPathLinks":true,"topics":{"a":{"description":"","sources":[{"path":"wiki/a.md"}]}}}`,
	})
	start := time.Now()
	report := measure(t, hostile, DefaultLimits)
	if topic := report.Topics[0]; topic.Bytes != nil || topic.Reason != ReasonLimit || !report.Truncated {
		t.Errorf("oversized compacted body: %+v", topic)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("oversized compacted body took %v", elapsed)
	}

	// Past the time budget nothing more is measured.
	lim := DefaultLimits
	lim.MaxDuration = time.Nanosecond
	report = measure(t, goldenNamed(t, "routed").materialize(t), lim)
	for _, topic := range report.Topics {
		if topic.Bytes != nil || topic.Reason != ReasonLimit {
			t.Errorf("past the deadline %s: %+v", topic.Name, topic)
		}
	}
	if !report.Truncated {
		t.Error("deadline did not mark the report truncated")
	}
}
