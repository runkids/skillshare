package main

import "testing"

func TestLooksLikeURLOrPath_SSH(t *testing.T) {
	cases := map[string]bool{
		"git@github.com:owner/repo.git":                 true,
		"git@ghe.corp.com:team/skills.git//hubs/h.json": true,
		"ssh://git@host/org/repo.git":                   true,
		"https://internal.corp/hub.json":                true,
		"./skillshare-hub.json":                         true,
		"team":                                          false,
		"my-hub":                                        false,
	}
	for in, want := range cases {
		if got := looksLikeURLOrPath(in); got != want {
			t.Errorf("looksLikeURLOrPath(%q) = %v, want %v", in, got, want)
		}
	}
}
