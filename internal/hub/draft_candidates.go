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
	discovered, err := ssync.DiscoverSourceSkills(sourcePath)
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
				parsed, err := install.ParseSource(candidate.Source)
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
			setField(data, "source", entry.Source)
			if selector != "" {
				setField(data, "skill", selector)
			} else if parsed, err := install.ParseSource(entry.Source); err == nil && parsed.Subdir == "" && entry.Subdir != "" {
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
