package main

import (
	"fmt"
	"os"
)

// missingTrackedRepoShortReason is the concise per-item reason used in summarized
// output (JSON items, UI). The full rehydration hint is surfaced once via
// missingTrackedRepoHint instead of being repeated on every item.
const missingTrackedRepoShortReason = "clone directory absent"

// missingTrackedRepoHint is the one-shot recovery instruction for missing
// tracked repos, shown alongside the summarized list.
func missingTrackedRepoHint() string {
	return "Run 'skillshare install' to rehydrate tracked repositories"
}

func missingTrackedRepoMessage(name string) string {
	return fmt.Sprintf("tracked repository clone is missing: %s (metadata exists but directory is absent). %s.", name, missingTrackedRepoHint())
}

// missingTrackedRepoResult returns a skipped updateResult for a tracked repo
// whose clone directory is absent on disk, and true. If the directory exists it
// returns the zero value and false. Shared by the global and project single-target
// update paths so both report missing repos consistently (issue #212).
func missingTrackedRepoResult(t updateTarget) (updateResult, bool) {
	if _, err := os.Stat(t.path); os.IsNotExist(err) {
		return updateResult{
			skipped:             1,
			items:               []updateJSONItem{{Name: t.name, Type: "repo", Status: "skipped", Error: missingTrackedRepoShortReason}},
			missingTrackedRepos: []string{t.name},
		}, true
	}
	return updateResult{}, false
}

func updateTrackedRepoErrorMessage(target updateTarget, err error) string {
	if _, statErr := os.Stat(target.path); os.IsNotExist(statErr) {
		return missingTrackedRepoMessage(target.name)
	}
	return err.Error()
}
