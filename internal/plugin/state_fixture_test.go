package plugin

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type hostFixture struct {
	t       *testing.T
	service *Service
	home    string
}

func seedLocalBinding(t *testing.T, s *Service, name, target string, b Binding) {
	t.Helper()
	d, err := s.load()
	if err != nil {
		t.Fatal(err)
	}
	d.putLocal(name, target, localBinding{Applied: &receipt{Intent: d.intent[name].Bindings[target].fingerprint(), Store: s.storeIdentity(target), Binding: b}})
	if err := s.saveState(d); err != nil {
		t.Fatal(err)
	}
}

func hostStatePiSource(t *testing.T) string {
	t.Helper()
	source := t.TempDir()
	writePluginFile(t, source, "package.json", `{"name":"demo","keywords":["pi-package"]}`)
	writePluginFile(t, source, "extensions/demo.ts", "export default () => {}")
	return source
}

func readState(t *testing.T, s *Service) *document {
	t.Helper()
	d, err := s.load()
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func applyFailure(t *testing.T, s *Service, r Request) *Result {
	t.Helper()
	p, err := s.Preview(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if p.Blocked {
		t.Fatalf("blocked instead of attempted: %+v", p.Changes)
	}
	result, err := s.Apply(context.Background(), r, p.Revision)
	if err == nil {
		t.Fatalf("expected failure: %+v", result)
	}
	return result
}

func repairHost(t *testing.T, s *Service) {
	t.Helper()
	t.Setenv("HOME", filepath.Dir(s.ConfigPath))
	t.Setenv("PI_CODING_AGENT_DIR", filepath.Join(filepath.Dir(s.ConfigPath), ".pi/agent"))
}

func repairGitSource(t *testing.T, name string) (string, string) {
	source, next, _ := repairGitSourceRepo(t, name)
	return source, next
}

func repairGitSourceRepo(t *testing.T, name string) (string, string, string) {
	t.Helper()
	src := hostStatePiSource(t)
	git := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = src
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git: %s %v", out, err)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-b", "main")
	git("add", ".")
	git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-m", "one")
	git("checkout", "-b", "next")
	writePluginFile(t, src, "extensions/demo.ts", "export default () => 2")
	git("add", ".")
	git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-m", "two")
	url := "https://fixture.invalid/" + name + ".git"
	cfg := filepath.Join(t.TempDir(), "gitconfig")
	writePluginFile(t, filepath.Dir(cfg), filepath.Base(cfg), "[url \"file://"+src+"\"]\n  insteadOf = "+url+"\n")
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)
	return url, git("rev-parse", "next"), src
}

func newHostFixture(t *testing.T, project bool) *hostFixture {
	t.Helper()
	s := (&fakeAgents{}).service(t)
	f := &hostFixture{t: t, service: s, home: filepath.Dir(s.ConfigPath)}
	if project {
		s.ProjectRoot = t.TempDir()
		s.ConfigPath = filepath.Join(s.ProjectRoot, ".skillshare", "config.yaml")
	}
	f.activate()
	return f
}

func (f *hostFixture) activate() {
	f.t.Helper()
	f.t.Setenv("HOME", f.home)
	f.t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(f.home, ".claude"))
	f.t.Setenv("CODEX_HOME", filepath.Join(f.home, ".codex"))
	f.t.Setenv("XDG_CONFIG_HOME", filepath.Join(f.home, ".config"))
	f.t.Setenv("PI_CODING_AGENT_DIR", filepath.Join(f.home, ".pi", "agent"))
}

func (f *hostFixture) apply(r Request) {
	f.t.Helper()
	f.activate()
	applyPluginRequest(f.t, f.service, r)
}

func (f *hostFixture) preview(r Request) *Plan {
	f.t.Helper()
	f.activate()
	p, err := f.service.Preview(context.Background(), r)
	if err != nil {
		f.t.Fatal(err)
	}
	return p
}

func (f *hostFixture) receive(from *hostFixture) {
	f.t.Helper()
	raw, err := os.ReadFile(from.service.ConfigPath)
	if err != nil {
		f.t.Fatal(err)
	}
	writePluginFile(f.t, filepath.Dir(f.service.ConfigPath), filepath.Base(f.service.ConfigPath), string(raw))
}

func hostPackages(t *testing.T, s *Service) map[string]Package {
	t.Helper()
	inventory, err := s.Packages()
	if err != nil {
		t.Fatal(err)
	}
	return inventory.Packages
}

func previewChanges(t *testing.T, s *Service, r Request) []Change {
	t.Helper()
	p, err := s.Preview(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	return p.Changes
}

// isolatedAgentService is the native-agent fixture used by host-state adapter tests.
func isolatedAgentService(t *testing.T, native *fakeAgents) *Service {
	t.Helper()
	s := native.service(t)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(filepath.Dir(s.ConfigPath), ".config"))
	for _, key := range []string{"OPENCODE_CONFIG", "OPENCODE_CONFIG_DIR", "OPENCODE_CONFIG_CONTENT"} {
		t.Setenv(key, "")
	}
	return s
}
