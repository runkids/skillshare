package sync

import (
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/sourcewalk"
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

func entryMutationAllowed(target string, input EntryInput, discard []DiscardEntry, sources ...CopySource) (EntryClassification, bool) {
	entry := classifyEntry(input, sources...)
	switch entry.Class {
	case EntryAbsent, EntryManagedLink, EntryCleanCopy:
		return entry, true
	case EntryModifiedCopy:
		return entry, discardAllowed(target, cleanEntryDir(filepath.Dir(input.Path)), input.Name, entry, discard)
	default:
		return entry, false
	}
}

// legacyCopySources enumerates naming-compatible originals without verifying the copy.
func legacyCopySources(name, dir string, skills []DiscoveredSkill, manifest *Manifest, ignore []string) []CopySource {
	info, err := os.Lstat(filepath.Join(dir, name))
	if err != nil || !info.IsDir() || utils.IsLinkMode(filepath.Join(dir, name), info.Mode()) || manifest == nil || manifest.Managed[name] == "" || manifest.Managed[name] == manifestSymlink {
		return nil
	}
	if output, present := manifest.Outputs[name]; present && !legacyOutput(output) {
		return nil
	}
	var sources []CopySource
	for _, skill := range skills {
		candidates := targetNameCandidates(skill)[:2]
		if skill.declaredName != "" && candidates[0].name != name && candidates[1].name != name && PrefixedTargetName(skill, skill.declaredName) != name {
			continue
		}
		skillName, err := utils.ParseSkillName(skill.SourcePath)
		if err == nil {
			candidates = append(candidates, namedTarget{"prefixed", PrefixedTargetName(skill, skillName)})
		}
		if candidates[0].name != name && candidates[1].name != name && (len(candidates) == 2 || candidates[2].name != name) {
			continue
		}
		first := len(sources)
		for _, candidate := range candidates {
			// A migrated folder can retain its original naming's rewritten bytes.
			if recorded := manifest.Naming[name]; recorded != "" && recorded != candidate.naming || recorded == "" && candidate.name != name {
				continue
			}
			source := CopySource{Path: skill.SourcePath, IgnorePatterns: ignore}
			// The producer compares the output name with the declared name.
			if candidate.naming == "prefixed" && candidate.name != skillName {
				source.OutputName = candidate.name
			}
			if len(sources) > first && sources[len(sources)-1].OutputName == source.OutputName {
				continue
			}
			sources = append(sources, source)
		}
	}
	return sources
}

// ClassifyEntry determines ownership from the actual entry and manifest, not target mode.
func ClassifyEntry(in EntryInput) EntryClassification {
	return classifyEntry(in)
}

func classifyEntry(in EntryInput, sources ...CopySource) EntryClassification {
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
			if err == nil && linkWithinSource(link, in.SourceRoot) {
				return EntryClassification{Class: EntryManagedLink}
			}
		}
		return EntryClassification{Class: EntryUnmanaged, Reason: "link is not a recorded link into the skills source"}
	}
	if !info.IsDir() || managed == "" || managed == manifestSymlink {
		return EntryClassification{Class: EntryUnmanaged, Reason: "entry is not a recorded copy"}
	}
	if in.Source != nil {
		sources = append(sources, *in.Source)
	}
	sum, clean := in.Manifest.verifiedCopyFingerprint(in.Name, in.Path, true, sources...)
	if clean {
		return EntryClassification{Class: EntryCleanCopy, Fingerprint: sum}
	}
	return EntryClassification{Class: EntryModifiedCopy, Fingerprint: sum, Reason: "copy was edited or could not be verified"}
}

// linkWithinSource preserves the logical descendants written by createLink.
// Following them would move sanctioned source-directory links into external
// checkouts and lose the identity that the manifest records.
func linkWithinSource(link, source string) bool {
	root, err := filepath.Abs(source)
	if err != nil {
		return false
	}
	prefix := func(path string) string {
		return strings.TrimSuffix(path, string(filepath.Separator)) + string(filepath.Separator)
	}
	if utils.PathHasPrefix(link, prefix(root)) {
		return true
	}
	root = sourcewalk.Canonical(root)
	return utils.PathHasPrefix(link, prefix(root)) || utils.PathHasPrefix(sourcewalk.Canonical(link), prefix(root))
}

// PreviewEntry resolves original copy inputs and shares the writer's grant-free check.
func PreviewEntry(name, dir, root string, skills []DiscoveredSkill, manifest *Manifest, ignore []string) (EntryClassification, bool) {
	return entryMutationAllowed("", EntryInput{Name: name, Path: filepath.Join(dir, name), SourceRoot: root, Manifest: manifest}, nil, legacyCopySources(name, dir, skills, manifest, ignore)...)
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
