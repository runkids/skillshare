package plugin

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func FuzzPluginHostStateRefProjection(f *testing.F) {
	f.Setenv("HOME", f.TempDir())
	f.Setenv("PI_CODING_AGENT_DIR", f.TempDir())
	f.Add("main", "next", "native-version", uint8(0))
	f.Add("", "", "", uint8(1))
	f.Add("next", "", "pending", uint8(2))
	s := &Service{ConfigPath: filepath.Join(f.TempDir(), "config.yaml"), StateDir: f.TempDir()}
	f.Fuzz(func(t *testing.T, appliedRef, desiredRef, metadata string, mode uint8) {
		selected := mode%3 != 1
		var sync *bool
		if mode%3 != 0 {
			sync = &selected
		}
		source := "https://fixture.invalid/source.git"
		decl := declaration{Source: source, SourceRef: desiredRef, Plugin: "demo", Sync: sync}
		b := Binding{Source: source, SourceRef: appliedRef, Plugin: "demo", Version: metadata, Digest: hash([]byte(metadata)), Commit: hash([]byte(metadata))[:40]}
		b.ID = filepath.Join(s.snapshotPath(b, "pi"), "content")
		switch mode % 5 {
		case 1:
			decl.Source += "-replacement"
		case 2:
			decl = declaration{Imported: true, Sync: sync}
		case 3:
			decl.Entry = "nested"
		case 4:
			decl.Plugin = "other"
		}
		d := &document{intent: map[string]packageDeclaration{"demo": {Bindings: map[string]declaration{"pi": decl}}}, state: localState{Version: 1, Bindings: map[string]map[string]localBinding{"demo": {"pi": {Applied: &receipt{Intent: declareBinding(b).fingerprint(), Store: s.storeIdentity("pi"), Binding: b}}}}}}
		s.compose(d)
		got := d.packages["demo"].Bindings["pi"]
		raw, _ := json.Marshal(got)
		var view struct {
			RefChanged bool `json:"refChanged"`
		}
		if err := json.Unmarshal(raw, &view); err != nil {
			t.Fatal(err)
		}
		want := selected && mode%5 == 0 && appliedRef != desiredRef
		if view.RefChanged != want || got.ID != b.ID || got.SourceRef != appliedRef {
			t.Fatalf("projection violated applied identity / desired ref: drift=%v want=%v", view.RefChanged, want)
		}
	})
}

// Local metadata can be arbitrary JSON strings. Neither saving it nor projecting
// it through Packages may rewrite shared bytes; an explicit save retains intent.

func FuzzPluginHostStateSharedIntentIsolation(f *testing.F) {
	f.Setenv("HOME", f.TempDir())
	f.Setenv("PI_CODING_AGENT_DIR", f.TempDir())
	f.Add("branch", "native-path", uint8(0))
	f.Add("", "\x00pending\n", uint8(1))
	f.Fuzz(func(t *testing.T, ref, metadata string, selection uint8) {
		// Bound materialized strings and files, without excluding any input byte class.
		if len(ref) > 4096 {
			ref = ref[:4096]
		}
		if len(metadata) > 4096 {
			metadata = metadata[:4096]
		}
		selected := selection%3 != 1
		decl := declaration{Source: "https://fixture.invalid/source.git", SourceRef: ref, Plugin: "demo"}
		if selection%3 != 0 {
			decl.Sync = &selected
		}
		root := t.TempDir()
		s := &Service{ConfigPath: filepath.Join(root, "config.yaml"), StateDir: filepath.Join(root, "state")}
		intent := map[string]packageDeclaration{"demo": {Bindings: map[string]declaration{"pi": decl}}}
		before, err := yaml.Marshal(intent)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := yaml.Marshal(map[string]any{"unrelated": "kept", "plugins": map[string]any{"version": 2, "packages": intent}})
		if err != nil {
			t.Fatal(err)
		}
		writePluginFile(t, root, "config.yaml", string(raw))
		d := readState(t, s)
		b := Binding{Source: decl.Source, SourceRef: ref, Plugin: "demo", Version: metadata, Digest: hash([]byte(metadata)), Commit: hash([]byte(metadata))[:40]}
		b.ID = filepath.Join(s.snapshotPath(b, "pi"), "content")
		r := &receipt{Intent: declareBinding(b).fingerprint(), Store: s.storeIdentity("pi"), Binding: b}
		local := localBinding{Applied: r}
		switch selection / 3 % 3 {
		case 1:
			r.Action = "install"
			local = localBinding{Pending: r}
		case 2:
			local = localBinding{Legacy: r}
		}
		d.putLocal("demo", "pi", local)
		same := unchanged(t, s.ConfigPath)
		if err := s.saveState(d); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Packages(); err != nil {
			t.Fatal(err)
		}
		same()
		// Exercise the declaration writer with composed runtime metadata present.
		d = readState(t, s)
		if err := s.save(d); err != nil {
			t.Fatal(err)
		}
		after, err := yaml.Marshal(readState(t, s).intent)
		if err != nil {
			t.Fatal(err)
		}
		if string(before) != string(after) {
			t.Fatal("local metadata changed shared declaration roundtrip")
		}
	})
}
