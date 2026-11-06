package main

import (
	"errors"

	"skillshare/internal/projects"
)

// projectConfirmation words the question a remove or share asks; an empty
// question means the change needs no confirmation.
func projectConfirmation(o projectOptions, service *projects.Service, root string) (string, error) {
	item, err := service.Show(root)
	if err != nil {
		return "", err
	}
	if o.action == "share" {
		_, err := service.Share(root, projects.ShareOptions{DryRun: true})
		if errors.Is(err, projects.ErrSharedDiffers) {
			return "config.yaml has different settings for " + root + ". Replace them with this machine's?", nil
		}
		return "", nil
	}
	switch {
	case item.SharedCopy && !o.shared:
		return "Remove this machine's copy of " + root + "? Its config.yaml settings apply again.", nil
	case item.Origin == projects.OriginShared || item.SharedCopy:
		return "Remove project " + root + " from config.yaml? Every machine that uses it loses the project.", nil
	}
	return "Remove project " + root + "?", nil
}

func projectStorageLabel(origin string) string {
	if origin == projects.OriginLocal {
		return "on this machine only"
	}
	return "in config.yaml"
}
