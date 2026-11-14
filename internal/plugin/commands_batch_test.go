package plugin

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestPluginCommandsBatch(t *testing.T) {
	for _, scope := range []string{"commands", "native", "moved", "empty"} {
		t.Run(scope, func(t *testing.T) {
			s, root, bin := commandFixture(t)
			applyPluginRequest(t, s, Request{Action: "add", Source: root, Targets: []string{"cursor"}})
			writePluginFile(t, root, "bin/one", "#!/bin/sh\nprintf checked\n")
			p, err := s.Preview(context.Background(), Request{Action: "check", Name: "demo"})
			if err != nil {
				t.Fatal(err)
			}
			seen := map[string]Seen{}
			for _, c := range p.Changes {
				seen[c.Target] = checkedObservation(t, c.Binding)
			}
			item := UpdateItem{Name: "demo", Commands: true, Targets: []string{}, Seen: seen}
			if scope == "native" {
				item.Commands = false
				item.Targets = []string{"cursor"}
			}
			if scope == "empty" {
				item.Commands = false
			}
			if scope == "moved" {
				writePluginFile(t, root, "bin/one", "#!/bin/sh\nprintf unchecked\n")
			}
			inventory, err := s.Packages()
			if err != nil {
				t.Fatal(err)
			}
			nativeBefore := inventory.Packages["demo"].Bindings["cursor"].Digest
			_, result, err := s.updateLocked(context.Background(), item, func(*Plan) {})
			if scope == "moved" {
				if !errors.Is(err, ErrSourceMoved) || result != nil {
					t.Fatalf("unchecked source applied: %+v %v", result, err)
				}
				commandOutput(t, bin, "demo-one", "old-one")
			} else if scope == "empty" {
				if err == nil {
					t.Fatal("empty selection expanded to package")
				}
			} else if err != nil {
				t.Fatal(err)
			}
			inventory, err = s.Packages()
			if err != nil {
				t.Fatal(err)
			}
			if scope != "native" && inventory.Packages["demo"].Bindings["cursor"].Digest != nativeBefore {
				t.Fatal("command-only selection mutated native binding")
			}
			if scope == "commands" {
				commandOutput(t, bin, "demo-one", "checked")
			}
			if scope == "native" {
				if inventory.Packages["demo"].Bindings["cursor"].Digest != seen["cursor"].Digest || result == nil || !slices.ContainsFunc(result.Results, func(o Outcome) bool { return o.Target == "cursor" && !o.Commands && o.Status == "installed" }) {
					t.Fatalf("selected native update did not complete: %+v", result)
				}
				commandOutput(t, bin, "demo-one", "old-one")
			}
			if _, err := os.Lstat(filepath.Join(bin, "demo-two")); err != nil {
				t.Fatal(err)
			}
		})
	}
}
