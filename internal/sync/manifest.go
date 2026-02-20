package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// ManifestFile is the filename for the copy-mode manifest.
const ManifestFile = ".skillshare-manifest.json"

// Manifest tracks which skills are managed by copy mode in a target directory.
type Manifest struct {
	Managed   map[string]string `json:"managed"` // flatName → dirChecksum
	UpdatedAt time.Time         `json:"updated_at"`
}

// ReadManifest reads the manifest from a target directory.
// Returns an empty manifest if the file does not exist.
func ReadManifest(targetPath string) (*Manifest, error) {
	p := filepath.Join(targetPath, ManifestFile)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &Manifest{Managed: make(map[string]string)}, nil
		}
		return nil, err
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		// Corrupt manifest — treat as empty so next sync rebuilds it.
		return &Manifest{Managed: make(map[string]string)}, nil
	}
	if m.Managed == nil {
		m.Managed = make(map[string]string)
	}
	return &m, nil
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
