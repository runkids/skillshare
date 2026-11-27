package plugin

import (
	"net/url"
	"path"
	"strings"

	"skillshare/internal/install"
)

// githubPluginLink recognizes web links separately from Git clone URLs. Anchors
// and display options belong to the GitHub page, not the installed source.
func githubPluginLink(source string) (*url.URL, string) {
	u, err := url.Parse(source)
	if err != nil || u.Scheme != "https" || !strings.EqualFold(u.Host, "github.com") || u.User != nil {
		return nil, ""
	}
	parts := strings.SplitN(strings.Trim(u.Path, "/"), "/", 4)
	if len(parts) != 4 || (parts[2] != "tree" && parts[2] != "blob") {
		return nil, ""
	}
	u.Host, u.RawQuery, u.Fragment, u.RawFragment, u.ForceQuery = "github.com", "", "", "", false
	u.Path = strings.TrimRight(u.Path, "/")
	return u, parts[2]
}

func herdrModName(u *url.URL) string {
	dir := path.Dir(u.Path)
	name := path.Base(dir)
	if path.Base(u.Path) == "main.js" && path.Base(path.Dir(dir)) == "mods" && namePattern.MatchString(name) {
		return name
	}
	return ""
}

var errHerdrModLink = agentError{
	key:     "plugins.error.herdrModLink",
	message: "Use a GitHub folder link or a Herdr mod's mods/<name>/main.js link. For a mod, leave the launcher entry empty.",
}

func resolveGitHubPluginLink(link *url.URL, kind string) (*install.Source, error) {
	name := herdrModName(link)
	if kind == "blob" && name == "" {
		return nil, errHerdrModLink
	}
	source, err := install.ParseSource(link.String())
	if err != nil {
		return nil, err
	}
	if err := (&install.RemoteRefs{}).Resolve(source); err != nil {
		return nil, err
	}
	if kind == "blob" {
		file := "mods/" + name + "/main.js"
		switch {
		case source.Subdir == file:
			source.Subdir = ""
		case strings.HasSuffix(source.Subdir, "/"+file):
			source.Subdir = strings.TrimSuffix(source.Subdir, "/"+file)
		default:
			return nil, errHerdrModLink
		}
	}
	return source, nil
}

func pluginFolderError(dir string) error {
	return agentError{
		key:     "plugins.error.sourceFolder",
		message: "Plugin folder " + dir + " is not available in this revision. Open the package folder on GitHub and copy its link.",
		args:    map[string]string{"folder": dir},
	}
}
