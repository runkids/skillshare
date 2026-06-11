package main

import (
	"fmt"
	"os"
)

func missingTrackedRepoMessage(name string) string {
	return fmt.Sprintf("tracked repository clone is missing: %s (metadata exists but directory is absent). Run 'skillshare install' to rehydrate tracked repositories.", name)
}

func updateTrackedRepoErrorMessage(target updateTarget, err error) string {
	if _, statErr := os.Stat(target.path); os.IsNotExist(statErr) {
		return missingTrackedRepoMessage(target.name)
	}
	return err.Error()
}
