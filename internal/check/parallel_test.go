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
}
