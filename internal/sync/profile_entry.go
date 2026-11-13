package sync

import (
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/utils"
)

type EntryClass string

const (
	EntryAbsent       EntryClass = "absent"
	EntryManagedLink  EntryClass = "managed_link"
	EntryCleanCopy    EntryClass = "clean_copy"
	EntryModifiedCopy EntryClass = "modified_copy"
	EntryUnmanaged    EntryClass = "unmanaged"
)

type EntryInput struct {
	Name, Path, SourceRoot string
	Manifest               *Manifest
	Source                 *CopySource
}

type EntryClassification struct {
	Class               EntryClass
	Fingerprint, Reason string
}

type EntryChange struct {
	Dir         string     `json:"-"`
	Target      string     `json:"target"`
	Entry       string     `json:"entry"`
	Step        string     `json:"step"`
	Class       EntryClass `json:"class"`
	Reason      string     `json:"reason"`
	Fingerprint string     `json:"-"`
}

type DiscardEntry struct {
	Target, Dir, Entry, Fingerprint string
}

func discardAllowed(target, dir, name string, entry EntryClassification, discard []DiscardEntry) bool {
	if entry.Class != EntryModifiedCopy || entry.Fingerprint == "" || dir == "" {
		return false
	}
	for _, d := range discard {
		if d.Target == target && filepath.IsAbs(d.Dir) && filepath.Clean(d.Dir) == dir && d.Entry == name && d.Fingerprint == entry.Fingerprint {
			return true
		}
	}
	return false
}

func cleanEntryDir(dir string) string {
	if dir == "" {
		return ""
	}
	absolute, _ := filepath.Abs(dir)
	return absolute
}

func entryMutationAllowed(target string, input EntryInput, discard []DiscardEntry) (EntryClassification, bool) {
	entry := ClassifyEntry(input)
	switch entry.Class {
	case EntryAbsent, EntryManagedLink, EntryCleanCopy:
		return entry, true
	case EntryModifiedCopy:
		return entry, discardAllowed(target, cleanEntryDir(filepath.Dir(input.Path)), input.Name, entry, discard)
	default:
		return entry, false
	}
}

// legacyCopySource finds original inputs without guessing from target contents.
func legacyCopySource(name string, skills []DiscoveredSkill, manifest *Manifest, ignore []string) *CopySource {
	for _, skill := range skills {
		for _, candidate := range targetNameCandidates(skill) {
			if candidate.name != name {
				continue
			}
			if manifest != nil && manifest.Naming[name] != "" && manifest.Naming[name] != candidate.naming {
				continue
			}
			source := &CopySource{Path: skill.SourcePath, IgnorePatterns: ignore}
			if candidate.naming == "prefixed" && candidate.name != filepath.Base(skill.SourcePath) {
				source.OutputName = candidate.name
			}
			return source
		}
	}
	return nil
}

// ClassifyEntry determines ownership from the actual entry and manifest, not target mode.
func ClassifyEntry(in EntryInput) EntryClassification {
	info, err := os.Lstat(in.Path)
	if os.IsNotExist(err) {
		return EntryClassification{Class: EntryAbsent}
	}
	managed := ""
	if in.Manifest != nil {
		managed = in.Manifest.Managed[in.Name]
	}
	if err != nil {
		if managed != "" && managed != manifestSymlink {
			return EntryClassification{Class: EntryModifiedCopy, Reason: err.Error()}
		}
		return EntryClassification{Class: EntryUnmanaged, Reason: err.Error()}
	}
	if utils.IsLinkMode(in.Path, info.Mode()) {
		if managed == manifestSymlink && in.SourceRoot != "" {
			link, err := utils.ResolveLinkTarget(in.Path)
			root, rootErr := filepath.Abs(in.SourceRoot)
			if err == nil && rootErr == nil {
				root = utils.ResolveSymlink(root)
				link = evalOrClean(link)
				if utils.PathHasPrefix(link, strings.TrimSuffix(root, string(filepath.Separator))+string(filepath.Separator)) {
					return EntryClassification{Class: EntryManagedLink}
				}
			}
		}
		return EntryClassification{Class: EntryUnmanaged, Reason: "link is not a recorded link into the skills source"}
	}
	if !info.IsDir() || managed == "" || managed == manifestSymlink {
		return EntryClassification{Class: EntryUnmanaged, Reason: "entry is not a recorded copy"}
	}
	var sources []CopySource
	if in.Source != nil {
		sources = append(sources, *in.Source)
	}
	sum, clean := in.Manifest.verifiedCopyFingerprint(in.Name, in.Path, true, sources...)
	if clean {
		return EntryClassification{Class: EntryCleanCopy, Fingerprint: sum}
	}
	sum, _ = CopyOutputFingerprint(in.Path)
	return EntryClassification{Class: EntryModifiedCopy, Fingerprint: sum, Reason: "copy was edited or could not be verified"}
}

// PreviewEntry resolves original copy inputs and shares the writer's grant-free check.
func PreviewEntry(name, dir, root string, skills []DiscoveredSkill, manifest *Manifest, ignore []string) (EntryClassification, bool) {
	return entryMutationAllowed("", EntryInput{Name: name, Path: filepath.Join(dir, name), SourceRoot: root, Manifest: manifest, Source: legacyCopySource(name, skills, manifest, ignore)}, nil)
}

// PreviewPreservedEntries uses the writer's ownership checks without grants or writes.
// Sources come from full discovery, independently of the desired selection.
func PreviewPreservedEntries(target, dir, root, mode string, resolution *TargetSkillResolution, skills []DiscoveredSkill, manifest *Manifest, ignore []string, orphanNames ...string) []EntryChange {
	names := make(map[string]string)
	if manifest != nil {
		for name := range manifest.Managed {
			names[name] = "prune"
		}
	}
	for _, name := range orphanNames {
		names[name] = "prune"
	}
	legacy := RenamedFrom(resolution.LegacyNames(mode, dir, manifest))
	for _, skill := range resolution.Skills {
		name := skill.TargetName
		if old, ok := legacy[name]; ok {
			name = old
		}
		names[name] = "refresh"
	}
	var kept []EntryChange
	for name, step := range names {
		entry, allowed := PreviewEntry(name, dir, root, skills, manifest, ignore)
		if allowed {
			continue
		}
		kept = append(kept, EntryChange{Target: target, Dir: cleanEntryDir(dir), Entry: name, Step: step, Class: entry.Class, Reason: previewKeptReason(entry.Class, entry.Reason)})
	}
	return kept
}

// PreviewMovedProjectEntries reuses the frozen cleanup plan in read-only mode.
func PreviewMovedProjectEntries(root, source string, targets []MovedTarget, skills []DiscoveredSkill, ignore []string) []EntryChange {
	out := cleanMovedProjectDirsWithPolicy(root, source, targets, true, true, nil, PruneOptions{Skills: skills, IgnorePatterns: ignore, Quiet: true})
	var kept []EntryChange
	for _, target := range out.Outcomes {
		kept = append(kept, target.KeptModified...)
		kept = append(kept, target.Blocked...)
	}
	for i := range kept {
		kept[i].Reason = previewKeptReason(kept[i].Class, kept[i].Reason)
	}
	return kept
}

func previewKeptReason(class EntryClass, reason string) string {
	recovery := "; collect or remove manually"
	if class == EntryModifiedCopy {
		recovery += ", or use verified --discard-edits"
	}
	return "kept: " + reason + recovery
}
