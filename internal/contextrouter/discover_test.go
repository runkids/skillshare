package contextrouter

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestDiscover(t *testing.T) {
	// Resolved, since macOS temp dirs sit behind a symlink.
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(base, "home")
	at := func(parts ...string) string { return filepath.Join(append([]string{home}, parts...)...) }
	mkdir := func(dir string) string {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	route := func(dir string) string {
		writeFile(t, filepath.Join(dir, "wiki", "ai-context.json"), `{"topics":{}}`)
		return dir
	}
	check := func(name string, in DiscoverInput, want ...Root) {
		t.Helper()
		if want == nil {
			want = []Root{}
		}
		in.Home = home
		if got := Discover(in); !reflect.DeepEqual(got, want) {
			t.Errorf("%s:\n got  %+v\n want %+v", name, got, want)
		}
	}

	vault := route(at("vault"))
	skills := mkdir(at("vault", "skillshare", "skills"))
	agents := mkdir(at("vault", "skillshare", "agents"))
	plain := mkdir(at("plain", "skills"))
	check("skills source two levels below", DiscoverInput{SkillsSource: skills},
		Root{vault, "vault", OriginSkillsSource})
	check("agents source alone", DiscoverInput{SkillsSource: plain, AgentsSource: agents},
		Root{vault, "vault", OriginAgentsSource})
	check("both sources, one router", DiscoverInput{SkillsSource: skills, AgentsSource: agents},
		Root{vault, "vault", OriginSkillsSource})
	check("no router", DiscoverInput{SkillsSource: plain, AgentsSource: plain})
	check("missing source", DiscoverInput{SkillsSource: at("gone", "skills")})

	// The source itself is depth 0, and the nearest router wins.
	inner := route(mkdir(at("vault", "inner", "skills")))
	check("router in the source itself", DiscoverInput{SkillsSource: inner},
		Root{inner, "skills", OriginSkillsSource})

	deep := route(at("deep"))
	check("four folders above", DiscoverInput{SkillsSource: mkdir(at("deep", "1", "2", "3", "4"))},
		Root{deep, "deep", OriginSkillsSource})
	check("five folders above", DiscoverInput{SkillsSource: mkdir(at("deep", "1", "2", "3", "4", "5"))})

	// A router in the home folder itself is never picked up from a source.
	route(home)
	check("home is not searched", DiscoverInput{SkillsSource: plain})
	if err := os.RemoveAll(filepath.Join(home, "wiki")); err != nil {
		t.Fatal(err)
	}

	zed := route(at("work", "b"))
	app := route(at("work", "a"))
	projects := []Project{{Path: zed, Name: "zed"}, {Path: app}, {Path: at("work", "gone")}, {Path: mkdir(at("work", "unrouted"))}}
	check("projects by name after the source", DiscoverInput{SkillsSource: skills, Projects: projects},
		Root{vault, "vault", OriginSkillsSource}, Root{app, "a", OriginProject}, Root{zed, "zed", OriginProject})
	check("project mode uses only the project", DiscoverInput{ProjectRoot: app, SkillsSource: skills, Projects: projects},
		Root{app, "a", OriginCurrent})
	check("project mode without a router", DiscoverInput{ProjectRoot: plain, SkillsSource: skills, Projects: projects})

	// A config that cannot be used is still found, so it can be reported.
	odd := mkdir(at("odd"))
	mkdir(filepath.Join(odd, "wiki", "ai-context.json"))
	check("config is a directory", DiscoverInput{Projects: []Project{{Path: odd}}}, Root{odd, "odd", OriginProject})

	if runtime.GOOS == "windows" {
		return
	}
	dangling := mkdir(at("dangling"))
	mkdir(filepath.Join(dangling, "wiki"))
	if err := os.Symlink(at("nowhere.json"), filepath.Join(dangling, "wiki", "ai-context.json")); err != nil {
		t.Fatal(err)
	}
	check("config is a dangling link", DiscoverInput{Projects: []Project{{Path: dangling}}}, Root{dangling, "dangling", OriginProject})

	// A symlinked source is walked from where it really is.
	link := at("link-to-skills")
	if err := os.Symlink(skills, link); err != nil {
		t.Fatal(err)
	}
	check("symlinked source", DiscoverInput{SkillsSource: link}, Root{vault, "vault", OriginSkillsSource})

	// The same folder reached as a project through a link is one root.
	alias := at("vault-alias")
	if err := os.Symlink(vault, alias); err != nil {
		t.Fatal(err)
	}
	check("project is the source router", DiscoverInput{SkillsSource: skills, Projects: []Project{{Path: alias, Name: "alias"}}},
		Root{vault, "vault", OriginSkillsSource})
}
