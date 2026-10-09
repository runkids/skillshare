package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
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
