package hooks

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// adapterEntry is one valid binding per Agent, in that Agent's own native shape.
var adapterEntry = map[string]string{
	"claude":      `{"events":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo claude","timeout":5}]}]}}`,
	"codex":       `{"events":{"PreToolUse":[{"matcher":"shell","hooks":[{"type":"command","command":"echo codex","timeout":5}]}]}}`,
	"gemini":      `{"events":{"BeforeTool":[{"matcher":"run_shell_command","hooks":[{"type":"command","command":"echo gemini","timeout":5000}]}]}}`,
	"qwen":        `{"events":{"PreToolUse":[{"matcher":"Shell","hooks":[{"type":"command","command":"echo qwen"}]}]}}`,
	"droid":       `{"events":{"PreToolUse":[{"matcher":"Execute","hooks":[{"type":"command","command":"echo droid"}]}]}}`,
	"cursor":      `{"events":{"beforeShellExecution":[{"command":"./hooks/skillshare/demo/guard.sh"}]},"files":{"guard.sh":"#!/bin/sh\necho cursor\n"}}`,
	"copilot":     `{"events":{"preToolUse":[{"type":"command","bash":"echo copilot","timeoutSec":10}]}}`,
	"antigravity": `{"events":{"PreToolUse":[{"matcher":"run_command","hooks":[{"type":"command","command":"echo agy","timeout":10}]}],"Stop":[{"command":"echo stop"}]}}`,
	"pi":          `{"code":"export default function (pi) {\n  pi.on(\"tool_call\", async () => {});\n}\n"}`,
	"omp":         `{"code":"export default function (pi) {\n  pi.on(\"tool_call\", async () => {});\n}\n"}`,
	"amp":         `{"code":"export default function (amp) {\n  amp.on(\"tool.call\", async () => ({ action: \"allow\" }));\n}\n"}`,
	"opencode":    `{"code":"export const Demo = async () => ({\n  \"tool.execute.before\": async () => {},\n})\n"}`,
}

func TestAdapters_GlobalPathsAndNativeShape(t *testing.T) {
	want := map[string]string{
		"claude": ".claude/settings.json", "codex": ".codex/hooks.json", "gemini": ".gemini/settings.json",
		"qwen": ".qwen/settings.json", "droid": ".factory/hooks.json", "cursor": ".cursor/hooks.json",
		"copilot": ".copilot/hooks/skillshare-demo.json", "pi": ".pi/agent/extensions/skillshare-demo.ts", "omp": ".omp/agent/extensions/skillshare-demo.ts",
		"amp": ".config/amp/plugins/skillshare-demo.ts", "opencode": ".config/opencode/plugins/skillshare-demo.ts",
		"antigravity": ".gemini/config/hooks.json",
	}
	for target, binding := range adapterEntry {
		t.Run(target, func(t *testing.T) {
			e := newEnv(t)
			save(t, e.service, Mutation{Name: "demo", Entry: entry(t, `{"bindings":{"`+target+`":`+binding+`}}`)})
			path := filepath.Join(e.home, want[target])
			content := read(t, path)
			var b Binding
			must(t, json.Unmarshal([]byte(binding), &b))
			switch target {
			case "pi", "omp", "amp", "opencode":
				if content != b.Code {
					t.Fatalf("code must be written verbatim, got %q", content)
				}
			case "antigravity":
				var doc map[string]map[string]any
				must(t, json.Unmarshal([]byte(content), &doc))
				if len(doc) != 1 || doc["demo"]["PreToolUse"] == nil || doc["demo"]["Stop"] == nil {
					t.Fatalf("antigravity writes one block named after the hook: %s", content)
				}
			case "copilot":
				var doc map[string]any
				must(t, json.Unmarshal([]byte(content), &doc))
				if doc["version"] != float64(1) || doc["hooks"].(map[string]any)["preToolUse"] == nil {
					t.Fatalf("copilot file: %s", content)
				}
			default:
				got := events(t, path, target != "droid")
				for event := range b.Events {
					if len(got[event]) != 1 {
						t.Fatalf("%s missing %s: %s", path, event, content)
					}
				}
				if target == "cursor" && !strings.Contains(content, `"version": 1`) {
					t.Fatalf("cursor hooks.json needs version 1: %s", content)
				}
			}
			if target == "cursor" {
				script := filepath.Join(e.home, ".cursor", "hooks", "skillshare", "demo", "guard.sh")
				if read(t, script) != "#!/bin/sh\necho cursor\n" {
					t.Fatal("script file not written")
				}
			}
			p, err := e.service.Preview()
			must(t, err)
			for _, c := range p.Changes {
				if c.Action != "unchanged" {
					t.Fatalf("second sync must be idempotent: %s", actions(p))
				}
			}
		})
	}
}

func TestAdapters_ProjectPathsNeverFallBackToGlobal(t *testing.T) {
	want := map[string]string{
		"claude": ".claude/settings.json", "codex": ".codex/hooks.json", "gemini": ".gemini/settings.json",
		"qwen": ".qwen/settings.json", "droid": ".factory/hooks.json", "cursor": ".cursor/hooks.json",
		"copilot": ".github/hooks/skillshare-demo.json", "pi": ".pi/extensions/skillshare-demo.ts", "omp": ".omp/extensions/skillshare-demo.ts",
		"amp": ".amp/plugins/skillshare-demo.ts", "opencode": ".opencode/plugins/skillshare-demo.ts",
		"antigravity": ".agents/hooks.json",
	}
	e := newEnv(t)
	root := filepath.Join(filepath.Dir(e.home), "repo")
	project := e.project(root)
	project.ConfigDirs = map[string]string{"claude": filepath.Join(e.home, "elsewhere"), "xdg": filepath.Join(e.home, "xdg")}
	var bindings []string
	for target, binding := range adapterEntry {
		bindings = append(bindings, `"`+target+`":`+binding)
	}
	save(t, project, Mutation{Name: "demo", Entry: entry(t, `{"bindings":{`+strings.Join(bindings, ",")+`}}`)})
	for target, rel := range want {
		if !exists(filepath.Join(root, rel)) {
			t.Errorf("%s: %s not written", target, rel)
		}
	}
	if exists(filepath.Join(e.home, ".claude")) || exists(filepath.Join(e.home, "elsewhere")) {
		t.Fatal("project sync wrote a global path")
	}
}

func TestAdapters_ConfigDirOverrides(t *testing.T) {
	e := newEnv(t)
	dir := filepath.Join(e.home, "custom-claude")
	e.service.ConfigDirs = map[string]string{"claude": dir, "pi": filepath.Join(e.home, "pi-home"), "xdg": filepath.Join(e.home, "xdg")}
	paths := e.service.Paths()
	if paths["claude"] != filepath.Join(dir, "settings.json") || paths["pi"] != filepath.Join(e.home, "pi-home", "extensions") || paths["opencode"] != filepath.Join(e.home, "xdg", "opencode", "plugins") {
		t.Fatalf("overrides ignored: %v", paths)
	}
}

func TestParseEntry_ValidatesNativeShape(t *testing.T) {
	bad := map[string]string{
		"invalid agent key":    `{"bindings":{"kiro space":{"events":{}}}}`,
		"code on command":      `{"bindings":{"claude":{"code":"x"}}}`,
		"events on code":       `{"bindings":{"pi":{"events":{"x":[]}}}}`,
		"missing hooks array":  `{"bindings":{"claude":{"events":{"PreToolUse":[{"matcher":"Bash"}]}}}}`,
		"cursor pascal case":   `{"bindings":{"cursor":{"events":{"BeforeShellExecution":[{"command":"x"}]}}}}`,
		"copilot no command":   `{"bindings":{"copilot":{"events":{"preToolUse":[{"type":"command"}]}}}}`,
		"traversal file":       `{"bindings":{"claude":{"events":{"Stop":[{"hooks":[{"type":"command","command":"x"}]}]},"files":{"../x.sh":"x"}}}}`,
		"nested file":          `{"bindings":{"claude":{"events":{"Stop":[{"hooks":[{"type":"command","command":"x"}]}]},"files":{"a/x.sh":"x"}}}}`,
		"unknown field":        `{"bindings":{},"runtime":"node"}`,
		"negative timeout":     `{"bindings":{"claude":{"events":{"Stop":[{"hooks":[{"type":"command","command":"x","timeout":-1}]}]}}}}`,
		"alias and canonical":  `{"bindings":{"droid":` + adapterEntry["droid"] + `,"factory":` + adapterEntry["droid"] + `}}`,
		"empty event array":    `{"bindings":{"claude":{"events":{"Stop":[]}}}}`,
		"code agent no code":   `{"bindings":{"amp":{}}}`,
		"yaml unknown binding": "bindings:\n  claude:\n    command: echo\n",
	}
	for name, doc := range bad {
		if _, err := ParseEntry([]byte(doc)); err == nil {
			t.Errorf("%s: accepted %s", name, doc)
		}
	}
	e, err := ParseEntry([]byte("bindings:\n  factory:\n    events:\n      Stop:\n        - hooks:\n            - type: command\n              command: echo hi\n              timeout: 5\n"))
	must(t, err)
	if _, ok := e.Bindings["droid"]; !ok {
		t.Fatal("factory alias must resolve to droid")
	}
	if e.Bindings["droid"].Events["Stop"].([]any)[0].(map[string]any)["hooks"].([]any)[0].(map[string]any)["timeout"] != float64(5) {
		t.Fatal("YAML events must take their JSON shape")
	}
}

func TestCopilotFileChangesNameTheirEvents(t *testing.T) {
	e := newEnv(t)
	start := `{"bindings":{"copilot":{"events":{"sessionStart":[{"type":"command","bash":"echo a"}]}}}}`
	p, err := e.service.PreviewMutation(Mutation{Name: "demo", Entry: entry(t, start)})
	must(t, err)
	if len(p.Changes) != 1 || p.Changes[0].Events == nil || !slices.Equal(p.Changes[0].Events.Added, []string{"sessionStart"}) {
		t.Fatalf("add: %+v", p.Changes)
	}
	save(t, e.service, Mutation{Name: "demo", Entry: entry(t, start)})
	next := `{"bindings":{"copilot":{"events":{"sessionStart":[{"type":"command","bash":"echo b"}],"preToolUse":[{"type":"command","bash":"echo c"}]}}}}`
	p, err = e.service.PreviewMutation(Mutation{Name: "demo", Entry: entry(t, next)})
	must(t, err)
	if ev := p.Changes[0].Events; ev == nil || !slices.Equal(ev.Added, []string{"preToolUse"}) || !slices.Equal(ev.Updated, []string{"sessionStart"}) {
		t.Fatalf("update: %+v", p.Changes[0].Events)
	}
	p, err = e.service.PreviewMutation(Mutation{Name: "demo", Remove: true})
	must(t, err)
	if ev := p.Changes[0].Events; p.Changes[0].Action != "remove" || ev == nil || !slices.Equal(ev.Removed, []string{"sessionStart"}) {
		t.Fatalf("remove: %+v", p.Changes[0])
	}
}
