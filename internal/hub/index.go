package hub

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"skillshare/internal/install"
	ssync "skillshare/internal/sync"
)

// Index is the private hub index document format (schema v1).
type Index struct {
	SchemaVersion int          `json:"schemaVersion"`
	GeneratedAt   string       `json:"generatedAt"`
	SourcePath    string       `json:"sourcePath,omitempty"`
	Skills        []SkillEntry `json:"skills"`
}

// SkillEntry is one skill item in a hub index.
// In minimal mode only Name, Description, Source are emitted.
// In full mode all metadata fields are included (with omitempty).
type SkillEntry struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Source      string `json:"source,omitempty"`

	// Metadata fields — only emitted with --full.
	FlatName    string `json:"flatName,omitempty"`
	RelPath     string `json:"relPath,omitempty"`
	Type        string `json:"type,omitempty"`
	RepoURL     string `json:"repoUrl,omitempty"`
	Version     string `json:"version,omitempty"`
	InstalledAt string `json:"installedAt,omitempty"`
	IsInRepo    *bool  `json:"isInRepo,omitempty"`
}

// BuildIndex scans the source directory and returns a hub index.
// If full is true, metadata fields are included; otherwise only
// name, description, source are populated.
func BuildIndex(sourcePath string, full bool) (*Index, error) {
	// Fail fast if source directory does not exist.
	if _, err := os.Stat(sourcePath); err != nil {
		return nil, fmt.Errorf("source directory: %w", err)
	}

	discovered, err := ssync.DiscoverSourceSkills(sourcePath)
	if err != nil {
		return nil, err
	}

	entries := make([]SkillEntry, 0, len(discovered))
	for _, d := range discovered {
		item := SkillEntry{
			Name: filepath.Base(d.SourcePath),
		}

		// Determine source: prefer meta.Source (remote origin), fallback to relPath.
		source := d.RelPath
		if meta, _ := install.ReadMeta(d.SourcePath); meta != nil {
			if meta.Source != "" {
				source = meta.Source
			}
			if full {
				item.Type = meta.Type
				item.RepoURL = meta.RepoURL
				item.Version = meta.Version
				if !meta.InstalledAt.IsZero() {
					item.InstalledAt = meta.InstalledAt.UTC().Format(time.RFC3339)
				}
			}
		}
		item.Source = source

		if desc, _ := readSkillDescription(d.SourcePath); desc != "" {
			item.Description = desc
		}

		if full {
			// Only emit flatName when different from name.
			if d.FlatName != item.Name {
				item.FlatName = d.FlatName
			}
			// Only emit relPath when different from source.
			if d.RelPath != source {
				item.RelPath = d.RelPath
			}
			// Only emit isInRepo when true (omitempty on *bool skips nil/false).
			if d.IsInRepo {
				v := true
				item.IsInRepo = &v
			}
		}

		entries = append(entries, item)
	}

	// Deterministic output: sort by name ascending.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	return &Index{
		SchemaVersion: 1,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		SourcePath:    sourcePath,
		Skills:        entries,
	}, nil
}

// WriteIndex writes the index JSON to a file with stable pretty formatting.
func WriteIndex(path string, idx *Index) error {
	if idx == nil {
		return fmt.Errorf("index is nil")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

// readSkillDescription extracts the description from SKILL.md frontmatter.
func readSkillDescription(skillPath string) (string, error) {
	data, err := os.ReadFile(filepath.Join(skillPath, "SKILL.md"))
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return "", nil
	}
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "---" {
			break
		}
		if strings.HasPrefix(strings.ToLower(line), "description:") {
			desc := strings.TrimSpace(strings.TrimPrefix(line, "description:"))
			desc = strings.TrimPrefix(desc, "Description:")
			desc = strings.TrimSpace(desc)
			desc = strings.Trim(desc, `"'`)
			// Skip YAML block scalar indicators (| or >).
			if desc == "|" || desc == ">" || desc == "|+" || desc == "|-" || desc == ">+" || desc == ">-" {
				return "", nil
			}
			return desc, nil
		}
	}
	return "", nil
}
