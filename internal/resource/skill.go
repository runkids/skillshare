package resource

import (
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/sourcewalk"
	"skillshare/internal/utils"
)

// SkillKind handles directory-based skill resources identified by SKILL.md.
type SkillKind struct{}

var _ ResourceKind = SkillKind{}

func (SkillKind) Kind() string { return "skill" }

// Discover scans sourceDir for directories containing SKILL.md.
// This is a simplified discovery for the resource package; the full
// discovery with ignore support, frontmatter parsing, and context
// collection remains in internal/sync/discover_walk.go.
func (SkillKind) Discover(sourceDir string) ([]DiscoveredResource, error) {
	walkRoot := utils.ResolveSymlink(sourceDir)

	var resources []DiscoveredResource

	err := sourcewalk.Walk(walkRoot, sourcewalk.Options{}, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		if !info.IsDir() && info.Name() == "SKILL.md" {
			skillDir := filepath.Dir(path)
			relPath, relErr := filepath.Rel(walkRoot, skillDir)
			if relErr != nil || relPath == "." {
				return nil
			}
			relPath = strings.ReplaceAll(relPath, "\\", "/")

			name := utils.ParseFrontmatterField(filepath.Join(skillDir, "SKILL.md"), "name")
			if name == "" {
				name = filepath.Base(skillDir)
			}

			isInRepo := false
			parts := strings.Split(relPath, "/")
			if len(parts) > 0 && utils.IsTrackedRepoDir(parts[0]) {
				isInRepo = true
			}

			resources = append(resources, DiscoveredResource{
				Name:       name,
				Kind:       "skill",
				RelPath:    relPath,
				AbsPath:    skillDir,
				IsNested:   strings.Contains(relPath, "/"),
				FlatName:   utils.PathToFlatName(relPath),
				IsInRepo:   isInRepo,
				SourcePath: filepath.Join(sourceDir, relPath),
			})
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return resources, nil
}

// ResolveName reads the name field from SKILL.md frontmatter.
// Falls back to directory base name if frontmatter has no name.
func (SkillKind) ResolveName(path string) string {
	skillFile := filepath.Join(path, "SKILL.md")
	name := utils.ParseFrontmatterField(skillFile, "name")
	if name != "" {
		return name
	}
	return filepath.Base(path)
}

// FlatName converts a relative path to a flat name using __ separator.
func (SkillKind) FlatName(relPath string) string {
	return utils.PathToFlatName(relPath)
}

// CreateLink creates a directory symlink from dst pointing to src.
func (SkillKind) CreateLink(src, dst string) error {
	return os.Symlink(src, dst)
}

func (SkillKind) SupportsAudit() bool   { return true }
func (SkillKind) SupportsTrack() bool   { return true }
func (SkillKind) SupportsCollect() bool { return true }
