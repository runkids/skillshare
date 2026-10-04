package git

import (
	"fmt"
	"skillshare/internal/install"
)

// PullFollowed shares the user-owned update policy with install --update.
func PullFollowed(policy *install.FollowedUpdate, onProgress func(string)) (*UpdateInfo, error) {
	before, err := GetCurrentFullHash(policy.Path)
	if err != nil {
		return nil, fmt.Errorf("%w: %v; resolve in `%s`", install.ErrFollowedUpdate, err, policy.Path)
	}
	if err := policy.Pull(onProgress); err != nil {
		return nil, fmt.Errorf("%w: %v; resolve in `%s`", install.ErrFollowedUpdate, err, policy.Path)
	}
	after, err := GetCurrentFullHash(policy.Path)
	if err != nil {
		return nil, fmt.Errorf("%w: %v; resolve in `%s`", install.ErrFollowedUpdate, err, policy.Path)
	}
	info := &UpdateInfo{BeforeHash: before, AfterHash: after, UpToDate: before == after}
	if !info.UpToDate {
		info.Commits, _ = GetCommitsBetween(policy.Path, before, after)
		info.Stats, _ = GetDiffStats(policy.Path, before, after)
	}
	return info, nil
}
