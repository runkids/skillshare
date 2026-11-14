package main

import (
	"reflect"
	"strings"
	"testing"

	"skillshare/internal/skillserve"
)

func TestParseMCPServeProfileBindings(t *testing.T) {
	env := map[string]string{"WRITING_TOKEN": "fixture-writing-secret", "CODING_TOKEN": "fixture-coding-secret", "EMPTY": "", "PADDED": "fixture-secret ", "CONTROL": "fixture-secret\r"}
	lookup := func(k string) (string, bool) { v, ok := env[k]; return v, ok }
	for _, tc := range []struct {
		name   string
		values []string
		bad    bool
	}{
		{"valid", []string{"writing=WRITING_TOKEN", "coding=CODING_TOKEN"}, false},
		{"missing-equals", []string{"writing"}, true},
		{"empty-name", []string{"=WRITING_TOKEN"}, true},
		{"bad-profile", []string{"UPPER=WRITING_TOKEN"}, true},
		{"reserved-profile", []string{"none=WRITING_TOKEN"}, true},
		{"empty-env", []string{"writing="}, true},
		{"bad-env", []string{"writing=TOKEN-NAME"}, true},
		{"extra-equals", []string{"writing=TOKEN=VALUE"}, true},
		{"absent-env", []string{"writing=MISSING"}, true},
		{"empty-secret", []string{"writing=EMPTY"}, true},
		{"padded-secret", []string{"writing=PADDED"}, true},
		{"control-secret", []string{"writing=CONTROL"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseMCPServeProfileBindings(tc.values, lookup)
			if (err != nil) != tc.bad {
				t.Fatalf("bindings %v error %v", tc.values, err)
			}
			if err != nil {
				if tc.name == "padded-secret" || tc.name == "control-secret" {
					_, env, _ := strings.Cut(tc.values[0], "=")
					if !strings.Contains(err.Error(), "writing") || !strings.Contains(err.Error(), env) {
						t.Fatalf("missing redacted context: %v", err)
					}
				}
				for _, secret := range []string{"fixture-writing-secret", "fixture-coding-secret", "fixture-secret"} {
					if strings.Contains(err.Error(), secret) {
						t.Fatal("credential in diagnostic")
					}
				}
			} else if !reflect.DeepEqual(got, []skillserve.ProfileBinding{{Profile: "writing", Token: env["WRITING_TOKEN"]}, {Profile: "coding", Token: env["CODING_TOKEN"]}}) {
				t.Fatalf("wrong credential resolution: %v", got)
			}
		})
	}
}

func FuzzMCPServeProfileBindings(f *testing.F) {
	for _, s := range []string{"writing=TOKEN", "writing", "=TOKEN", "writing=TOKEN=secret", "none=TOKEN", "UPPER=TOKEN", "writing=TOKEN-NAME", "_writing=TOKEN", "0team=_TOKEN", strings.Repeat("a", 64) + "=TOKEN", strings.Repeat("a", 65) + "=TOKEN"} {
		f.Add(s)
	}
	// Independent grammar oracle: no production name/env validator is consulted.
	f.Fuzz(func(t *testing.T, s string) {
		const secret = "fixture-credential-never-diagnostic"
		name, env, found := strings.Cut(s, "=")
		valid := found && len(name) >= 1 && len(name) <= 64 && name != "none" && len(env) >= 1
		for i, c := range []byte(name) {
			valid = valid && (c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || i > 0 && (c == '_' || c == '-'))
		}
		for i, c := range []byte(env) {
			valid = valid && (c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' || i > 0 && c >= '0' && c <= '9')
		}
		var requested []string
		bindings, err := parseMCPServeProfileBindings([]string{s}, func(k string) (string, bool) { requested = append(requested, k); return secret, true })
		if (err == nil) != valid {
			t.Fatalf("grammar acceptance for %q: %v", s, err)
		}
		if err != nil {
			if strings.Contains(err.Error(), secret) {
				t.Fatal("credential leaked")
			}
			if len(requested) != 0 {
				t.Fatal("invalid grammar reached environment lookup")
			}
			return
		}
		if !reflect.DeepEqual(requested, []string{env}) || !reflect.DeepEqual(bindings, []skillserve.ProfileBinding{{Profile: name, Token: secret}}) {
			t.Fatal("wrong profile or environment lookup")
		}
	})
}
