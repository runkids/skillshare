package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fingerprint(t *testing.T, s *Service) *Plan {
	t.Helper()
	p, err := s.Preview()
	must(t, err)
	if p.Fingerprint == "" {
		t.Fatal("every plan needs a fingerprint")
	}
	return p
}

func TestFingerprint_DeterministicIncludingEmptyPlans(t *testing.T) {
	e := newEnv(t)
	if fingerprint(t, e.service).Fingerprint != fingerprint(t, e.service).Fingerprint {
		t.Fatal("an empty plan's fingerprint must be deterministic")
	}
	save(t, e.service, Mutation{Name: "guard", Entry: entry(t, claudeEntry)})
	if fingerprint(t, e.service).Fingerprint != fingerprint(t, e.service).Fingerprint {
		t.Fatal("a noop plan's fingerprint must be deterministic")
	}
}

// A pending change edited again keeps the same actions; only the fingerprint shows
// the content the user reviewed is no longer what sync would write.
func TestFingerprint_SameActionsChangedContent(t *testing.T) {
	cases := map[string][2]string{
		"command": {claudeEntry, strings.Replace(claudeEntry, "echo guard", "echo other", 1)},
		"code": {
			`{"bindings":{"opencode":{"code":"export const A = async () => ({})\n"}}}`,
			`{"bindings":{"opencode":{"code":"export const B = async () => ({})\n"}}}`,
		},
		"script": {
			`{"bindings":{"claude":{"events":{"Stop":[{"hooks":[{"type":"command","command":"run.sh"}]}]},"files":{"run.sh":"echo a\n"}}}}`,
			`{"bindings":{"claude":{"events":{"Stop":[{"hooks":[{"type":"command","command":"run.sh"}]}]},"files":{"run.sh":"echo b\n"}}}}`,
		},
	}
	for name, docs := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			_, err := e.service.Mutate(Mutation{Name: "guard", Entry: entry(t, docs[0])}, "", false)
			must(t, err)
			before := fingerprint(t, e.service)
			_, err = e.service.Mutate(Mutation{Name: "guard", Entry: entry(t, docs[1])}, "", false)
			must(t, err)
			after := fingerprint(t, e.service)
			if actions(before) != actions(after) {
				t.Fatalf("fixture must keep the actions: %s vs %s", actions(before), actions(after))
			}
			if before.Fingerprint == after.Fingerprint {
				t.Fatal("changed content must change the fingerprint")
			}
		})
	}
}

func TestFingerprint_TracksHooksSourceNativeAndOwnership(t *testing.T) {
	e := newEnv(t)
	settings := filepath.Join(e.home, ".claude", "settings.json")
	write(t, settings, `{"model":"x","hooks":{}}`)
	save(t, e.service, Mutation{Name: "guard", Entry: entry(t, claudeEntry)})
	base := fingerprint(t, e.service)

	// Unrelated Skillshare configuration changes the revision only.
	config := read(t, e.config)
	write(t, e.config, strings.Replace(config, "targets: {}", "targets:\n  claude:\n    path: /elsewhere", 1))
	p := fingerprint(t, e.service)
	if p.Revision == base.Revision || p.Fingerprint != base.Fingerprint {
		t.Fatalf("unrelated config: revision must change, fingerprint must not")
	}
	// Unrelated settings in the same native file leave it unchanged.
	write(t, settings, strings.Replace(read(t, settings), `"model":"x"`, `"model":"y","mcpServers":{"a":{"command":"a"}}`, 1))
	if fingerprint(t, e.service).Fingerprint != base.Fingerprint {
		t.Fatal("unrelated native settings must not change the fingerprint")
	}

	changed := map[string]func(){
		"disabled": func() {
			disabled := entry(t, claudeEntry)
			off := false
			disabled.Enabled = &off
			_, err := e.service.Mutate(Mutation{Name: "guard", Entry: disabled}, "", false)
			must(t, err)
		},
		"native hooks": func() {
			write(t, settings, strings.Replace(read(t, settings), `"hooks":{`, `"hooks":{"Stop":[{"hooks":[{"type":"command","command":"x"}]}],`, 1))
		},
		"ownership": func() { must(t, os.Remove(e.service.statePath())) },
	}
	for name, change := range changed {
		t.Run(name, func(t *testing.T) {
			restore := map[string]string{e.config: read(t, e.config), settings: read(t, settings), e.service.statePath(): read(t, e.service.statePath())}
			defer func() {
				for path, content := range restore {
					write(t, path, content)
				}
			}()
			change()
			if fingerprint(t, e.service).Fingerprint == base.Fingerprint {
				t.Fatal("a hooks-relevant change must change the fingerprint")
			}
		})
	}
}

func TestFingerprint_TracksReplaceAndDroidGuard(t *testing.T) {
	e := newEnv(t)
	write(t, filepath.Join(e.home, ".claude", "settings.json"), `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo guard","timeout":5}]}]}}`)
	keep, err := e.service.PreviewMutation(Mutation{Name: "guard", Entry: entry(t, claudeEntry)})
	must(t, err)
	take, err := e.service.PreviewMutation(Mutation{Name: "guard", Entry: entry(t, claudeEntry), Replace: true})
	must(t, err)
	if keep.Fingerprint == take.Fingerprint {
		t.Fatal("replacement semantics must change the fingerprint")
	}

	droid := `{"bindings":{"droid":{"events":{"Stop":[{"hooks":[{"type":"command","command":"x"}]}]}}}}`
	_, err = e.service.Mutate(Mutation{Name: "d", Entry: entry(t, droid)}, "", false)
	must(t, err)
	open := fingerprint(t, e.service)
	write(t, filepath.Join(e.home, ".factory", "settings.json"), `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"y"}]}]}}`)
	if fingerprint(t, e.service).Fingerprint == open.Fingerprint {
		t.Fatal("the Droid inline-hooks guard must change the fingerprint")
	}
}
