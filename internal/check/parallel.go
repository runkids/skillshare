package check

import (
	"sync"

	"skillshare/internal/git"
)

const maxWorkers = 8

// RepoCheckInput describes a tracked repo to check.
type RepoCheckInput struct {
	Name     string
	RepoPath string
}

// RepoCheckOutput holds the result of checking a single repo.
type RepoCheckOutput struct {
	Name    string
	Status  string // "up_to_date", "behind", "dirty", "error"
	Behind  int
	Message string
}

// URLCheckInput describes a remote URL to probe.
type URLCheckInput struct {
	RepoURL string
}

// URLCheckOutput holds the result of probing a remote URL.
type URLCheckOutput struct {
	RepoURL    string
	RemoteHash string
	Err        error
}

// ParallelCheckRepos checks multiple repos concurrently using a bounded
// semaphore of maxWorkers goroutines. onDone (if non-nil) is called after
// each repo finishes, useful for progress updates.
func ParallelCheckRepos(repos []RepoCheckInput, onDone func()) []RepoCheckOutput {
	outputs := make([]RepoCheckOutput, len(repos))
	if len(repos) == 0 {
		return outputs
	}

	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	for i, repo := range repos {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, r RepoCheckInput) {
			defer wg.Done()
			defer func() { <-sem }()

			outputs[idx] = checkOneRepo(r)
			if onDone != nil {
				onDone()
			}
		}(i, repo)
	}
	wg.Wait()

	return outputs
}

func checkOneRepo(r RepoCheckInput) RepoCheckOutput {
	out := RepoCheckOutput{Name: r.Name}

	isDirty, err := git.IsDirty(r.RepoPath)
	if err != nil {
		out.Status = "error"
		out.Message = err.Error()
		return out
	}
	if isDirty {
		out.Status = "dirty"
		out.Message = "has uncommitted changes"
		return out
	}

	behind, err := git.GetBehindCountWithAuth(r.RepoPath)
	if err != nil {
		out.Status = "error"
		out.Message = err.Error()
		return out
	}

	if behind == 0 {
		out.Status = "up_to_date"
	} else {
		out.Status = "behind"
		out.Behind = behind
	}
	return out
}
