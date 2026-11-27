package version

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pythonTree creates files under a temporary root and returns it. A path
// ending in "/" is a directory; any other path is a file holding its content.
func pythonTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for path, content := range files {
		full := filepath.Join(root, filepath.FromSlash(path))
		dir := full
		if !strings.HasSuffix(path, "/") {
			dir = filepath.Dir(full)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if dir == full {
			continue
		}
		if err := os.WriteFile(full, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

const distInfo = "skillshare_cli-0.31.0.dist-info/"

func TestDetectInstallMethodPyPI(t *testing.T) {
	tests := []struct {
		name  string
		exec  string
		files map[string]string
		want  InstallMethod
	}{
		{name: "uv tool", exec: "tools/skillshare-cli/bin/skillshare", files: map[string]string{
			"tools/skillshare-cli/bin/skillshare":                           "binary",
			"tools/skillshare-cli/pyvenv.cfg":                               "home = /usr/bin\nuv = 0.9.26\n",
			"tools/skillshare-cli/uv-receipt.toml":                          "[tool]\n",
			"tools/skillshare-cli/lib/python3.12/site-packages/" + distInfo: "",
		}, want: InstallUV},
		// uv venv writes the same pyvenv.cfg as uv tool; only a tool has a receipt.
		// A uv venv has no pip, so it gets uv pip's command.
		{name: "uv pip in a uv venv", exec: ".venv/bin/skillshare", files: map[string]string{
			".venv/bin/skillshare":                           "binary",
			".venv/pyvenv.cfg":                               "home = /usr/bin\nuv = 0.9.26\n",
			".venv/lib/python3.12/site-packages/" + distInfo: "",
		}, want: InstallUVPip},
		// uv venv --seed adds pip, and pip is then what installed the wheel.
		{name: "pip in a seeded uv venv", exec: ".venv/bin/skillshare", files: map[string]string{
			".venv/bin/skillshare": "binary",
			".venv/bin/pip":        "pip",
			".venv/pyvenv.cfg":     "home = /usr/bin\nuv = 0.9.26\n",
			".venv/lib/python3.12/site-packages/" + distInfo: "",
		}, want: InstallPip},
		// uvx runs from a throwaway environment in uv's cache, with no receipt.
		{name: "uvx", exec: "uv/archive-v0/I9r2ojtmqjfZAQfG/bin/skillshare", files: map[string]string{
			"uv/archive-v0/I9r2ojtmqjfZAQfG/bin/skillshare":                           "binary",
			"uv/archive-v0/I9r2ojtmqjfZAQfG/pyvenv.cfg":                               "home = /usr/bin\nuv = 0.12.17\nrelocatable = true\n",
			"uv/archive-v0/I9r2ojtmqjfZAQfG/lib/python3.12/site-packages/" + distInfo: "",
		}, want: InstallUVX},
		{name: "pipx", exec: "pipx/venvs/skillshare-cli/bin/skillshare", files: map[string]string{
			"pipx/venvs/skillshare-cli/bin/skillshare":                           "binary",
			"pipx/venvs/skillshare-cli/pyvenv.cfg":                               "home = /usr/bin\n",
			"pipx/venvs/skillshare-cli/pipx_metadata.json":                       "{}",
			"pipx/venvs/skillshare-cli/lib/python3.12/site-packages/" + distInfo: "",
		}, want: InstallPipx},
		{name: "custom home with app metadata", exec: "apps/venvs/skillshare-cli/bin/skillshare", files: map[string]string{
			"apps/venvs/skillshare-cli/bin/skillshare":                           "binary",
			"apps/venvs/skillshare-cli/pipx_metadata.json":                       "{}",
			"apps/venvs/skillshare-cli/lib/python3.12/site-packages/" + distInfo: "",
		}, want: InstallPipx},
		{name: "a plain venv under a pipx folder", exec: "pipx/venvs/mine/bin/skillshare", files: map[string]string{
			"pipx/venvs/mine/bin/skillshare":                           "binary",
			"pipx/venvs/mine/lib/python3.12/site-packages/" + distInfo: "",
		}, want: InstallPip},
		{name: "pip in a venv", exec: "venv/bin/skillshare", files: map[string]string{
			"venv/bin/skillshare":                           "binary",
			"venv/pyvenv.cfg":                               "home = /usr/bin\n",
			"venv/lib/python3.12/site-packages/" + distInfo: "",
		}, want: InstallPip},
		{name: "pip --user", exec: ".local/bin/skillshare", files: map[string]string{
			".local/bin/skillshare":                           "binary",
			".local/lib/python3.11/site-packages/" + distInfo: "",
		}, want: InstallPip},
		{name: "Debian system pip", exec: "usr/local/bin/skillshare", files: map[string]string{
			"usr/local/bin/skillshare":                           "binary",
			"usr/local/lib/python3.11/dist-packages/" + distInfo: "",
		}, want: InstallPip},
		{name: "Fedora platlib", exec: "usr/bin/skillshare", files: map[string]string{
			"usr/bin/skillshare":                             "binary",
			"usr/lib64/python3.12/site-packages/" + distInfo: "",
		}, want: InstallPip},
		{name: "Windows venv", exec: "venv/Scripts/skillshare.exe", files: map[string]string{
			"venv/Scripts/skillshare.exe":        "binary",
			"venv/Lib/site-packages/" + distInfo: "",
		}, want: InstallPip},
		{name: "Windows user scheme", exec: "Python311/Scripts/skillshare.exe", files: map[string]string{
			"Python311/Scripts/skillshare.exe":    "binary",
			"Python311/site-packages/" + distInfo: "",
		}, want: InstallPip},
		{name: "install.sh in ~/.local/bin", exec: ".local/bin/skillshare", files: map[string]string{
			".local/bin/skillshare": "binary",
		}, want: InstallDirect},
		{name: "another package's dist-info only", exec: "venv/bin/skillshare", files: map[string]string{
			"venv/bin/skillshare": "binary",
			"venv/lib/python3.12/site-packages/other_tool-1.0.dist-info/": "",
		}, want: InstallDirect},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := pythonTree(t, tt.files)
			execPath := filepath.Join(root, filepath.FromSlash(tt.exec))
			if got := DetectInstallMethod(execPath); got != tt.want {
				t.Fatalf("DetectInstallMethod(%q) = %v, want %v", tt.exec, got, tt.want)
			}
		})
	}
}

func TestPyPIUpgradeCommands(t *testing.T) {
	commands := map[InstallMethod]string{
		InstallUV:    "uv tool upgrade skillshare-cli",
		InstallPipx:  "pipx upgrade skillshare-cli",
		InstallPip:   "pip install --upgrade skillshare-cli",
		InstallUVPip: "uv pip install --upgrade skillshare-cli",
		InstallUVX:   "uvx --from skillshare-cli@latest skillshare",
	}
	for method, want := range commands {
		if got := method.UpgradeCommand(); got != want {
			t.Errorf("%v.UpgradeCommand() = %q, want %q", method, got, want)
		}
		if !method.IsPyPI() {
			t.Errorf("%v.IsPyPI() = false, want true", method)
		}
	}
	for _, method := range []InstallMethod{InstallNix, InstallNixProfile, InstallUV, InstallPipx, InstallPip, InstallUVPip, InstallUVX} {
		if !method.IsPackageManaged() {
			t.Errorf("%v.IsPackageManaged() = false, want true", method)
		}
	}
	for _, method := range []InstallMethod{InstallDirect, InstallBrew} {
		if method.IsPackageManaged() || method.IsPyPI() {
			t.Errorf("%v: IsPackageManaged() = %v, IsPyPI() = %v; want false, false", method, method.IsPackageManaged(), method.IsPyPI())
		}
	}
}

func TestFetchPyPILatestVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"info": {"name": "skillshare-cli", "version": "0.32.1"}}`))
	}))
	defer srv.Close()
	old := pypiJSONURL
	pypiJSONURL = srv.URL
	t.Cleanup(func() { pypiJSONURL = old })

	got, err := FetchPyPILatestVersion()
	if err != nil || got != "0.32.1" {
		t.Fatalf("FetchPyPILatestVersion() = %q, %v; want 0.32.1", got, err)
	}
}

// GitHub publishes before PyPI, so a wheel's installer can update only to what PyPI has.
func TestFetchLatestVersionForPyPIAsksPyPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"info": {"version": "0.0.7"}}`))
	}))
	defer srv.Close()
	old := pypiJSONURL
	pypiJSONURL = srv.URL
	t.Cleanup(func() { pypiJSONURL = old })

	for _, method := range []InstallMethod{InstallUV, InstallPipx, InstallPip, InstallUVPip, InstallUVX, InstallPixiPyPI} {
		if got, err := FetchLatestVersionFor(method); err != nil || got != "0.0.7" {
			t.Errorf("FetchLatestVersionFor(%v) = %q, %v; want PyPI's 0.0.7", method, got, err)
		}
	}
}

func TestFetchPyPILatestVersionMissingProject(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	old := pypiJSONURL
	pypiJSONURL = srv.URL
	t.Cleanup(func() { pypiJSONURL = old })

	if got, err := FetchPyPILatestVersion(); err == nil {
		t.Fatalf("FetchPyPILatestVersion() = %q, want an error for a project PyPI does not have", got)
	}
}
