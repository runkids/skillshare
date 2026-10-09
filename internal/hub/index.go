package hub

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"skillshare/internal/audit"
	"skillshare/internal/install"
	"skillshare/internal/sourcewalk"
	ssync "skillshare/internal/sync"
	"skillshare/internal/utils"
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
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Source      string   `json:"source,omitempty"`
	Skill       string   `json:"skill,omitempty"`
	Tags        []string `json:"tags,omitempty"`

	// Metadata fields — only emitted with --full.
	FlatName    string `json:"flatName,omitempty"`
	RelPath     string `json:"relPath,omitempty"`
	Type        string `json:"type,omitempty"`
	RepoURL     string `json:"repoUrl,omitempty"`
	Version     string `json:"version,omitempty"`
	InstalledAt string `json:"installedAt,omitempty"`
	IsInRepo    *bool  `json:"isInRepo,omitempty"`

	// Audit fields — only emitted with --audit.
	RiskScore *int   `json:"riskScore,omitempty"`
	RiskLabel string `json:"riskLabel,omitempty"`
	AuditedAt string `json:"auditedAt,omitempty"`
}

// BuildIndex scans the source directory and returns a hub index.
// If full is true, metadata fields are included; otherwise only
// name, description, source are populated.
// If auditSkills is true, each skill is scanned with audit.ScanSkill
// and risk fields are populated.
func BuildIndex(sourcePath string, full bool, auditSkills bool, walk ...sourcewalk.Options) (*Index, error) {
	// Fail fast if source directory does not exist.
	if _, err := os.Stat(sourcePath); err != nil {
		return nil, fmt.Errorf("source directory: %w", err)
	}

	discovered, err := ssync.DiscoverSourceSkills(sourcePath, walk...)
	if err != nil {
		return nil, err
	}

	// Load centralized metadata store once for all skills.
	store := install.LoadMetadataOrNew(sourcePath)

	entries := make([]SkillEntry, len(discovered))
	for i, d := range discovered {
		item := SkillEntry{
			Name: filepath.Base(d.SourcePath),
		}

		// Determine source: prefer entry.Source (remote origin), fallback to relPath.
		source := d.RelPath
		if entry := store.GetByPath(d.RelPath); entry != nil {
			if entry.Source != "" {
				source = entry.Source
			}
			if full {
				item.Type = entry.Type
				item.RepoURL = entry.RepoURL
				item.Version = entry.Version
				if !entry.InstalledAt.IsZero() {
					item.InstalledAt = entry.InstalledAt.UTC().Format(time.RFC3339)
				}
			}
		}
		item.Source = source

		if desc, _ := readSkillDescription(d.SourcePath); desc != "" {
			item.Description = desc
		}
		if tags := readSkillTags(d.SourcePath); len(tags) > 0 {
			item.Tags = tags
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

		entries[i] = item
	}

	// Parallel audit enrichment: scan skills concurrently with bounded workers.
	if auditSkills {
		const maxWorkers = 8
		sem := make(chan struct{}, maxWorkers)
		var wg sync.WaitGroup
		for i, d := range discovered {
			wg.Add(1)
			sem <- struct{}{}
			go func(idx int, sourcePath string) {
				defer wg.Done()
				defer func() { <-sem }()
				if res, err := audit.ScanSkill(sourcePath); err == nil {
					score := res.RiskScore
					entries[idx].RiskScore = &score
					entries[idx].RiskLabel = res.RiskLabel
					entries[idx].AuditedAt = time.Now().UTC().Format(time.RFC3339)
				}
			}(i, d.SourcePath)
		}
		wg.Wait()
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

// readSkillTags extracts the tags from SKILL.md frontmatter.
// Supports comma-separated inline values: tags: git, workflow
func readSkillTags(skillPath string) []string {
	data, err := os.ReadFile(filepath.Join(skillPath, "SKILL.md"))
	if err != nil {
		return nil
	}
	lines := strings.Split(utils.TrimBOM(string(data)), "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return nil
	}
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "---" {
			break
		}
		if strings.HasPrefix(strings.ToLower(line), "tags:") {
			raw := strings.TrimSpace(line[len("tags:"):])
			raw = strings.Trim(raw, `"'`)
			if raw == "" {
				return nil
			}
			parts := strings.Split(raw, ",")
			tags := make([]string, 0, len(parts))
			for _, p := range parts {
				t := strings.TrimSpace(p)
				if t != "" {
					tags = append(tags, t)
				}
			}
			return tags
		}
	}
	return nil
}

// readSkillDescription extracts the description from SKILL.md frontmatter.
func readSkillDescription(skillPath string) (string, error) {
	data, err := os.ReadFile(filepath.Join(skillPath, "SKILL.md"))
	if err != nil {
		return "", err
	}
	lines := strings.Split(utils.TrimBOM(string(data)), "\n")
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
