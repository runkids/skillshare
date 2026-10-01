package hooks

import (
	"strings"
	"testing"
)

func TestCatalog_CoversEveryCommandAgent(t *testing.T) {
	for _, target := range Targets {
		c, ok := Catalog[target.Name]
		if target.Kind == KindCode {
			if ok {
				t.Fatalf("%s is a code Agent and has no event catalog", target.Name)
			}
			continue
		}
		if target.Kind == KindGit {
			if !ok || len(c.Events) != 28 || c.TimeoutUnit != "" {
				t.Fatalf("git catalog: %+v", c)
			}
			for _, event := range c.Events {
				if !gitEventName.MatchString(event.Name) || event.Description == "" {
					t.Fatalf("git event: %+v", event)
				}
			}
			continue
		}
		if !ok || len(c.Events) == 0 || (c.TimeoutUnit != "seconds" && c.TimeoutUnit != "milliseconds") {
			t.Fatalf("%s catalog: %+v", target.Name, c)
		}
		for _, event := range c.Events {
			if !eventName.MatchString(event.Name) || event.Description == "" {
				t.Fatalf("%s event: %+v", target.Name, event)
			}
		}
	}
}

func TestPlan_UnknownEventWarnsWithoutBlocking(t *testing.T) {
	e := newEnv(t)
	typo := entry(t, `{"bindings":{"claude":{"events":{"Stopp":[{"hooks":[{"type":"command","command":"echo"}]}]}},"codex":{"events":{"Stop":[{"hooks":[{"type":"command","command":"echo"}]}]}},"pi":{"code":"export default function () {}"}}}`)
	p, err := e.service.PreviewMutation(Mutation{Name: "typo", Entry: typo})
	must(t, err)
	if p.Blocked || len(p.Warnings) != 1 || !strings.Contains(p.Warnings[0], `"Stopp"`) || !strings.Contains(p.Warnings[0], "claude") {
		t.Fatalf("warnings: %v blocked=%t", p.Warnings, p.Blocked)
	}
	// Copilot's documented PascalCase aliases are known events.
	if w := EventWarnings("c", Entry{Bindings: map[string]Binding{"copilot": {Events: map[string]any{"PreToolUse": []any{}, "sessionStart": []any{}}}}}); len(w) != 0 {
		t.Fatalf("copilot aliases: %v", w)
	}
}
