package plugin

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestPluginHostStateDetachedAccountRetainsReceipt(t *testing.T) {
	var commands []string
	s := fakePiNpm(t, &commands)
	s.Accounts = map[string]Account{"work": {Agent: "pi", Dir: t.TempDir()}}
	applyPluginRequest(t, s, Request{Action: "add", Source: hostStatePiSource(t), Targets: []string{"work"}})
	d := readState(t, s)
	pack := d.intent["demo"]
	delete(pack.Bindings, "work")
	d.intent["demo"] = pack
	if err := s.save(d); err != nil {
		t.Fatal(err)
	}
	s.Accounts = nil
	inv, err := s.Packages()
	if err != nil {
		t.Fatal(err)
	}
	b := inv.Packages["demo"].Bindings["work"]
	if !b.Cleanup || !b.ContextChanged || b.ID == "" {
		t.Fatalf("detached account lost cleanup evidence: %+v", b)
	}
	p, err := s.Preview(context.Background(), Request{Action: "sync"})
	if err != nil {
		t.Fatal(err)
	}
	if !p.Blocked {
		t.Fatal("detached account could execute native cleanup")
	}
	request := Request{Action: "update", All: true}
	p, err = s.Preview(context.Background(), request)
	if err != nil || p.Blocked || len(p.Changes) != 1 || p.Changes[0].Action != "skip" || !p.Changes[0].batchError {
		t.Fatalf("--all did not isolate the detached account: %+v %v", p, err)
	}
	same := unchanged(t, s.ConfigPath, s.statePath())
	if _, err := s.Apply(context.Background(), request, p.Revision); err == nil {
		t.Fatal("--all did not report the detached account failure")
	}
	same()
}

func TestPluginHostStateRebindChangedAccountBlocked(t *testing.T) {
	for _, tc := range []struct{ kind, change string }{
		{"source", "directory"}, {"source", "existing-id"}, {"source", "cli"},
		{"npm", "directory"}, {"npm", "cli"}, {"npm", "filtered"},
		{"import", "directory"}, {"import", "cli"}, {"import", "filtered"},
	} {
		t.Run(tc.kind+"/"+tc.change, func(t *testing.T) {
			var commands []string
			s := fakePiNpm(t, &commands)
			originalAccount := Account{Agent: "pi", Dir: t.TempDir()}
			s.Accounts = map[string]Account{"work": originalAccount}
			r := Request{Action: "add", Source: "npm:demo", Targets: []string{"work"}}
			if tc.kind == "source" || tc.kind == "npm" && tc.change == "filtered" {
				r.Source = hostStatePiSource(t)
			}
			if tc.kind == "import" {
				file, _ := s.piSettingsPath("work")
				writePluginFile(t, filepath.Dir(file), filepath.Base(file), `{"packages":["npm:demo"]}`)
				r = Request{Action: "import", From: "work", Plugin: "npm:demo", Name: "demo"}
			}
			applyPluginRequest(t, s, r)
			if tc.kind == "npm" && tc.change == "filtered" {
				d := readState(t, s)
				pack := d.intent["demo"]
				pack.Bindings["work"] = declaration{Source: "npm:demo"}
				d.intent["demo"] = pack
				if err := s.save(d); err != nil {
					t.Fatal(err)
				}
				r.Source = "npm:demo"
			}
			changed := originalAccount
			if tc.change == "cli" {
				changed.CLI = "pi-fork"
			} else {
				changed.Dir = t.TempDir()
			}
			s.Accounts["work"] = changed
			file, _ := s.piSettingsPath("work")
			if tc.kind == "import" || tc.change == "existing-id" {
				b := hostPackages(t, s)["demo"].Bindings["work"]
				writePluginFile(t, filepath.Dir(file), filepath.Base(file), `{"packages":[`+strconv.Quote(b.ID)+`]}`)
			}
			if tc.change == "filtered" {
				writePluginFile(t, filepath.Dir(file), filepath.Base(file), `{"packages":[{"source":"npm:demo","extensions":["-private.ts"]}]}`)
			}
			same := unchanged(t, s.ConfigPath, s.statePath())
			before := len(commands)
			p, err := s.Preview(context.Background(), r)
			if err != nil {
				t.Fatal(err)
			}
			if !p.Blocked {
				// On dacbba6e this Apply proves the original applied store is overwritten.
				if _, err := s.Apply(context.Background(), r, p.Revision); err != nil {
					t.Fatal(err)
				}
				t.Fatal("changed account add/import was not blocked")
			}
			if !strings.Contains(p.Changes[0].Message, "Native account or scope changed") {
				t.Fatalf("context guard ran after registration preparation: %+v", p.Changes)
			}
			if _, err := s.Apply(context.Background(), r, p.Revision); err == nil {
				t.Fatal("applied blocked account change")
			}
			same()
			if len(commands) != before {
				t.Fatal("blocked account ran native command")
			}
			s.Accounts["work"] = originalAccount
			if tc.change == "filtered" && tc.kind == "npm" {
				applyPluginRequest(t, s, Request{Action: "sync", Name: "demo"})
			} else {
				applyPluginRequest(t, s, r)
			}
		})
	}
}
