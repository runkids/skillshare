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

func TestParallelCheckURLs_Empty(t *testing.T) {
	results := ParallelCheckURLs(nil, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestParallelCheckURLs_CallbackCount(t *testing.T) {
	inputs := []URLCheckInput{
		{RepoURL: "https://invalid.example.com/a.git"},
		{RepoURL: "https://invalid.example.com/b.git"},
	}

	var count int64
	results := ParallelCheckURLs(inputs, func() {
		atomic.AddInt64(&count, 1)
	})

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	if count != 2 {
		t.Errorf("expected onDone called 2 times, got %d", count)
	}
	for i, r := range results {
		if r.Err == nil {
			t.Errorf("result[%d]: expected error, got nil", i)
		}
	}
}

func TestParallelCheckURLs_IndexAlignment(t *testing.T) {
	inputs := []URLCheckInput{
		{RepoURL: "https://invalid.example.com/first.git"},
		{RepoURL: "https://invalid.example.com/second.git"},
	}

	results := ParallelCheckURLs(inputs, nil)
	if results[0].RepoURL != inputs[0].RepoURL {
		t.Errorf("results[0].RepoURL = %q, want %q", results[0].RepoURL, inputs[0].RepoURL)
	}
	if results[1].RepoURL != inputs[1].RepoURL {
		t.Errorf("results[1].RepoURL = %q, want %q", results[1].RepoURL, inputs[1].RepoURL)
	}
}
