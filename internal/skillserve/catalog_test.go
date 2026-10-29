package skillserve

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/skillpkg"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func skillMD(name string) string {
	return "---\nname: " + name + "\ndescription: Use when testing " + name + "\nmetadata:\n  tags: [a, b]\n---\n# " + name + "\n"
}

func find(c *Catalog, uri string) *Skill {
	for _, s := range c.Skills {
		if s.URI == uri {
			return s
		}
	}
	return nil
}

func hasWarning(c *Catalog, substr string) bool {
	for _, w := range c.Skipped {
		if strings.Contains(w.String(), substr) {
			return true
		}
	}
	return false
}

func TestBuild_PublishesFrontmatterAndCompleteManifest(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "_team/tools/pdf/SKILL.md"), skillMD("pdf"))
	writeFile(t, filepath.Join(src, "_team/tools/pdf/references/FORMS.md"), "forms")
	writeFile(t, filepath.Join(src, "_team/tools/pdf/.git/HEAD"), "ref")

	c, err := (&Builder{Source: src}).Build()
	if err != nil {
		t.Fatal(err)
	}
	s := find(c, "skill://_team/tools/pdf/SKILL.md")
	if s == nil {
		t.Fatalf("skill not published; skills=%v warnings=%v", c.Skills, c.Skipped)
	}
	if s.Frontmatter["name"] != "pdf" || s.Frontmatter["metadata"] == nil {
		t.Errorf("frontmatter = %v, want full frontmatter", s.Frontmatter)
	}
	sum := sha256.Sum256([]byte("forms"))
	want := map[string]File{
		"skill://_team/tools/pdf/SKILL.md":            {Size: int64(len(skillMD("pdf")))},
		"skill://_team/tools/pdf/references/FORMS.md": {Size: 5, Digest: "sha256:" + hex.EncodeToString(sum[:])},
	}
	if len(s.Resources) != len(want) {
		t.Fatalf("resources = %+v, want SKILL.md and FORMS.md only", s.Resources)
	}
	for _, f := range s.Resources {
		w, ok := want[f.URI]
		if !ok || f.Size != w.Size || (w.Digest != "" && f.Digest != w.Digest) {
			t.Errorf("unexpected resource %+v", f)
		}
	}
}

func TestBuild_SkipsSkillWhoseNameDiffersFromDirectory(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "renamed/SKILL.md"), skillMD("original"))

	c, err := (&Builder{Source: src}).Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Skills) != 0 || !hasWarning(c, `does not match directory name "renamed"`) {
		t.Errorf("skills=%v warnings=%v, want renamed skipped with a warning", c.Skills, c.Skipped)
	}
}

func TestBuild_AppliesTargetSelectionAndDisabledState(t *testing.T) {
	src := t.TempDir()
	for _, n := range []string{"keep", "drop", "off"} {
		writeFile(t, filepath.Join(src, n, "SKILL.md"), skillMD(n))
	}
	writeFile(t, filepath.Join(src, ".skillignore"), "off\n")

	c, err := (&Builder{Source: src, Target: "claude", Skills: &config.ResourceTargetConfig{Exclude: []string{"drop"}}}).Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Skills) != 1 || c.Skills[0].URI != "skill://keep/SKILL.md" {
		t.Errorf("skills = %v, want only keep", c.Skills)
	}
}

func TestBuild_ReportsAmbiguousTargetVariants(t *testing.T) {
	src := t.TempDir()
	for _, agent := range []string{"claude", "claude-code", "codex"} {
		writeFile(t, filepath.Join(src, "_repo/skills", agent, "fix/SKILL.md"), skillMD("fix"))
	}
	c, err := (&Builder{Source: src, Target: "claude", Skills: &config.ResourceTargetConfig{}}).Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Skills) != 0 || len(c.Skipped) != 2 || !hasWarning(c, "ambiguous") || !hasWarning(c, "target override") {
		t.Fatalf("ambiguous catalog silently skipped variants: skills=%v, skipped=%v", c.Skills, c.Skipped)
	}
	if c.Skipped[0].Path != "_repo/skills/claude-code/fix" || c.Skipped[1].Path != "_repo/skills/claude/fix" {
		t.Fatalf("skip paths must retain source order: %v", c.Skipped)
	}
	account := config.TargetConfig{Agent: "codex", Skills: &config.ResourceTargetConfig{TargetNaming: "prefixed"}}
	sc := account.SkillsConfig()
	c, err = (&Builder{Source: src, Target: "codex-work", Skills: &sc}).Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Skills) != 1 || c.Skills[0].URI != "skill://_repo/skills/codex/fix/SKILL.md" || len(c.Skipped) != 0 {
		t.Fatalf("account catalog must serve the selected physical variant: skills=%v, skipped=%v", c.Skills, c.Skipped)
	}
}

func TestBuild_SkipsParentOfExcludedNestedSkill(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "suite/SKILL.md"), skillMD("suite"))
	writeFile(t, filepath.Join(src, "suite/child/SKILL.md"), skillMD("child"))

	c, err := (&Builder{Source: src, Target: "claude", Skills: &config.ResourceTargetConfig{Exclude: []string{"suite__child"}}}).Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Skills) != 0 || !hasWarning(c, "suite/child") {
		t.Errorf("skills=%v warnings=%v, want suite skipped because of excluded child", c.Skills, c.Skipped)
	}
}

// A parent and its nested skill list the same files under the same URIs; an
// unchanged tree serves both.
func TestBuild_ServesParentAndNestedSkillTogether(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "suite/SKILL.md"), skillMD("suite"))
	writeFile(t, filepath.Join(src, "suite/child/SKILL.md"), skillMD("child"))

	c, err := (&Builder{Source: src}).Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Skills) != 2 {
		t.Errorf("skills=%v warnings=%v, want suite and suite/child served", c.Skills, c.Skipped)
	}
}

// A nested skill created after discovery would otherwise be published only as a
// file of its parent.
func TestBuild_ListsNestedSkillCreatedDuringTheBuild(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "suite/SKILL.md"), skillMD("suite"))
	afterDiscover = func() { writeFile(t, filepath.Join(src, "suite/child/SKILL.md"), skillMD("child")) }
	t.Cleanup(func() { afterDiscover = func() {} })

	c, err := (&Builder{Source: src}).Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Skills) != 2 {
		t.Errorf("skills=%v warnings=%v, want suite and suite/child served", c.Skills, c.Skipped)
	}
}

func TestBuild_SkipsParentOfInvalidNestedSkill(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "suite/SKILL.md"), skillMD("suite"))
	writeFile(t, filepath.Join(src, "suite/child/SKILL.md"), skillMD("other"))

	c, err := (&Builder{Source: src}).Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Skills) != 0 || !hasWarning(c, "contains suite/child") {
		t.Errorf("skills=%v warnings=%v, want suite skipped because its child is skipped", c.Skills, c.Skipped)
	}
}

// A non-string key decodes to a map JSON cannot encode, which would fail the whole skills/list page.
func TestBuild_SkipsSkillWhoseFrontmatterJSONCannotCarry(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "ok/SKILL.md"), skillMD("ok"))
	writeFile(t, filepath.Join(src, "odd/SKILL.md"), "---\nname: odd\ndescription: Use when testing odd\nmetadata:\n  1: x\n---\n")

	c, err := (&Builder{Source: src}).Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Skills) != 1 || c.Skills[0].URI != "skill://ok/SKILL.md" || !hasWarning(c, "skipped odd") {
		t.Errorf("skills=%v warnings=%v, want odd skipped and ok served", c.Skills, c.Skipped)
	}
}

// The Skills extension requires SKILL.md to begin with its frontmatter.
func TestBuild_SkipsSkillWhoseFrontmatterIsNotAtTheStart(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "late/SKILL.md"), "# intro\n"+skillMD("late"))
	writeFile(t, filepath.Join(src, "open/SKILL.md"), "---\nname: open\ndescription: Use when testing open\n# open\n")
	writeFile(t, filepath.Join(src, "indented/SKILL.md"), "  "+skillMD("indented"))
	writeFile(t, filepath.Join(src, "closer/SKILL.md"), "---\nname: closer\ndescription: Use when testing\n  ---\n# closer\n")

	c, err := (&Builder{Source: src}).Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Skills) != 0 || !hasWarning(c, "skipped late") || !hasWarning(c, "skipped open") || !hasWarning(c, "skipped indented") || !hasWarning(c, "skipped closer") {
		t.Errorf("skills=%v warnings=%v, want late, unclosed open and indented skipped", c.Skills, c.Skipped)
	}
}

// The Agent Skills format caps a description at 1,024 characters and compatibility at 500, not bytes.
func TestBuild_SkipsSkillWhoseDescriptionOrCompatibilityIsTooLong(t *testing.T) {
	src := t.TempDir()
	skill := func(name, desc string) string { return "---\nname: " + name + "\ndescription: " + desc + "\n---\n" }
	writeFile(t, filepath.Join(src, "long/SKILL.md"), skill("long", strings.Repeat("a", 1025)))
	writeFile(t, filepath.Join(src, "wide/SKILL.md"), skill("wide", strings.Repeat("技", 1024)))
	writeFile(t, filepath.Join(src, "compat/SKILL.md"), "---\nname: compat\ndescription: Use when testing\ncompatibility: "+strings.Repeat("a", 501)+"\n---\n")
	writeFile(t, filepath.Join(src, "blank/SKILL.md"), "---\nname: blank\ndescription: Use when testing\ncompatibility: \"\"\n---\n")

	c, err := (&Builder{Source: src}).Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Skills) != 1 || c.Skills[0].URI != "skill://wide/SKILL.md" || !hasWarning(c, "skipped long") || !hasWarning(c, "skipped compat") || !hasWarning(c, "skipped blank") {
		t.Errorf("skills=%v warnings=%v, want long and compat skipped, wide served", c.Skills, c.Skipped)
	}
}

// The 16 MiB limit counts the bytes read, SKILL.md included.
func TestBuild_SkipsSkillOverTheSizeLimit(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "big/SKILL.md"), skillMD("big"))
	writeFile(t, filepath.Join(src, "big/data.bin"), strings.Repeat("x", skillpkg.MaxBytes-len(skillMD("big"))+1))

	c, err := (&Builder{Source: src}).Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Skills) != 0 || !hasWarning(c, "skipped big: is larger than 16 MiB") {
		t.Errorf("skills=%v warnings=%v, want big skipped", c.Skills, c.Skipped)
	}
}

// The Agent Skills reference validator takes lowercase Unicode letters and digits,
// after NFKC normalization, and counts the 64-character limit in characters.
func TestBuild_AcceptsLowercaseUnicodeNames(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "技能-審查/SKILL.md"), skillMD("技能-審查"))
	writeFile(t, filepath.Join(src, "café/SKILL.md"), skillMD("café"))
	writeFile(t, filepath.Join(src, "Été/SKILL.md"), skillMD("Été"))
	writeFile(t, filepath.Join(src, "a_b/SKILL.md"), skillMD("a_b"))
	// The URI leaf is the directory, and it must equal the name exactly, not only after NFKC.
	writeFile(t, filepath.Join(src, "abc/SKILL.md"), skillMD("ａｂｃ"))
	// Surrounding whitespace is not part of a valid name, even when the directory has it too.
	if runtime.GOOS != "windows" {
		writeFile(t, filepath.Join(src, " sp /SKILL.md"), strings.Replace(skillMD("x"), "name: x", `name: " sp "`, 1))
	}

	c, err := (&Builder{Source: src}).Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Skills) != 2 || !hasWarning(c, "skipped Été") || !hasWarning(c, "skipped a_b") || !hasWarning(c, "skipped abc") || (runtime.GOOS != "windows" && !hasWarning(c, "skipped  sp ")) {
		t.Errorf("skills=%v warnings=%v, want 技能-審查 and café served, Été, a_b and fullwidth abc skipped", c.Skills, c.Skipped)
	}
}

func TestBuild_SkipsSkillOverFileLimit(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "big/SKILL.md"), skillMD("big"))
	for i := range skillpkg.MaxFiles {
		writeFile(t, filepath.Join(src, "big/refs", fmt.Sprintf("f%03d", i)), "")
	}

	c, err := (&Builder{Source: src}).Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Skills) != 0 || !hasWarning(c, "512") {
		t.Errorf("skills=%d warnings=%v, want big skipped over the file limit", len(c.Skills), c.Skipped)
	}
}

func TestBuild_RefreshesDigestWhenContentChanges(t *testing.T) {
	src := t.TempDir()
	ref := filepath.Join(src, "doc/notes.md")
	writeFile(t, filepath.Join(src, "doc/SKILL.md"), skillMD("doc"))
	writeFile(t, ref, "one")
	b := &Builder{Source: src}
	first, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	// Same size and modification time, as cp -p or rsync -t leave it.
	info, err := os.Stat(ref)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, ref, "two")
	if err := os.Chtimes(ref, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	second, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	digest := func(c *Catalog) string {
		for _, f := range c.Skills[0].Resources {
			if strings.HasSuffix(f.URI, "notes.md") {
				return f.Digest
			}
		}
		return ""
	}
	if digest(first) == digest(second) {
		t.Error("digest did not change after the file changed")
	}
}

func TestRead_ServesOnlyManifestFilesInsideTheSkill(t *testing.T) {
	src := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.txt")
	writeFile(t, outside, "secret")
	writeFile(t, filepath.Join(src, "doc/SKILL.md"), skillMD("doc"))
	if err := os.Symlink(outside, filepath.Join(src, "doc/leak.txt")); err != nil {
		t.Fatal(err)
	}
	c, err := (&Builder{Source: src}).Build()
	if err != nil {
		t.Fatal(err)
	}

	if data, err := c.Read("skill://doc/SKILL.md"); err != nil || string(data) != skillMD("doc") {
		t.Errorf("Read SKILL.md = %q, %v", data, err)
	}
	for _, uri := range []string{"skill://doc/leak.txt", "skill://doc/../doc/SKILL.md", "skill://doc/%2e%2e/doc/SKILL.md"} {
		if _, err := c.Read(uri); err == nil {
			t.Errorf("Read(%s) succeeded, want rejection", uri)
		}
	}
}

// Bytes that no longer match the listed digest would be rejected by a verifying client.
func TestRead_RefusesFileChangedSinceListed(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "doc/SKILL.md"), skillMD("doc"))
	writeFile(t, filepath.Join(src, "doc/notes.md"), "one")
	c, err := (&Builder{Source: src}).Build()
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(src, "doc/notes.md"), "two")

	if _, err := c.Read("skill://doc/notes.md"); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Errorf("Read error = %v, want the file reported as changed", err)
	}
}

func TestBuild_ReadsFrontmatterAfterBOMOrWithCRLF(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "bom/SKILL.md"), "\ufeff"+skillMD("bom"))
	writeFile(t, filepath.Join(src, "crlf/SKILL.md"), strings.ReplaceAll(skillMD("crlf"), "\n", "\r\n"))

	c, err := (&Builder{Source: src}).Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Skills) != 2 {
		t.Errorf("skills=%v warnings=%v, want bom and crlf served", c.Skills, c.Skipped)
	}
}

// The walk lists no links, so a linked SKILL.md would leave the skill without its own file.
func TestBuild_SkipsSkillWhoseSkillMDIsALink(t *testing.T) {
	src := t.TempDir()
	target := filepath.Join(t.TempDir(), "SKILL.md")
	writeFile(t, target, skillMD("linked"))
	if err := os.MkdirAll(filepath.Join(src, "linked"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(src, "linked/SKILL.md")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}

	c, err := (&Builder{Source: src}).Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Skills) != 0 || !hasWarning(c, "skipped linked") {
		t.Errorf("skills=%v warnings=%v, want linked skipped", c.Skills, c.Skipped)
	}
}

func TestBuild_SkipsSkillWithNonStringCompatibilityOrOversizedSkillMD(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "list/SKILL.md"), "---\nname: list\ndescription: Use when testing\ncompatibility: [linux]\n---\n")
	writeFile(t, filepath.Join(src, "huge/SKILL.md"), skillMD("huge")+strings.Repeat("x", skillpkg.MaxBytes))

	c, err := (&Builder{Source: src}).Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Skills) != 0 || !hasWarning(c, "skipped list") || !hasWarning(c, "skipped huge: is larger than 16 MiB") {
		t.Errorf("skills=%v warnings=%v, want list and huge skipped", c.Skills, c.Skipped)
	}
}

func TestRead_ServesListedFileWithBackslashInName(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a backslash separates paths on Windows")
	}
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "doc/SKILL.md"), skillMD("doc"))
	writeFile(t, filepath.Join(src, `doc/a\b.txt`), "x")
	c, err := (&Builder{Source: src}).Build()
	if err != nil {
		t.Fatal(err)
	}
	if data, err := c.Read("skill://doc/a%5Cb.txt"); err != nil || string(data) != "x" {
		t.Errorf("Read = %q, %v; want the listed file", data, err)
	}
}

func TestBuild_ServesParentWithDifferentlyRoutedVariant(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "_repo/parent/SKILL.md"), skillMD("parent"))
	for _, agent := range []string{"claude", "codex"} {
		writeFile(t, filepath.Join(src, "_repo/parent/skills", agent, "fix/SKILL.md"), skillMD("fix"))
	}
	for _, tc := range []struct {
		name     string
		sc       config.ResourceTargetConfig
		disabled bool
	}{
		{name: "excluded peer", sc: config.ResourceTargetConfig{Exclude: []string{"_repo__parent__skills__codex__fix"}}},
		{name: "peer omitted by include", sc: config.ResourceTargetConfig{Include: []string{"_repo__parent", "_repo__parent__skills__claude__fix"}}},
		{name: "disabled peer", disabled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.disabled {
				writeFile(t, filepath.Join(src, ".skillignore"), "_repo/parent/skills/codex/fix\n")
			}
			c, err := (&Builder{Source: src, Target: "claude", Skills: &tc.sc}).Build()
			if err != nil {
				t.Fatal(err)
			}
			if find(c, "skill://_repo/parent/SKILL.md") != nil {
				t.Fatalf("parent exposes unserved filtered peer: skips=%v", c.Skipped)
			}
		})
	}
	writeFile(t, filepath.Join(src, ".skillignore"), "")
	for _, target := range []string{"claude", "codex"} {
		c, err := (&Builder{Source: src, Target: target, Skills: &config.ResourceTargetConfig{}}).Build()
		if err != nil {
			t.Fatal(err)
		}
		if find(c, "skill://_repo/parent/SKILL.md") == nil || len(c.Skills) != 2 {
			t.Fatalf("%s: skills=%v skips=%v", target, c.Skills, c.Skipped)
		}
	}
}

func TestBuild_VariantParentRespectsExplicitChildTargets(t *testing.T) {
	for _, override := range []bool{false, true} {
		t.Run(fmt.Sprintf("override=%t", override), func(t *testing.T) {
			src := t.TempDir()
			writeFile(t, filepath.Join(src, "_repo/parent/SKILL.md"), skillMD("parent"))
			for _, agent := range []string{"claude", "codex"} {
				content := skillMD("fix")
				if agent == "codex" && !override {
					content = strings.Replace(content, "metadata:\n", "metadata:\n  targets: [codex]\n", 1)
				}
				writeFile(t, filepath.Join(src, "_repo/parent/skills", agent, "fix/SKILL.md"), content)
			}
			if override {
				store := install.NewMetadataStore()
				store.SetTargetOverride("_repo/parent/skills/codex/fix", []string{"codex"})
				if err := store.Save(src); err != nil {
					t.Fatal(err)
				}
			}
			c, err := (&Builder{Source: src, Target: "claude", Skills: &config.ResourceTargetConfig{}}).Build()
			if err != nil {
				t.Fatal(err)
			}
			if find(c, "skill://_repo/parent/SKILL.md") != nil {
				t.Fatalf("parent exposes explicitly restricted child: skips=%v", c.Skipped)
			}
		})
	}
}
