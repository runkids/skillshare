package hooks

import (
	"path/filepath"
	"strings"
	"testing"
)

const trialSettings = `{"model":"opus","hooks":{"Stop":[{"hooks":[{"type":"command","command":"afplay /System/Library/Sounds/Glass.aiff","timeout":5}]}],"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"~/.claude/guard.sh"}]}]}}`

func importOne(t *testing.T, s *Service, from, name string) Candidate {
	t.Helper()
	c, err := s.Import(ImportRequest{From: from, Name: name})
	must(t, err)
	if len(c) != 1 {
		t.Fatalf("import %s: %+v", name, c)
	}
	return c[0]
}

func TestImport_CandidateNameTakesOnlyItsEvent(t *testing.T) {
	e := newEnv(t)
	write(t, filepath.Join(e.home, ".claude", "settings.json"), trialSettings)
	c := importOne(t, e.service, "claude", "claude-stop")
	if events := c.Entry.Bindings["claude"].Events; len(events) != 1 || events["Stop"] == nil {
		t.Fatalf("claude-stop must hold only Stop: %v", events)
	}
}

func TestImport_SavingTakesOverWithoutReplace(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(e.home, ".claude", "settings.json")
	write(t, path, trialSettings)
	c := importOne(t, e.service, "claude", "claude-stop")
	for _, w := range c.Warnings {
		if strings.Contains(w, "replace") {
			t.Fatalf("import must not ask for replace: %q", w)
		}
	}
	m := Mutation{Name: c.Name, Entry: &c.Entry, Adopt: true}
	p, err := e.service.PreviewMutation(m)
	must(t, err)
	if p.Blocked || actions(p) != "claude:claude-stop:adopt" {
		t.Fatalf("import preview: %s", actions(p))
	}
	// Saved without sync, the takeover is still recorded.
	if _, err := e.service.Mutate(m, p.Revision, false); err != nil {
		t.Fatal(err)
	}
	p, err = e.service.Preview()
	must(t, err)
	if p.Blocked || actions(p) != "claude:claude-stop:adopt" {
		t.Fatalf("sync after import: %s", actions(p))
	}
	r := sync(t, e.service)
	if len(r.Applied) != 0 || read(t, path) != trialSettings {
		t.Fatalf("taking over must not rewrite the file: %v\n%s", r.Applied, read(t, path))
	}
	if p, _ = e.service.Preview(); actions(p) != "claude:claude-stop:unchanged" {
		t.Fatalf("after the adopting sync: %s", actions(p))
	}
	if rest, _ := e.service.Import(ImportRequest{From: "claude"}); len(rest) != 1 || rest[0].Name != "claude-pretooluse" {
		t.Fatalf("PreToolUse stays unmanaged: %+v", rest)
	}
	write(t, path, strings.Replace(trialSettings, `"timeout":5`, `"timeout":9`, 1))
	if p, _ = e.service.Preview(); !p.Blocked {
		t.Fatalf("an outside edit after import is a conflict: %s", actions(p))
	}
}

func TestImport_AdoptNeverClaimsOtherConfigsOrPlainSaves(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(e.home, ".claude", "settings.json")
	write(t, path, trialSettings)
	c := importOne(t, e.service, "claude", "claude-stop")
	p, err := e.service.PreviewMutation(Mutation{Name: "copy", Entry: &c.Entry})
	must(t, err)
	if !p.Blocked {
		t.Fatalf("a plain add of an identical hook is still a conflict: %s", actions(p))
	}
}

func TestPlan_NarrowedEntryIsAnUpdateWithEventDetail(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(e.home, ".claude", "settings.json")
	write(t, path, `{"model":"opus"}`)
	both := entry(t, `{"bindings":{"claude":{"events":{"Stop":[{"hooks":[{"type":"command","command":"echo stop"}]}],"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo guard"}]}]}}}}`)
	p, err := e.service.PreviewMutation(Mutation{Name: "trial", Entry: both})
	must(t, err)
	if c := p.Changes[0]; c.Action != "add" || c.Events == nil || strings.Join(c.Events.Added, ",") != "PreToolUse,Stop" {
		t.Fatalf("add detail: %+v %+v", c, c.Events)
	}
	save(t, e.service, Mutation{Name: "trial", Entry: both})

	stop := entry(t, `{"bindings":{"claude":{"events":{"Stop":[{"hooks":[{"type":"command","command":"echo stop2"}]}]}}}}`)
	p, err = e.service.PreviewMutation(Mutation{Name: "trial", Entry: stop})
	must(t, err)
	c := p.Changes[0]
	if c.Action != "update" || c.Events == nil || strings.Join(c.Events.Removed, ",") != "PreToolUse" || strings.Join(c.Events.Updated, ",") != "Stop" || len(c.Events.Added) != 0 {
		t.Fatalf("narrowed entry: %+v %+v", c, c.Events)
	}

	narrowed := entry(t, `{"bindings":{"claude":{"events":{"Stop":[{"hooks":[{"type":"command","command":"echo stop"}]}]}}}}`)
	p, err = e.service.PreviewMutation(Mutation{Name: "trial", Entry: narrowed})
	must(t, err)
	if c := p.Changes[0]; c.Action != "update" || strings.Join(c.Events.Removed, ",") != "PreToolUse" || len(c.Events.Updated) != 0 {
		t.Fatalf("an entry that keeps Stop in the file is an update: %+v %+v", c, c.Events)
	}

	p, err = e.service.PreviewMutation(Mutation{Name: "trial", Remove: true})
	must(t, err)
	if c := p.Changes[0]; c.Action != "remove" || strings.Join(c.Events.Removed, ",") != "PreToolUse,Stop" {
		t.Fatalf("leaving the file entirely is a remove: %+v %+v", c, c.Events)
	}
}

func TestReplace_OwnedRegistrationEditedOutsideIsReplacedInPlace(t *testing.T) {
	for _, viaSync := range []bool{false, true} {
		e := newEnv(t)
		path := filepath.Join(e.home, ".claude", "settings.json")
		write(t, path, `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"say done"}]}]}}`)
		c := importOne(t, e.service, "claude", "claude-stop")
		save(t, e.service, Mutation{Name: c.Name, Entry: &c.Entry, Adopt: true})
		write(t, path, strings.Replace(read(t, path), "say done", "say finished", 1))
		if p, _ := e.service.Preview(); !p.Blocked {
			t.Fatalf("an outside edit is a conflict: %s", actions(p))
		}
		m := Mutation{Name: c.Name, Entry: &c.Entry, Replace: true}
		if viaSync {
			m = Mutation{Name: c.Name, Replace: true}
		}
		p, err := e.service.PreviewMutation(m)
		must(t, err)
		files := p.Files()
		save(t, e.service, m)
		got := events(t, path, true)["Stop"]
		if len(got) != 1 || !strings.Contains(read(t, path), "say done") {
			t.Fatalf("replace must take the edited group over in place (sync=%t):\n%s", viaSync, read(t, path))
		}
		if len(files) != 1 || files[0].After != read(t, path) {
			t.Fatalf("preview after must match the written file: %+v", files)
		}
		if p, _ := e.service.Preview(); actions(p) != "claude:claude-stop:unchanged" {
			t.Fatalf("after replace: %s", actions(p))
		}
	}
}
