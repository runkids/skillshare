package mcp

import (
	"os"
	"strings"
	"testing"
)

func TestBatchMutationPreflightAndRevision(t *testing.T) {
	s := testService(t)
	if err := os.WriteFile(s.ConfigPath, []byte("mcp: {servers: {}}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	one := Server{Command: "echo", Targets: []string{"claude"}}
	two := Server{URL: "https://example.com/mcp", Targets: []string{"codex"}}
	batch := []Mutation{{Name: "one", Server: &one}, {Name: "two", Server: &two}}
	bad := Server{Command: "echo", URL: "https://example.com/mcp", Targets: []string{"claude"}}
	if _, err := s.MutateBatch([]Mutation{batch[0], {Name: "bad", Server: &bad}}, "", true); err == nil {
		t.Fatal("invalid batch accepted")
	}
	data, _ := os.ReadFile(s.ConfigPath)
	if strings.Contains(string(data), "one") {
		t.Fatal("part of invalid batch was saved")
	}
	p, err := s.PreviewMutations(batch)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.ConfigPath, append(data, []byte("# concurrent edit\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MutateBatch(batch, p.Revision, true); err == nil {
		t.Fatal("stale preview accepted")
	}
	p, err = s.PreviewMutations(batch)
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.MutateBatch(batch, p.Revision, true)
	if err != nil || len(r.Applied) != 2 {
		t.Fatalf("batch failed: %+v %v", r, err)
	}
	p, err = s.Preview()
	if err != nil || p.Blocked || len(p.Changes) != 2 {
		t.Fatalf("preview: %+v %v", p, err)
	}
	for _, c := range p.Changes {
		if c.Action != "unchanged" {
			t.Fatalf("not synchronized: %+v", c)
		}
	}
}
