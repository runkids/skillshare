package hub

import (
	"encoding/json"
	"path/filepath"
	"sort"

	"skillshare/internal/install"
	ssync "skillshare/internal/sync"
	"skillshare/internal/utils"
)

// DraftCandidates offers installed skills without treating local relative paths
// as GitHub shorthand. Unknown origins stay local until the author supplies one.
func DraftCandidates(sourcePath string) ([]DraftEntry, error) {
	return DraftCandidatesWithOptions(sourcePath, ssync.DiscoveryOptions{})
}

// DraftCandidatesWithOptions shares the operation's source discovery policy.
func DraftCandidatesWithOptions(sourcePath string, opts ssync.DiscoveryOptions) ([]DraftEntry, error) {
	discovered, _, err := ssync.DiscoverSourceSkillsWithOptions(sourcePath, opts)
	if err != nil {
		return nil, err
	}
	metadata := install.LoadMetadataOrNew(sourcePath)
	entries := make([]DraftEntry, 0, len(discovered))
	for _, skill := range discovered {
		data := make(map[string]json.RawMessage)
		setField(data, "name", filepath.Base(skill.SourcePath))
		setField(data, "source", skill.SourcePath)
		entry := metadata.GetByPath(skill.RelPath)
		selector := ""
		if entry == nil && skill.IsInRepo {
			for parent := filepath.Dir(skill.RelPath); parent != "."; parent = filepath.Dir(parent) {
				candidate := metadata.GetByPath(parent)
				if candidate == nil || !candidate.Tracked {
					continue
				}
				parsed, err := parseInstalled(candidate)
				if err == nil && parsed.Subdir == "" {
					entry = candidate
					relative, err := filepath.Rel(parent, skill.RelPath)
					if err == nil {
						selector = filepath.ToSlash(relative)
					}
				}
				break
			}
		}
		if entry != nil && entry.Source != "" {
			setField(data, "source", installedSource(entry))
			if selector != "" {
				setField(data, "skill", selector)
			} else if parsed, err := parseInstalled(entry); err == nil && parsed.Subdir == "" && entry.Subdir != "" {
				setField(data, "skill", entry.Subdir)
			}
		}
		if desc := utils.ParseFrontmatterField(filepath.Join(skill.SourcePath, "SKILL.md"), "description"); desc != "" {
			setField(data, "description", desc)
		}
		if tags := utils.ParseFrontmatterList(filepath.Join(skill.SourcePath, "SKILL.md"), "tags"); len(tags) > 0 {
			data["tags"], _ = json.Marshal(tags)
		}
		if SourceProblem(field(data, "source")) == "credentials" {
			setField(data, "source", "")
		}
		entries = append(entries, DraftEntry{ID: skill.RelPath, Data: data})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	return entries, nil
}

// parseInstalled parses the entry's source as it was installed, so a web URL
// whose ref contains "/" splits into ref and subdir as the install did.
func parseInstalled(entry *install.MetadataEntry) (*install.Source, error) {
	parsed, err := install.ParseSource(entry.Source)
	if err == nil && entry.Branch != "" {
		parsed.ApplyRecordedBranch(entry.Branch)
	}
	return parsed, err
}

// installedSource returns the entry's source pinned at the ref it was
// installed from, so the hub entry installs the same version.
func installedSource(entry *install.MetadataEntry) string {
	if entry.Branch == "" {
		return entry.Source
	}
	parsed, err := parseInstalled(entry)
	if err != nil {
		return entry.Source
	}
	if pinned, err := parsed.AtRef(entry.Branch); err == nil {
		return pinned
	}
	return entry.Source
}
