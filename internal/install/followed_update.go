package install

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"

	"skillshare/internal/sourcewalk"
)

// ErrFollowedUpdate marks an item refusal that batch callers must report as a failure.
var ErrFollowedUpdate = errors.New("followed repository update refused")

// FollowedUpdate is the shared update policy for user-owned repositories.
// Audit rollback still resets to the pre-pull commit; callers must keep that gate.
type FollowedUpdate struct{ Path string }

// PrepareFollowedUpdate validates ownership and cleanliness before any mutation
// or dry-run reporting. A nil policy means the ordinary installed-repo flow.
func PrepareFollowedUpdate(sourceDir, repoPath string, follow *sourcewalk.FollowSet, force bool) (*FollowedUpdate, error) {
	if follow == nil {
		return nil, nil
	}
	if err := followDiscoveryError(follow); err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(sourceDir, repoPath)
	if err != nil {
		return nil, err
	}
	entry, followed := follow.InFollowed(filepath.ToSlash(rel))
	if !followed {
		return nil, nil
	}
	policy := &FollowedUpdate{Path: entry.ResolvedTarget}
	if entry.State != sourcewalk.Followed {
		return policy, fmt.Errorf("%w: followed entry %s is %s: %s", ErrFollowedUpdate, entry.Name, entry.State, entry.Reason)
	}
	resolved, resolveErr := sourcewalk.Canonicalize(repoPath)
	if resolveErr != nil {
		return policy, fmt.Errorf("%w: cannot resolve followed repository: %v", ErrFollowedUpdate, resolveErr)
	}
	policy.Path = resolved
	if force {
		return policy, fmt.Errorf("%w: --force is not allowed for followed repository %s; resolve in `%s`", ErrFollowedUpdate, entry.Name, policy.Path)
	}
	if err := policy.CheckClean(); err != nil {
		return policy, err
	}
	return policy, nil
}

// RefuseFollowedSkillUpdate refuses a regular skill below a followed entry
// before any fetch, audit, or write. Only the followed repository itself is
// updated, through PrepareFollowedUpdate.
func RefuseFollowedSkillUpdate(skillRel string, follow *sourcewalk.FollowSet) error {
	if follow == nil {
		return nil
	}
	if err := followDiscoveryError(follow); err != nil {
		return err
	}
	entry, followed := follow.InFollowed(filepath.ToSlash(skillRel))
	if !followed {
		return nil
	}
	return fmt.Errorf("%w: skill %s is inside followed entry %s; skillshare does not reinstall skills in a followed tree", ErrFollowedUpdate, filepath.ToSlash(skillRel), entry.Name)
}

// followDiscoveryError fails closed: an incomplete snapshot cannot prove that a
// path is outside every followed entry.
func followDiscoveryError(follow *sourcewalk.FollowSet) error {
	if err := follow.Err(); err != nil {
		return fmt.Errorf("%w: skillfollow discovery is incomplete: %w", ErrFollowedUpdate, err)
	}
	return nil
}

// CheckClean fails closed when git cannot inspect the user's working tree.
func (p *FollowedUpdate) CheckClean() error {
	cmd := exec.Command("git", "status", "--porcelain", "-z", "--untracked-files=all")
	cmd.Dir = p.Path
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("%w: cannot check followed repository status; resolve in `%s`: %v", ErrFollowedUpdate, p.Path, err)
	}
	if len(out) != 0 {
		return fmt.Errorf("%w: followed repository has uncommitted changes; resolve in `%s`", ErrFollowedUpdate, p.Path)
	}
	return nil
}

// Pull never invokes conflict resolution, cleanup, or force recovery.
func (p *FollowedUpdate) Pull(onProgress ProgressCallback) error {
	if err := p.CheckClean(); err != nil {
		return err
	}
	args := []string{"pull", "--ff-only", "--no-rebase", "--quiet"}
	if onProgress != nil {
		args[len(args)-1] = "--progress"
	}
	if err := runGitCommandWithProgress(args, p.Path, authEnv(getRemoteURL(p.Path)), onProgress); err != nil {
		return fmt.Errorf("%w: fast-forward update failed; resolve in `%s`: %v", ErrFollowedUpdate, p.Path, err)
	}
	return nil
}
