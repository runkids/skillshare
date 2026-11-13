//go:build !online

package integration

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/testutil"
)

func TestArchiveCLIEndToEnd(t *testing.T) {
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
	// SSL_CERT_FILE is not used by Go's macOS platform verifier. Build a
	// fixture-only CLI with explicit roots, keeping normal TLS verification.
	projectRoot := findProjectRoot(t)
	fixtureDir := t.TempDir()
	binary := filepath.Join(fixtureDir, "skillshare.exe")
	initFile := filepath.Join(fixtureDir, "tls.go")
	initSource := fmt.Sprintf(`package main
import ("crypto/tls"; "crypto/x509"; "net/http")
func init() {
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM([]byte(%q)) { panic("invalid fixture certificate") }
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{RootCAs: roots}
	http.DefaultTransport = transport
}
`, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}))
	if err := os.WriteFile(initFile, []byte(initSource), 0600); err != nil {
		t.Fatal(err)
	}
	overlay, err := json.Marshal(map[string]any{"Replace": map[string]string{
		filepath.Join(projectRoot, "cmd", "skillshare", "archive_tls_fixture.go"): initFile,
	}})
	if err != nil {
		t.Fatal(err)
	}
	overlayFile := filepath.Join(fixtureDir, "overlay.json")
	if err := os.WriteFile(overlayFile, overlay, 0600); err != nil {
		t.Fatal(err)
	}
	build := exec.Command("go", "build", "-overlay", overlayFile, "-o", binary, "./cmd/skillshare")
	build.Dir = projectRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build fixture CLI: %v\n%s", err, out)
	}
	for _, mode := range []string{"-g", "-p"} {
		t.Run(mode, func(t *testing.T) {
			mu.Lock()
			version = "v1"
			mu.Unlock()
			sb := testutil.NewSandbox(t)
			sb.BinaryPath = binary
			defer sb.Cleanup()
			sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
			cwd, source := sb.Root, sb.SourcePath
			if mode == "-p" {
				cwd = sb.SetupProjectDir("claude")
				source = filepath.Join(cwd, ".skillshare", "skills")
			}
			run := func(args ...string) *testutil.Result { return sb.RunCLIInDir(cwd, append(args, mode)...) }
			wrongHost := strings.Replace(srv.URL, "127.0.0.1", "localhost", 1)
			rejected := run("install", wrongHost+"/pack.zip", "--name", "wrong-host")
			rejected.AssertFailure(t)
			rejected.AssertOutputContains(t, "not localhost")
			if _, err := os.Stat(filepath.Join(source, "wrong-host")); !os.IsNotExist(err) {
				t.Fatal("wrong-host fixture installed a skill")
			}
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
			// The dashboard source editor uses this mapping without replacing file hashes.
			store := install.LoadMetadataOrNew(source)
			replacement, err := install.ParseSource(srv.URL + "/new.zip#skill=brief")
			if err != nil {
				t.Fatal(err)
			}
			store.GetByPath("legal/custom").SetSource(replacement)
			if err := store.Save(source); err != nil {
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
			// JSON output alone must not authorize replacing an edited ZIP install.
			sb.WriteFile(file, "# Local JSON edit")
			args := []string{"install", srv.URL + "/pack.zip", "--name", "custom", "--into", "legal", "--json"}
			refused := run(args...)
			refused.AssertFailure(t)
			output := parseJSON(t, refused.Stdout)
			if installed, ok := output["skills"].([]any); !ok || len(installed) != 0 {
				t.Fatalf("refused ZIP reported installed: %s", refused.Stdout)
			}
			if got := sb.ReadFile(file); got != "# Local JSON edit" {
				t.Fatalf("JSON overwrote local ZIP edit: %q", got)
			}
			forced := run(append(args, "--force")...)
			forced.AssertSuccess(t)
			output = parseJSON(t, forced.Stdout)
			if installed, ok := output["skills"].([]any); !ok || len(installed) != 1 || installed[0] != "custom" {
				t.Fatalf("forced ZIP not reported installed: %s", forced.Stdout)
			}
			if got := sb.ReadFile(file); got != "# Brief v3" {
				t.Fatalf("JSON force did not replace ZIP edit: %q", got)
			}
			noop := run(append(args, "--update")...)
			noop.AssertSuccess(t)
			output = parseJSON(t, noop.Stdout)
			if skipped, ok := output["skipped"].([]any); !ok || len(skipped) != 1 || skipped[0] != "custom" {
				t.Fatalf("unchanged ZIP not reported skipped: %s", noop.Stdout)
			}
			if installed, ok := output["skills"].([]any); !ok || len(installed) != 0 {
				t.Fatalf("unchanged ZIP reported installed: %s", noop.Stdout)
			}
		})
	}
}
