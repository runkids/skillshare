package main

import (
	"reflect"
	"strings"
	"testing"

	"skillshare/internal/skillserve"
)

func TestParseMCPServeProfileBindings(t *testing.T) {
	env := map[string]string{"WRITING_TOKEN": "fixture-writing-secret", "CODING_TOKEN": "fixture-coding-secret", "DUP": "fixture-writing-secret", "EMPTY": ""}
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
		{"duplicate-profile", []string{"writing=WRITING_TOKEN", "writing=CODING_TOKEN"}, true},
		{"absent-env", []string{"writing=MISSING"}, true},
		{"empty-secret", []string{"writing=EMPTY"}, true},
		{"duplicate-secret", []string{"writing=WRITING_TOKEN", "coding=DUP"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseMCPServeProfileBindings(tc.values, lookup)
			if (err != nil) != tc.bad {
				t.Fatalf("bindings %v error %v", tc.values, err)
			}
			if err != nil {
				for _, secret := range []string{"fixture-writing-secret", "fixture-coding-secret"} {
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
	for _, s := range []string{"writing=TOKEN", "writing", "=TOKEN", "writing=TOKEN=secret", "none=TOKEN"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		const secret = "fixture-credential-never-diagnostic"
		bindings, err := parseMCPServeProfileBindings([]string{s}, func(string) (string, bool) { return secret, true })
		if err != nil {
			if strings.Contains(err.Error(), secret) {
				t.Fatal("credential leaked")
			}
			return
		}
		if len(bindings) != 1 || bindings[0].Token != secret || bindings[0].Profile == "" {
			t.Fatal("invalid binding accepted")
		}
		// One credential cannot authorize two profiles, even via different env names.
		if _, err := parseMCPServeProfileBindings([]string{s, "another=SECOND"}, func(string) (string, bool) { return secret, true }); err == nil {
			t.Fatal("duplicate credential accepted")
		}
	})
}
