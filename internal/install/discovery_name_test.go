package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverSkills_ContainerDirectoryName(t *testing.T) {
	for _, tt := range []struct {
		dir  string
		name string
		want string
	}{
		{"skills", "MinerU Document Extractor", "mineru-document-extractor"},
		{"skill", "  Document   Extractor  ", "document-extractor"},
		{"skills", "my_skill-v2", "my_skill-v2"},
		{"skills", "", "skills"},
		{"skills", "../escape", "skills"},
		{"skills", "foo/bar", "skills"},
		{"skills", `foo\\bar`, "skills"},
		{"skills", "文件擷取", "skills"},
		{"skills", "agents", "skills"},
		{"skills", strings.Repeat("a", 65), "skills"},
		{"extractor", "MinerU Document Extractor", "extractor"},
	} {
		t.Run(tt.dir+"/"+tt.name, func(t *testing.T) {
			repo := t.TempDir()
			dir := filepath.Join(repo, tt.dir)
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: \""+tt.name+"\"\n---\n# Skill"), 0644); err != nil {
				t.Fatal(err)
			}
			for _, root := range []string{repo, dir} {
				skills := discoverSkills(root, true)
				if len(skills) != 1 || skills[0].Name != tt.want {
					t.Fatalf("discoverSkills(%q) = %+v, want name %q", root, skills, tt.want)
				}
			}
			source, err := ParseSource(dir)
			if err != nil {
				t.Fatal(err)
			}
			discovery, err := DiscoverLocal(source)
			if err != nil {
				t.Fatal(err)
			}
			if len(discovery.Skills) != 1 || discovery.Skills[0].Name != tt.want {
				t.Fatalf("DiscoverLocal = %+v, want name %q", discovery.Skills, tt.want)
			}
		})
	}
}

func TestInstallTrackedRepo_ContainerDirectoryNameReport(t *testing.T) {
	repo := initTestRepo(t)
	dir := filepath.Join(repo, "skills")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: MinerU Document Extractor\n---\n# Skill"), 0644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "add skill")
	source, err := ParseSource(localFileURL(repo))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	for _, update := range []bool{false, true} {
		result, err := InstallTrackedRepo(source, root, InstallOptions{Name: "team", Update: update})
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Skills) != 1 || result.Skills[0] != "skills" {
			t.Fatalf("tracked report must use the actual directory name: %+v", result.Skills)
		}
	}
}

func TestDiscoverSkills_ContainerDirectoryNameCollision(t *testing.T) {
	for _, paths := range [][]string{{"skills", "pdf"}, {"skills", "PDF"}, {"a/skills", "b/skill"}} {
		t.Run(strings.Join(paths, "+"), func(t *testing.T) {
			repo := t.TempDir()
			for _, path := range paths {
				dir := filepath.Join(repo, path)
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: PDF\n---\n# Skill"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			for _, skill := range discoverSkills(repo, true) {
				if skill.Name != filepath.Base(skill.Path) {
					t.Fatalf("colliding frontmatter name should fall back to directory: %+v", skill)
				}
			}
		})
	}
}

func TestDiscoverFromGitSubdir_ContainerDirectoryName(t *testing.T) {
	repo := initTestRepo(t)
	skillDir := filepath.Join(repo, "nested", "skills")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: MinerU Document Extractor\n---\n# Skill"), 0644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "add skill")
	source := &Source{
		Type: SourceTypeGitHTTPS, CloneURL: localFileURL(repo),
		Raw: localFileURL(repo) + "/nested/skills", Subdir: "nested/skills", Name: "skills",
	}
	discovery, err := DiscoverFromGitSubdir(source)
	if err != nil {
		t.Fatal(err)
	}
	defer CleanupDiscovery(discovery)
	if len(discovery.Skills) != 1 || discovery.Skills[0].Name != "mineru-document-extractor" || discovery.Skills[0].Path != "." {
		t.Fatalf("unexpected subdir discovery: %+v", discovery.Skills)
	}

	// Source path resolution must still use directory basenames, not frontmatter.
	resolved, err := resolveSubdir(repo, "skills")
	if err != nil || resolved != "nested/skills" {
		t.Fatalf("resolveSubdir = %q, %v", resolved, err)
	}
	if skill, ok := lookupSkillSubdir(repo, "nested/skills"); !ok || skill.Name != "mineru-document-extractor" {
		t.Fatalf("unexpected batch lookup: %+v, %v", skill, ok)
	}
}
