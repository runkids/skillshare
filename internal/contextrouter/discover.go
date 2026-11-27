package contextrouter

import (
	"os"
	"path/filepath"
	"sort"
)

// Origin says why a folder was looked at.
type Origin string

const (
	OriginCurrent      Origin = "current"       // the project the dashboard runs in
	OriginSkillsSource Origin = "skills_source" // the skills source or a folder above it
	OriginAgentsSource Origin = "agents_source" // the agents source or a folder above it
	OriginProject      Origin = "project"       // a registered project
)

// Root is a folder that has a router config.
type Root struct {
	Path   string `json:"path"` // physical path
	Name   string `json:"name"`
	Origin Origin `json:"origin"`
}

// Project is a registered project folder. Name may be empty.
type Project struct {
	Path string
	Name string
}

type DiscoverInput struct {
	// ProjectRoot is set in project mode; nothing else is used then.
	ProjectRoot  string
	SkillsSource string
	AgentsSource string
	Projects     []Project
	Home         string
}

// maxParents is how far above a source folder the search goes.
const maxParents = 4

// hasConfig reports whether dir has something at wiki/ai-context.json. A
// config that cannot be used still counts: it is reported, not hidden.
func hasConfig(dir string) bool {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return false
	}
	defer root.Close()
	_, err = root.Lstat(ConfigRel)
	return err == nil
}

func physical(path string) string {
	if path == "" {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return filepath.Clean(path)
}

// aroundSource returns the nearest folder with a config among source and up
// to maxParents folders above it, never the home folder or a volume root.
func aroundSource(source, home string) string {
	dir := physical(source)
	for depth := 0; dir != "" && depth <= maxParents; depth++ {
		if dir == home || filepath.Dir(dir) == dir {
			return ""
		}
		if hasConfig(dir) {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	return ""
}

// Discover lists the folders with a router config: the project in project
// mode; otherwise the folders around the two sources, then the registered
// projects by name. A folder reached twice keeps its first origin.
func Discover(in DiscoverInput) []Root {
	roots := []Root{}
	seen := map[string]bool{}
	add := func(path, name string, origin Origin) {
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		if name == "" {
			name = filepath.Base(path)
		}
		roots = append(roots, Root{Path: path, Name: name, Origin: origin})
	}
	if in.ProjectRoot != "" {
		if dir := physical(in.ProjectRoot); hasConfig(dir) {
			add(dir, "", OriginCurrent)
		}
		return roots
	}
	home := physical(in.Home)
	add(aroundSource(in.SkillsSource, home), "", OriginSkillsSource)
	add(aroundSource(in.AgentsSource, home), "", OriginAgentsSource)

	projects := make([]Project, 0, len(in.Projects))
	for _, p := range in.Projects {
		dir := physical(p.Path)
		if !hasConfig(dir) {
			continue
		}
		if p.Name == "" {
			p.Name = filepath.Base(dir)
		}
		projects = append(projects, Project{Path: dir, Name: p.Name})
	}
	sort.Slice(projects, func(i, j int) bool {
		if projects[i].Name != projects[j].Name {
			return projects[i].Name < projects[j].Name
		}
		return projects[i].Path < projects[j].Path
	})
	for _, p := range projects {
		add(p.Path, p.Name, OriginProject)
	}
	return roots
}
