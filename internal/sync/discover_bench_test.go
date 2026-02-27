package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkDiscoverSourceSkills(b *testing.B) {
	src := b.TempDir()
	for i := 0; i < 100; i++ {
		dir := filepath.Join(src, fmt.Sprintf("skill-%03d", i))
		os.MkdirAll(dir, 0755)
		os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: bench\n---\n# Bench"), 0644)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DiscoverSourceSkills(src)
	}
}

func BenchmarkDiscoverSourceSkillsLite(b *testing.B) {
	src := b.TempDir()
	for i := 0; i < 100; i++ {
		dir := filepath.Join(src, fmt.Sprintf("skill-%03d", i))
		os.MkdirAll(dir, 0755)
		os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: bench\n---\n# Bench"), 0644)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DiscoverSourceSkillsLite(src)
	}
}
