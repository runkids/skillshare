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
	Target string     `json:"target"`
	Entry  string     `json:"entry"`
	Step   string     `json:"step"`
	Class  EntryClass `json:"class"`
	Reason string     `json:"reason"`
	Fingerprint string `json:"-"`
}

type DiscardEntry struct {
	Target, Entry, Fingerprint string
}

func discardAllowed(target, name string, entry EntryClassification, discard []DiscardEntry) bool {
	if entry.Class != EntryModifiedCopy || entry.Fingerprint == "" {
		return false
	}
	for _, d := range discard {
		if d.Target == target && d.Entry == name && d.Fingerprint == entry.Fingerprint {
			return true
		}
	}
	return false
}

func entryMutationAllowed(target string, input EntryInput, discard []DiscardEntry) (EntryClassification, bool) {
	entry := ClassifyEntry(input)
	switch entry.Class {
	case EntryAbsent, EntryManagedLink, EntryCleanCopy:
		return entry, true
	case EntryModifiedCopy:
		return entry, discardAllowed(target, input.Name, entry, discard)
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
				link = utils.ResolveSymlink(link)
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
	if in.Source == nil && in.Manifest.Naming[in.Name] == "prefixed" && in.Manifest.Outputs[in.Name] == "" {
		sum, _ := CopyOutputFingerprint(in.Path)
		return EntryClassification{Class: EntryModifiedCopy, Fingerprint: sum, Reason: "legacy prefixed copy cannot be verified without its source"}
	}
	var sources []CopySource
	if in.Source != nil {
		sources = append(sources, *in.Source)
	}
	sum, clean := in.Manifest.verifiedCopyFingerprint(in.Name, in.Path, sources...)
	if clean {
		return EntryClassification{Class: EntryCleanCopy, Fingerprint: sum}
	}
	sum, _ = CopyOutputFingerprint(in.Path)
	return EntryClassification{Class: EntryModifiedCopy, Fingerprint: sum, Reason: "copy was edited or could not be verified"}
}
