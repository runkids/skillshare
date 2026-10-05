package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const ompCode = "export default function (pi) {\n  pi.on(\"tool_call\", async () => {});\n}\n"

func ompEntry(t *testing.T, targets ...string) *Entry {
	t.Helper()
	var parts []string
	for _, target := range targets {
		parts = append(parts, `"`+target+`":{"code":`+jsonString(ompCode)+`}`)
	}
	return entry(t, `{"bindings":{`+strings.Join(parts, ",")+`}}`)
}

func jsonString(s string) string {
	return `"` + strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "\n", "\\n").Replace(s) + `"`
}

func TestOMP_AccountWritesIntoConfigDir(t *testing.T) {
	e := newEnv(t)
	dir := filepath.Join(e.home, "omp-work")
	must(t, os.MkdirAll(dir, 0755))
	e.service.Accounts = map[string]Account{"omp-work": {Agent: "omp", Dir: dir}}
	save(t, e.service, Mutation{Name: "demo", Entry: ompEntry(t, "omp-work")})
	if read(t, filepath.Join(dir, "extensions", "skillshare-demo.ts")) != ompCode {
		t.Fatal("account extension not written verbatim")
	}
	if _, err := os.Stat(filepath.Join(e.home, ".omp")); !os.IsNotExist(err) {
		t.Fatal("explicit account must not touch the default OMP home")
	}
}

func TestOMP_SharedAgentDirWithPiBlocksBeforeWriting(t *testing.T) {
	e := newEnv(t)
	shared := filepath.Join(e.home, "agent")
	e.service.ConfigDirs = map[string]string{"pi": shared, "omp": shared}
	_, err := e.service.PreviewMutation(Mutation{Name: "demo", Entry: ompEntry(t, "pi", "omp")})
	if err == nil || !strings.Contains(err.Error(), "omp and pi both write") || !strings.Contains(err.Error(), "PI_CODING_AGENT_DIR") {
		t.Fatalf("shared PI_CODING_AGENT_DIR must block: %v", err)
	}
	if _, err := os.Stat(shared); !os.IsNotExist(err) {
		t.Fatal("blocked plan wrote the shared directory")
	}
	// Distinct homes keep both Agents.
	e.service.ConfigDirs = map[string]string{}
	save(t, e.service, Mutation{Name: "demo", Entry: ompEntry(t, "pi", "omp")})
	if read(t, filepath.Join(e.home, ".pi", "agent", "extensions", "skillshare-demo.ts")) != ompCode || read(t, filepath.Join(e.home, ".omp", "agent", "extensions", "skillshare-demo.ts")) != ompCode {
		t.Fatal("Pi and OMP must each receive the extension in their own home")
	}
}

func TestOMP_NativeHookFactoriesAreListedNotImported(t *testing.T) {
	e := newEnv(t)
	agent := filepath.Join(e.home, ".omp", "agent")
	write(t, filepath.Join(agent, "hooks", "pre", "guard.ts"), "export default function () {}\n")
	write(t, filepath.Join(agent, "hooks", "post", "audit.js"), "export default function () {}\n")
	write(t, filepath.Join(agent, "hooks", "stray.ts"), "export default function () {}\n")
	write(t, filepath.Join(agent, "extensions", "mine.ts"), "export default function () {}\n")
	inv, err := e.service.List()
	must(t, err)
	got := map[string]string{}
	for _, u := range inv.Unmanaged {
		if u.Target == "omp" {
			got[strings.TrimPrefix(u.Path, agent+string(filepath.Separator))] = strings.Join(u.Names, ",")
		}
	}
	want := map[string]string{"extensions": "mine.ts", filepath.Join("hooks", "pre"): "guard.ts", filepath.Join("hooks", "post"): "audit.js"}
	if len(got) != len(want) {
		t.Fatalf("unmanaged OMP sources: %v", got)
	}
	for path, names := range want {
		if got[path] != names {
			t.Fatalf("%s: %q != %q", path, got[path], names)
		}
	}
	candidates, err := e.service.Import(ImportRequest{From: "omp"})
	must(t, err)
	if len(candidates) != 1 || candidates[0].Name != "mine" || !strings.Contains(strings.Join(candidates[0].Warnings, "\n"), "keeps loading") {
		t.Fatalf("import must offer extensions only: %+v", candidates)
	}
}
