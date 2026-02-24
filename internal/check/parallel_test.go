package check

import (
	"sync/atomic"
	"testing"
)

func TestParallelCheckRepos_Empty(t *testing.T) {
	results := ParallelCheckRepos(nil, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestParallelCheckRepos_CallbackCount(t *testing.T) {
	inputs := []RepoCheckInput{
		{Name: "repo-a", RepoPath: "/nonexistent/a"},
		{Name: "repo-b", RepoPath: "/nonexistent/b"},
		{Name: "repo-c", RepoPath: "/nonexistent/c"},
	}

	var count int64
	results := ParallelCheckRepos(inputs, func() {
		atomic.AddInt64(&count, 1)
	})

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
	if count != 3 {
		t.Errorf("expected onDone called 3 times, got %d", count)
	}
	for i, r := range results {
		if r.Status != "error" {
			t.Errorf("result[%d]: expected status 'error', got %q", i, r.Status)
		}
	}

	// Verify index alignment
	if results[0].Name != "repo-a" {
		t.Errorf("results[0].Name = %q, want %q", results[0].Name, "repo-a")
	}
	if results[1].Name != "repo-b" {
		t.Errorf("results[1].Name = %q, want %q", results[1].Name, "repo-b")
	}
	if results[2].Name != "repo-c" {
		t.Errorf("results[2].Name = %q, want %q", results[2].Name, "repo-c")
	}
}

func TestParallelCheckRepos_NilCallback(t *testing.T) {
	inputs := []RepoCheckInput{{Name: "x", RepoPath: "/nonexistent"}}
	results := ParallelCheckRepos(inputs, nil)
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}
