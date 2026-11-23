package sync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/utils"
)

// ManifestFile is the filename for the sync manifest.
const ManifestFile = ".skillshare-manifest.json"

// manifestSymlink is the Managed value merge mode records for a link it made.
const manifestSymlink = "symlink"

// Manifest tracks which skills are managed by skillshare in a target directory.
// Used by both merge mode (values: "symlink") and copy mode (values: SHA-256 checksum).
type Manifest struct {
	Managed   map[string]string `json:"managed"`           // flatName → "symlink" (merge) or SHA-256 checksum (copy)
	Mtimes    map[string]int64  `json:"mtimes,omitempty"`  // flatName → source dir max mtime (UnixNano), copy mode only
	Sources   map[string]string `json:"sources,omitempty"` // output name → SHA-256 of the source it was converted from, extension outputs only
	Naming    map[string]string `json:"naming,omitempty"`  // entry name → target_naming that produced it, copy mode only
	UpdatedAt time.Time         `json:"updated_at"`
}

// ReadManifest reads the manifest from a target directory.
// Returns an empty manifest if the file does not exist.
func ReadManifest(targetPath string) (*Manifest, error) {
	p := filepath.Join(targetPath, ManifestFile)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &Manifest{Managed: make(map[string]string), Mtimes: make(map[string]int64), Naming: make(map[string]string)}, nil
		}
		return nil, err
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		// Corrupt manifest — treat as empty so next sync rebuilds it.
		return &Manifest{Managed: make(map[string]string), Mtimes: make(map[string]int64), Naming: make(map[string]string)}, nil
	}
	if m.Managed == nil {
		m.Managed = make(map[string]string)
	}
	if m.Mtimes == nil {
		m.Mtimes = make(map[string]int64)
	}
	if m.Naming == nil {
		m.Naming = make(map[string]string)
	}
	return &m, nil
}

// Remove forgets every record of the entry name.
func (m *Manifest) Remove(name string) {
	delete(m.Managed, name)
	delete(m.Mtimes, name)
	delete(m.Naming, name)
}

// CopyToLinkReason is the diff reason for a copy-mode copy that merge sync replaces with a link.
const CopyToLinkReason = "copy mode copy (sync replaces with link)"

// OwnsCopy reports whether dir is still the copy that copy mode made for name:
// the manifest records a checksum (not a merge-mode link) and the folder still
// hashes to it, so nothing the user edited is lost when it is replaced.
// ignorePatterns are the effective patterns the copy was made under.
func (m *Manifest) OwnsCopy(name, dir string, ignorePatterns []string) bool {
	if m == nil {
		return false
	}
	recorded, ok := m.Managed[name]
	if !ok || recorded == manifestSymlink {
		return false
	}
	sum, err := DirChecksumWithIgnore(dir, copyHashPatterns(ignorePatterns))
	return err == nil && sum == recorded
}

// copyHashPatterns keeps the default patterns and the rules that re-include
// files. Hashing a copy under them counts every file sync copied, and still
// counts a file the user added to the copy that ignore: names.
func copyHashPatterns(ignorePatterns []string) []string {
	out := DefaultFileIgnorePatterns()
	for _, p := range ignorePatterns {
		if strings.HasPrefix(p, "!") {
			out = append(out, p)
		}
	}
	return out
}

// ForeignLinkReason is the diff reason for a same-name link that sync keeps
// because skillshare did not create it.
const ForeignLinkReason = "link from another tool (sync --force to replace)"

// Records reports whether the manifest has an entry for name, as a link or as
// a copy. A manifest that could not be read records nothing.
func (m *Manifest) Records(name string) bool {
	if m == nil {
		return false
	}
	_, ok := m.Managed[name]
	return ok
}

// OwnsLink reports whether the link at path under name is skillshare's to
// re-point or remove: the manifest records a link there, or it points into the
// source. A link that another tool or the user made under the same name is
// neither, also where the manifest records a copy the link has since replaced.
func (m *Manifest) OwnsLink(name, path, sourcePath string) bool {
	if m != nil && m.Managed[name] == manifestSymlink {
		return true
	}
	absSource, err := filepath.Abs(sourcePath)
	return err == nil && linksIntoSource(path, absSource)
}

// OrphanLinkVerdict is what prune does with a link in a target that no skill
// in the source maps to. At most one field is set; none means the link is kept
// without comment, as the user's own.
type OrphanLinkVerdict struct {
	// RemoveReason says why prune removes the link.
	RemoveReason string
	// KeptWarning is what sync prints after the entry name when it keeps the
	// link and says so.
	KeptWarning string
}

// OrphanLink decides what prune does with the orphan link at path. inManifest
// says whether the manifest records the entry. Prune and the dashboard diff
// both call this, so they cannot disagree.
func OrphanLink(path, sourcePath string, inManifest, force bool) OrphanLinkVerdict {
	dest, err := utils.ResolveLinkTarget(path)
	if err != nil {
		return OrphanLinkVerdict{KeptWarning: "unable to resolve link target, kept"}
	}
	_, statErr := os.Stat(dest)
	broken := statErr != nil
	absSource, absErr := filepath.Abs(sourcePath)

	switch {
	case absErr == nil && linksIntoSource(path, absSource):
		if broken {
			return OrphanLinkVerdict{RemoveReason: "broken symlink to source"}
		}
		if inManifest || force {
			return OrphanLinkVerdict{RemoveReason: "orphan symlink to source"}
		}
		// A live link into the source that skillshare never created is the
		// user's own wiring.
		return OrphanLinkVerdict{}
	case broken:
		// The destination is gone, for example after the source moved. Remove
		// the link only if skillshare made it: a broken link another tool or the
		// user made is theirs to clean up.
		if inManifest || force {
			return OrphanLinkVerdict{RemoveReason: "broken external symlink"}
		}
		return OrphanLinkVerdict{KeptWarning: fmt.Sprintf("broken link to %s that skillshare did not create, kept (delete it yourself, or sync with --force)", dest)}
	case force:
		return OrphanLinkVerdict{RemoveReason: "external symlink (force)"}
	default:
		return OrphanLinkVerdict{KeptWarning: fmt.Sprintf("symlink to external location (%s), kept", dest)}
	}
}

// SkipsHidden reports whether a target scan should skip name. Hidden entries
// are skipped unless the manifest records them as managed (e.g. a source skill
// under ".system/" synced as ".system__example").
func (m *Manifest) SkipsHidden(name string) bool {
	if !utils.IsHidden(name) {
		return false
	}
	if m == nil {
		return true
	}
	_, managed := m.Managed[name]
	return !managed
}

// WriteManifest writes the manifest to a target directory.
func WriteManifest(targetPath string, m *Manifest) error {
	m.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(targetPath, ManifestFile), data, 0644)
}

// RemoveManifest deletes the manifest file from a target directory.
// It is a no-op if the file does not exist.
func RemoveManifest(targetPath string) error {
	err := os.Remove(filepath.Join(targetPath, ManifestFile))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
