package sync

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
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
	Outputs   map[string]string `json:"outputs,omitempty"` // entry name → checksum of the written copy
	UpdatedAt time.Time         `json:"updated_at"`
}

// ReadManifest reads the manifest from a target directory.
// Returns an empty manifest if the file does not exist.
func ReadManifest(targetPath string) (*Manifest, error) {
	p := filepath.Join(targetPath, ManifestFile)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &Manifest{Managed: make(map[string]string), Mtimes: make(map[string]int64), Naming: make(map[string]string), Outputs: make(map[string]string)}, nil
		}
		return nil, err
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		// Corrupt manifest — treat as empty so next sync rebuilds it.
		return &Manifest{Managed: make(map[string]string), Mtimes: make(map[string]int64), Naming: make(map[string]string), Outputs: make(map[string]string)}, nil
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
	if m.Outputs == nil {
		m.Outputs = make(map[string]string)
	}
	return &m, nil
}

// Remove forgets every record of the entry name.
func (m *Manifest) Remove(name string) {
	delete(m.Managed, name)
	delete(m.Mtimes, name)
	delete(m.Naming, name)
	delete(m.Outputs, name)
}

// CopyToLinkReason is the diff reason for a copy-mode copy that merge sync replaces with a link.
const CopyToLinkReason = "copy mode copy (sync replaces with link)"

// CopySource supplies the original inputs needed to verify a legacy rewrite.
type CopySource struct {
	Path, OutputName string
	IgnorePatterns   []string
}

// CopyOutputFingerprint hashes the output using the standard ignored files.
func CopyOutputFingerprint(dir string) (string, error) {
	var entries []checksumEntry
	if err := collectChecksumEntries(dir, "", &entries, map[string]bool{}, compileFileIgnore(DefaultFileIgnorePatterns()), true); err != nil {
		return "", err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].relPath < entries[j].relPath })
	h := sha256.New()
	io.WriteString(h, "skillshare-copy-output\x00v2\x00")
	writeLength := func(n int) {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(n))
		h.Write(size[:])
	}
	writeLength(len(entries))
	for _, entry := range entries {
		writeLength(len(entry.relPath))
		io.WriteString(h, entry.relPath)
		writeLength(len(entry.content))
		h.Write(entry.content)
	}
	return fmt.Sprintf("sha256-v2:%x", h.Sum(nil)), nil
}

// OwnsCopy reports whether dir still matches the recorded copy, failing closed
// when an older rewritten copy cannot be verified from its original source.
func (m *Manifest) OwnsCopy(name, dir string, source ...CopySource) bool {
	_, ok := m.verifiedCopyFingerprint(name, dir, false, source...)
	return ok
}

// legacyOutput recognizes only the previous exact lowercase bare-hex encoding.
func legacyOutput(output string) bool {
	return len(output) == 64 && strings.Trim(output, "0123456789abcdef") == ""
}

func (m *Manifest) verifiedCopyFingerprint(name, dir string, strict bool, source ...CopySource) (string, bool) {
	if m == nil {
		return "", false
	}
	recorded, ok := m.Managed[name]
	if !ok || recorded == manifestSymlink {
		return "", false
	}
	sum, err := CopyOutputFingerprint(dir)
	if err != nil {
		return "", false
	}
	output, present := m.Outputs[name]
	if present && strings.HasPrefix(output, "sha256-v2:") {
		return sum, legacyOutput(strings.TrimPrefix(output, "sha256-v2:")) && sum == output
	}
	if present && !legacyOutput(output) {
		return sum, false // unknown, malformed and present-empty values never downgrade
	}
	if present || (!strict && len(source) == 0) {
		old, err := DirChecksumWithIgnore(dir, DefaultFileIgnorePatterns())
		if err != nil || (present && old != output) {
			return sum, false
		}
		if !strict && len(source) == 0 {
			return sum, present || old == recorded
		}
	}
	if len(source) == 0 || source[0].Path == "" {
		return sum, false
	}
	input := source[0]
	ignore := input.IgnorePatterns
	if ignore == nil {
		ignore = DefaultFileIgnorePatterns()
	}
	sourceSum, err := DirChecksumWithIgnore(input.Path, ignore)
	if err != nil || sourceSum != recorded {
		return "", false
	}
	scratch, err := os.MkdirTemp("", "skillshare-copy-replay-*")
	if err != nil {
		return "", false
	}
	defer os.RemoveAll(scratch)
	replay := filepath.Join(scratch, "copy")
	if err := copySkillToTarget(input.Path, replay, input.OutputName, ignore); err != nil {
		return "", false
	}
	expected, err := CopyOutputFingerprint(replay)
	return sum, err == nil && sum == expected
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
