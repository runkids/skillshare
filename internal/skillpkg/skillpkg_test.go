package skillpkg

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Measure real package reloads, including supporting-file inspection, rather
// than cached catalog pages. Memory should not grow with supporting-file size.
func BenchmarkLoadSupportingFile(b *testing.B) {
	for _, size := range []int{64 << 10, 8 << 20} {
		for _, kind := range []string{"ASCII", "UTF8", "Invalid"} {
			b.Run(fmt.Sprintf("%s/%dKiB", kind, size>>10), func(b *testing.B) {
				dir := filepath.Join(b.TempDir(), "bench")
				if err := os.Mkdir(dir, 0755); err != nil {
					b.Fatal(err)
				}
				data := bytes.Repeat([]byte("a"), size)
				if kind == "UTF8" {
					copy(data, bytes.Repeat([]byte("界"), size/3))
				} else if kind == "Invalid" {
					data[0] = 0xff
				}
				for name, content := range map[string][]byte{
					"SKILL.md": []byte("---\nname: bench\ndescription: Benchmark package reload\n---\n"),
					"asset":    data,
				} {
					if err := os.WriteFile(filepath.Join(dir, name), content, 0644); err != nil {
						b.Fatal(err)
					}
				}
				b.SetBytes(int64(size))
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if _, err := Load(dir); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
