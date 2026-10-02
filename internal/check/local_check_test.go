package check

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/install"
)

// localEntry writes a skill at a fresh local source dir and returns a
// metadata entry whose file hashes match it, as `install <path>` records.
func localEntry(t *testing.T) (*install.MetadataEntry, string) {
	t.Helper()
	src := filepath.Join(t.TempDir(), "my-skill")
	os.MkdirAll(src, 0755)
	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("# v1"), 0644)

	hashes, err := install.ComputeFileHashes(src)
	if err != nil {
		t.Fatal(err)
	}
	return &install.MetadataEntry{Source: src, Type: "local", FileHashes: hashes}, src
}

func TestLocalSourceStatus_Unchanged(t *testing.T) {
	entry, _ := localEntry(t)

	if status, _ := LocalSourceStatus(entry); status != "up_to_date" {
		t.Errorf("status = %q, want up_to_date", status)
	}
}

func TestLocalSourceStatus_FileChanged(t *testing.T) {
	entry, src := localEntry(t)
	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("# v2"), 0644)

	if status, _ := LocalSourceStatus(entry); status != "update_available" {
		t.Errorf("status = %q, want update_available", status)
	}
}

func TestLocalSourceStatus_FileAdded(t *testing.T) {
	entry, src := localEntry(t)
	os.WriteFile(filepath.Join(src, "reference.md"), []byte("new"), 0644)

	if status, _ := LocalSourceStatus(entry); status != "update_available" {
		t.Errorf("status = %q, want update_available", status)
	}
}

func TestLocalSourceStatus_SourceMissing(t *testing.T) {
	entry, src := localEntry(t)
	os.RemoveAll(src)

	status, msg := LocalSourceStatus(entry)
	if status != "error" || msg == "" {
		t.Errorf("got (%q, %q), want error with a message", status, msg)
	}
}

func TestLocalSourceStatus_NoRecordedHashes(t *testing.T) {
	entry, _ := localEntry(t)
	entry.FileHashes = nil

	if status, _ := LocalSourceStatus(entry); status != "local" {
		t.Errorf("status = %q, want local", status)
	}
}

func TestLocalSourceStatus_NotLocalType(t *testing.T) {
	entry, _ := localEntry(t)
	entry.Type = "github"

	if status, _ := LocalSourceStatus(entry); status != "local" {
		t.Errorf("status = %q, want local", status)
	}
}

func TestLocalSourceStatus_RelativeSourceIsNotCompared(t *testing.T) {
	entry, _ := localEntry(t)
	entry.Source = "./my-skill"

	if status, _ := LocalSourceStatus(entry); status != "local" {
		t.Errorf("status = %q, want local", status)
	}
}

// discoveryInstall installs the root skill of a local directory that also
// contains child skills, the way the dashboard does (DiscoverLocal +
// InstallFromDiscovery). That install copies only the root SKILL.md.
func discoveryInstall(t *testing.T) (repo string, entry *install.MetadataEntry) {
	t.Helper()
	tmp := t.TempDir()
	repo = filepath.Join(tmp, "repo")
	write := func(rel, body string) {
		p := filepath.Join(repo, rel)
		os.MkdirAll(filepath.Dir(p), 0755)
		os.WriteFile(p, []byte(body), 0644)
	}
	write("SKILL.md", "---\nname: root\n---\n# Root")
	write("README.md", "readme")
	write("skills/child/SKILL.md", "---\nname: child\n---\n# Child")

	discovery, err := install.DiscoverLocal(&install.Source{
		Type: install.SourceTypeLocalPath, Raw: repo, Path: repo, Name: "repo",
	})
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(tmp, "dest")
	for _, skill := range discovery.Skills {
		if skill.Path != "." {
			continue
		}
		opts := install.InstallOptions{SourceDir: dest, SkipAudit: true}
		if _, err := install.InstallFromDiscovery(discovery, skill, filepath.Join(dest, "root"), opts); err != nil {
			t.Fatal(err)
		}
	}
	entry = install.LoadMetadataOrNew(dest).GetByPath("root")
	if entry == nil {
		t.Fatal("root skill was not installed")
	}
	return repo, entry
}

func TestLocalSourceStatus_RootOfSkillCollection_UnchangedIsUpToDate(t *testing.T) {
	_, entry := discoveryInstall(t)

	if status, msg := LocalSourceStatus(entry); status != "up_to_date" {
		t.Errorf("status = %q (%s), want up_to_date", status, msg)
	}
}

func TestLocalSourceStatus_RootOfSkillCollection_IgnoresFilesNotInstalled(t *testing.T) {
	repo, entry := discoveryInstall(t)
	os.WriteFile(filepath.Join(repo, "README.md"), []byte("changed"), 0644)
	os.WriteFile(filepath.Join(repo, "skills/child/SKILL.md"), []byte("changed"), 0644)

	if status, _ := LocalSourceStatus(entry); status != "up_to_date" {
		t.Errorf("status = %q, want up_to_date", status)
	}
}

func TestLocalSourceStatus_RootOfSkillCollection_DetectsSkillFileChange(t *testing.T) {
	repo, entry := discoveryInstall(t)
	os.WriteFile(filepath.Join(repo, "SKILL.md"), []byte("changed"), 0644)

	if status, _ := LocalSourceStatus(entry); status != "update_available" {
		t.Errorf("status = %q, want update_available", status)
	}
}

func TestLocalSourceStatus_WholeDirectoryInstall_DetectsChildSkillChange(t *testing.T) {
	src := filepath.Join(t.TempDir(), "pack")
	os.MkdirAll(filepath.Join(src, "child"), 0755)
	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("# Pack"), 0644)
	os.WriteFile(filepath.Join(src, "child", "SKILL.md"), []byte("# v1"), 0644)
	hashes, _ := install.ComputeFileHashes(src)
	entry := &install.MetadataEntry{Source: src, Type: "local", FileHashes: hashes}

	os.WriteFile(filepath.Join(src, "child", "SKILL.md"), []byte("# v2"), 0644)

	if status, _ := LocalSourceStatus(entry); status != "update_available" {
		t.Errorf("status = %q, want update_available", status)
	}
}
