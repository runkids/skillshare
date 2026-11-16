package backup

import (
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"skillshare/internal/config"
)

// ErrAmbiguousEntry means a backup name identifies more than one resource.
var ErrAmbiguousEntry = errors.New("ambiguous backup entry")

// MovedEntryName identifies a declared former project directory without relying
// on declaration order or conflating slashes with characters in a directory name.
func MovedEntryName(target, rel string) string {
	rel = path.Clean(filepath.ToSlash(rel))
	return target + "-moved-" + hex.EncodeToString([]byte(rel))
}

// SkillsEntryDir resolves only configured skills names and declared former
// project paths. It never decodes an untrusted key into a filesystem path and
// does not require the destination to exist. Agent-name collisions refuse even
// when a skills name exactly matches, so forced recovery cannot change resource.
func SkillsEntryDir(root, entry string, targets map[string]config.TargetConfig) (string, error) {
	identities := make(map[[2]string]string)
	agentName := false
	for name, target := range targets {
		if entry == name+AgentsEntrySuffix {
			_, builtin := config.LookupProjectAgentTarget(name)
			agentName = builtin || target.AgentsConfig().Path != ""
		}
		if entry == name {
			identities[[2]string{name, ""}] = target.SkillsConfig().Path
		}
		for _, declared := range config.AlsoScansProject(name) {
			rel := path.Clean(filepath.ToSlash(declared))
			if rel == "." || path.IsAbs(rel) || filepath.IsAbs(declared) || rel == ".." || strings.HasPrefix(rel, "../") {
				continue
			}
			if entry == MovedEntryName(name, rel) {
				identities[[2]string{name, rel}] = filepath.Join(root, filepath.FromSlash(rel))
			}
		}
	}
	if len(identities) > 1 || len(identities) > 0 && agentName {
		return "", fmt.Errorf("%w: %s", ErrAmbiguousEntry, entry)
	}
	for _, dir := range identities {
		if dir != "" {
			return dir, nil
		}
	}
	return "", fmt.Errorf("skills backup entry %q not found in configured targets or declared former paths", entry)
}
