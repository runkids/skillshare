//go:build !online

package integration

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"skillshare/internal/testutil"
)

func TestArchiveCLIEndToEnd(t *testing.T) {
	for _, mode := range []string{"-g", "-p"} {
		t.Run(mode, func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
			cwd, source := sb.Root, sb.SourcePath
			if mode == "-p" {
				cwd = sb.SetupProjectDir("claude")
				source = filepath.Join(cwd, ".skillshare", "skills")
			}
			var mu sync.Mutex
			version := "v1"
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				var body bytes.Buffer
				z := zip.NewWriter(&body)
				f, _ := z.Create("brief/SKILL.md")
				_, _ = f.Write([]byte("# Brief " + version))
				_ = z.Close()
				_, _ = w.Write(body.Bytes())
			}))
			defer srv.Close()
			cert := filepath.Join(sb.Root, "fixture-ca.pem")
			if err := os.WriteFile(cert, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}), 0600); err != nil {
				t.Fatal(err)
			}
			sb.SetEnv("SSL_CERT_FILE", cert)
			run := func(args ...string) *testutil.Result { return sb.RunCLIInDir(cwd, append(args, mode)...) }
			run("install", srv.URL+"/pack.zip", "--name", "custom", "--into", "legal").AssertSuccess(t)
			run("check", "--json", "--exit-code").AssertSuccess(t)
			result := run("update", "--all", "--json")
			result.AssertSuccess(t)
			var counts struct {
				Updated  int
				UpToDate int `json:"up_to_date"`
			}
			if err := json.Unmarshal([]byte(result.Stdout), &counts); err != nil {
				t.Fatal(err, result.Stdout)
			}
			if counts.Updated != 0 || counts.UpToDate != 1 {
				t.Fatalf("no-op counts = %+v", counts)
			}
			run("install", srv.URL+"/pack.zip", "--name", "other", "--into", "legal").AssertSuccess(t)
			mu.Lock()
			version = "v2"
			mu.Unlock()
			run("check", "--json", "--exit-code").AssertExitCode(t, 1)
			run("update", "legal/custom", "--json").AssertSuccess(t)
			file := filepath.Join(source, "legal", "custom", "SKILL.md")
			content, err := os.ReadFile(file)
			if err != nil || string(content) != "# Brief v2" {
				t.Fatalf("changed update = %s, %v", content, err)
			}
			if err := os.WriteFile(file, []byte("# Local edit"), 0644); err != nil {
				t.Fatal(err)
			}
			run("update", "legal/custom", "--json").AssertFailure(t)
			mu.Lock()
			version = "v3"
			mu.Unlock()
			run("update", "--all", "--json").AssertSuccess(t)
			content, _ = os.ReadFile(file)
			if string(content) != "# Local edit" {
				t.Fatalf("bulk update overwrote local edits: %s", content)
			}
			run("check", "legal/custom", "--diff").AssertOutputContains(t, "modified SKILL.md")
			other, _ := os.ReadFile(filepath.Join(source, "legal", "other", "SKILL.md"))
			if string(other) != "# Brief v3" {
				t.Fatalf("bulk update skipped unmodified member: %s", other)
			}
			run("update", "--all", "--json", "--force").AssertSuccess(t)
			content, _ = os.ReadFile(file)
			if string(content) != "# Brief v3" {
				t.Fatal("force failed to restore archive")
			}
		})
	}
}
