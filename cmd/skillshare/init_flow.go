package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"skillshare/internal/theme"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
)

const (
	startNew     = "new"
	startConnect = "connect"
)

// printInitBanner shows the logo with what init found on this machine.
func printInitBanner(detected []detectedDir, animate bool) {
	var lines []string
	switch n := toolCount(detected); n {
	case 0:
		lines = append(lines, "No AI tools found yet")
	case 1:
		lines = append(lines, "Found 1 AI tool")
	default:
		lines = append(lines, fmt.Sprintf("Found %d AI tools", n))
	}
	if total, names := skillsOnMachine(detected); total > 0 {
		lines = append(lines, theme.Dim().Render(fmt.Sprintf("%s already in %s", plural(total, "skill"), strings.Join(names, ", "))))
	}
	fmt.Println()
	ui.LogoBanner(version, lines, animate)
	fmt.Println()
}

func skillsOnMachine(detected []detectedDir) (int, []string) {
	total := 0
	var names []string
	for _, d := range withSkills(detected) {
		total += d.skillCount
		names = append(names, d.name)
	}
	return total, names
}

// askInitPlan fills the plan from the user's answers. Questions a flag
// already answered are skipped.
func askInitPlan(p *initPlan, opts *initOptions, detected []detectedDir) error {
	start := startNew
	if opts.remoteURL == "" {
		choice, err := ui.Select("How do you want to start?", []ui.Option{
			{Label: "New setup on this machine", Value: startNew},
			{Label: "Connect my existing skillshare repo", Value: startConnect},
		}, startNew)
		if err != nil {
			return err
		}
		start = choice
		if start == startNew {
			ui.Answered("Start", "New setup on this machine")
		} else {
			ui.Answered("Start", "Connect my existing skillshare repo")
		}
	}
	if start == startConnect {
		return askConnect(p, opts, detected)
	}

	if err := askTools(p, opts, detected); err != nil {
		return err
	}
	if err := askImport(p, opts, detected); err != nil {
		return err
	}
	if !opts.noGit && !opts.initGit && opts.remoteURL == "" {
		on, err := ui.Confirm("Keep history with git? (undo deletes, sync machines later)", true)
		if err != nil {
			return err
		}
		p.git = on
		ui.Answered("Git", onOff(on))
	}
	if p.git {
		if err := askRemote(p, opts); err != nil {
			return err
		}
	}
	if opts.gitRootScope == "" {
		p.gitScope = p.defaultGitScope()
	}
	return nil
}

// askConnect is the second-machine path: the repo decides the layout.
func askConnect(p *initPlan, opts *initOptions, detected []detectedDir) error {
	p.connect = true
	for {
		url, err := ui.Input("Your skillshare repo", "git@github.com:you/skills.git", "")
		if err != nil {
			return err
		}
		url = strings.TrimSpace(url)
		if url == "" {
			continue
		}
		r := checkRemote(url)
		if !r.reachable {
			ui.Warning("Can't reach %s: %s", url, r.reason)
			continue
		}
		p.remoteURL, p.remote, p.git, p.pull = url, r, true, r.skills > 0
		ui.Answered("Repo", url+" "+theme.Dim().Render("· "+describeRemote(r)))
		break
	}
	applyRemoteLayout(p, opts)

	if !opts.noCopy && opts.copyFrom == "" {
		if err := askKeepLocal(p, detected); err != nil {
			return err
		}
	}
	return askTools(p, opts, detected)
}

// askKeepLocal offers this machine's skills that the repo doesn't have.
// Same-name skills are left out: the repo version is used.
func askKeepLocal(p *initPlan, detected []detectedDir) error {
	all, _ := collectImports(withSkills(detected))
	inRepo := map[string]bool{}
	for _, n := range p.remote.names {
		inRepo[n] = true
	}
	var local []importedSkill
	same := 0
	for _, s := range all {
		if inRepo[s.name] {
			same++
			continue
		}
		local = append(local, s)
	}
	if same > 0 {
		note := fmt.Sprintf("%d of your skills share a name with the repo — the repo version is used.", same)
		if same == 1 {
			note = "1 of your skills shares a name with the repo — the repo version is used."
		}
		fmt.Println("  " + theme.Dim().Render(note))
	}
	p.imports = nil
	if len(local) == 0 {
		return nil
	}
	options := make([]ui.Option, len(local))
	values := make([]string, len(local))
	for i, s := range local {
		options[i] = ui.Option{Label: fmt.Sprintf("%-14s %s", s.name, theme.Dim().Render(s.tool)), Value: s.from}
		values[i] = s.from
	}
	title := fmt.Sprintf("%s on this machine aren't in the repo. Keep them?", plural(len(local), "skill"))
	if len(local) == 1 {
		title = "1 skill on this machine isn't in the repo. Keep it?"
	}
	keep, err := ui.MultiSelect(title, options, values)
	if err != nil {
		return err
	}
	kept := map[string]bool{}
	for _, v := range keep {
		kept[v] = true
	}
	for _, s := range local {
		if kept[s.from] {
			p.imports = append(p.imports, s)
		}
	}
	ui.Answered("Local", fmt.Sprintf("keep %d %s", len(p.imports), theme.Dim().Render("(added to the repo on next push)")))
	return nil
}

func askTools(p *initPlan, opts *initOptions, detected []detectedDir) error {
	if opts.noTargets || opts.targetsArg != "" || len(detected) == 0 {
		return nil
	}
	options := make([]ui.Option, len(detected))
	for i, d := range detected {
		options[i] = ui.Option{Label: fmt.Sprintf("%-14s %s", d.name, theme.Dim().Render(toolStatus(d))), Value: d.name}
	}
	chosen, err := ui.MultiSelect("Sync skills to which tools?", options, p.targets)
	if err != nil {
		return err
	}
	p.targets = chosen
	ui.Answered("Targets", describeTools(chosen))
	return nil
}

func askImport(p *initPlan, opts *initOptions, detected []detectedDir) error {
	tools := withSkills(detected)
	if opts.noCopy || opts.copyFrom != "" || len(tools) == 0 {
		return nil
	}
	total := len(p.imports)
	var options []ui.Option
	if len(tools) == 1 {
		options = append(options, ui.Option{Label: fmt.Sprintf("Import all %d from %s", total, tools[0].name), Value: "all"})
	} else {
		var parts []string
		for _, d := range tools {
			parts = append(parts, fmt.Sprintf("%s %d", d.name, d.skillCount))
		}
		options = append(options, ui.Option{Label: fmt.Sprintf("Import all %d  %s", total, theme.Dim().Render("("+strings.Join(parts, " · ")+")")), Value: "all"})
		for _, d := range tools {
			options = append(options, ui.Option{Label: fmt.Sprintf("Only from %s (%d)", d.name, d.skillCount), Value: d.name})
		}
	}
	options = append(options, ui.Option{Label: "Start empty", Value: "none"})

	choice, err := ui.Select(fmt.Sprintf("You already have %s. Bring them in?", plural(total, "skill")), options, "all")
	if err != nil {
		return err
	}
	switch choice {
	case "all":
		ui.Answered("Import", fmt.Sprintf("all %d", total))
	case "none":
		p.imports, p.dupes, p.importAll = nil, nil, false
		ui.Answered("Import", "none")
	default:
		for _, d := range tools {
			if d.name == choice {
				p.imports, p.dupes = collectImports([]detectedDir{d})
			}
		}
		p.importAll = false
		ui.Answered("Import", fmt.Sprintf("only %s (%d)", choice, len(p.imports)))
	}
	return nil
}

// askRemote asks for an optional remote; when the repo already has skills,
// it asks before bringing them here.
func askRemote(p *initPlan, opts *initOptions) error {
	url := opts.remoteURL
	for url == "" {
		answer, err := ui.Input("Link a remote repo to sync other machines (optional)", "git@github.com:you/skills.git  ·  Enter to skip", "")
		if err != nil {
			return err
		}
		answer = strings.TrimSpace(answer)
		if answer == "" {
			ui.Answered("Remote", "skip "+theme.Dim().Render("(add later: skillshare init --remote <url>)"))
			return nil
		}
		r := checkRemote(answer)
		if !r.reachable {
			ui.Warning("Can't reach %s: %s", answer, r.reason)
			choice, err := ui.Select("What now?", []ui.Option{
				{Label: "Enter another URL", Value: "retry"},
				{Label: "Link it anyway (push and pull retry later)", Value: "link"},
				{Label: "Skip", Value: "skip"},
			}, "retry")
			if err != nil {
				return err
			}
			if choice == "skip" {
				ui.Answered("Remote", "skip")
				return nil
			}
			if choice == "retry" {
				continue
			}
		}
		url, p.remote = answer, r
	}
	p.remoteURL = url
	if p.remote == nil {
		p.remote = checkRemote(url)
	}
	if p.remote.hasSkills() {
		pull, err := ui.Confirm(fmt.Sprintf("This repo already has %s. Bring them to this machine? (same name: repo version wins)", plural(p.remote.skills, "skill")), true)
		if err != nil {
			return err
		}
		p.pull = pull
	}
	if p.pull {
		applyRemoteLayout(p, opts)
		ui.Answered("Remote", url+" "+theme.Dim().Render(fmt.Sprintf("· bring %s here", plural(p.remote.skills, "skill"))))
	} else {
		ui.Answered("Remote", url)
	}
	return nil
}

// checkRemote inspects a repo behind a spinner.
func checkRemote(url string) *remoteRepo {
	spinner := ui.StartSpinner("Checking " + url + "…")
	r := inspectRemote(url)
	spinner.Stop()
	return r
}

// applyRemoteLayout matches the local layout to the repo being pulled: a
// whole-folder repo is versioned at the root, a skills/ repo becomes the
// source subfolder.
func applyRemoteLayout(p *initPlan, opts *initOptions) {
	if p.remote == nil || !p.remote.reachable {
		return
	}
	switch {
	case p.remote.rootScope:
		p.gitScope = "root"
	case p.remote.subdir != "" && opts.subdir == "":
		p.subdir = p.remote.subdir
		p.gitScope = "skills"
	default:
		if opts.gitRootScope == "" {
			p.gitScope = "skills"
		}
	}
}

// resolveHeadlessRemote checks --remote without prompting: a repo with
// skills is pulled (same name: repo version wins).
func resolveHeadlessRemote(p *initPlan, opts *initOptions) {
	if p.remoteURL == "" || !p.git {
		return
	}
	p.remote = checkRemote(p.remoteURL)
	p.pull = p.remote.hasSkills()
	if p.pull {
		applyRemoteLayout(p, opts)
	}
}

// printHeadlessPlan prints one line per decision with the flag that changes it.
func printHeadlessPlan(p *initPlan) {
	hint := func(s string) string { return theme.Dim().Render("(" + s + ")") }
	ui.Answered("Source", utils.FoldHomePath(p.source())+" "+hint("--source, --subdir"))
	ui.Answered("Targets", describeTools(p.targets)+" "+hint("--targets, --no-targets"))
	ui.Answered("Import", describeImports(p)+" "+hint("--copy-from, --no-copy"))
	ui.Answered("Git", describeGit(p)+" "+hint("--no-git, --git-root"))
	remote := "none"
	if p.remoteURL != "" {
		remote = p.remoteURL
		switch {
		case p.remote != nil && !p.remote.reachable:
			remote += " · unreachable: " + p.remote.reason
		case p.pull:
			remote += fmt.Sprintf(" · bring %s here", plural(p.remote.skills, "skill"))
		}
	}
	ui.Answered("Remote", remote+" "+hint("--remote <url>"))
	if same := p.sameNameAsRemote(); len(same) > 0 {
		fmt.Printf("  repo version used for %s\n", strings.Join(same, ", "))
	}
	skill := "install skillshare"
	if !p.skill {
		skill = "skip"
	}
	ui.Answered("Skill", skill+" "+hint("--skill, --no-skill"))
	ui.Answered("Sync", p.mode+" "+hint("--mode"))
	fmt.Println()
}

var syncModeNotes = map[string]string{
	"merge":   "link each skill",
	"copy":    "real files",
	"symlink": "link the whole folder",
}

// summaryLines renders the "Ready to set up" block; changed marks the row
// the user just edited.
func summaryLines(p *initPlan, title, changed string) []string {
	dim := theme.Dim()
	mark := func(key string) string {
		if key != "" && key == changed {
			return " " + theme.Accent().Render("• changed")
		}
		return ""
	}
	lines := []string{"", theme.Primary().Bold(true).Render(title)}
	row := func(label, value, key string) {
		lines = append(lines, fmt.Sprintf("  %-8s %s%s", label, value, mark(key)))
	}
	row("Source", utils.FoldHomePath(p.source()), "source")
	row("Sync", p.mode+" "+dim.Render("("+syncModeNotes[p.mode]+")"), "sync")
	row("Targets", describeTools(p.targets), "")
	if !p.connect {
		row("Import", describeImports(p), "")
	}
	switch {
	case !p.git:
		row("Git", "off "+dim.Render("(deleted skills can't be restored)"), "git")
	case p.gitScope == "root":
		row("Git", "skills, agents, extras", "git")
		lines = append(lines, fmt.Sprintf("  %-8s %s", "", dim.Render("plugins, MCP and hooks stay in each machine's config.yaml")))
	default:
		row("Git", p.gitScope+" only", "git")
	}
	if p.remoteURL != "" {
		if p.pull {
			row("Remote", p.remoteURL+" "+dim.Render(fmt.Sprintf("(%s → this machine)", plural(p.remote.skills, "skill"))), "")
		} else {
			row("Remote", p.remoteURL+" "+dim.Render("(link only)"), "")
		}
	}
	if p.skill {
		row("Skill", "skillshare "+dim.Render("(lets your AI run skillshare)"), "")
	}
	return append(lines, "")
}

// confirmPlan shows the summary until the user sets it up or cancels.
// Returns false when cancelled.
func confirmPlan(p *initPlan, home string) (bool, error) {
	changed := ""
	for {
		lines := summaryLines(p, "Ready to set up", changed)
		fmt.Println(strings.Join(lines, "\n"))
		action, err := ui.Select("Set up now?", []ui.Option{
			{Label: "Yes, set it up", Value: "yes"},
			{Label: "Change settings", Value: "change"},
			{Label: "Cancel", Value: "cancel"},
		}, "yes")
		if err != nil || action == "cancel" {
			return false, nilIfCancelled(err)
		}
		if action == "yes" {
			return true, nil
		}
		if changed, err = changeSetting(p, home); err != nil {
			return false, nilIfCancelled(err)
		}
		// Redraw the summary in place.
		fmt.Printf("\x1b[%dA\x1b[J", len(lines))
	}
}

func changeSetting(p *initPlan, home string) (string, error) {
	options := []ui.Option{
		{Label: "Source   " + utils.FoldHomePath(p.source()), Value: "source"},
		{Label: "Sync     " + p.mode, Value: "sync"},
	}
	// A pulled repo decides what git versions.
	if !p.pull {
		options = append(options, ui.Option{Label: "Git      " + describeGit(p), Value: "git"})
	}
	options = append(options, ui.Option{Label: "Back", Value: "back"})
	what, err := ui.Select("Change which setting?", options, "source")
	if err != nil {
		return "", err
	}
	switch what {
	case "source":
		v, err := ui.Input("Where to keep your skills", "", utils.FoldHomePath(p.source()))
		if err != nil {
			return "", err
		}
		if v = strings.TrimSpace(v); v != "" {
			if utils.HasTildePrefix(v) {
				v = filepath.Join(home, v[1:])
			}
			if abs, err := filepath.Abs(v); err == nil {
				p.base, p.subdir = abs, ""
			}
		}
	case "sync":
		v, err := ui.Select("How should skills reach your tools?", []ui.Option{
			{Label: "merge     link each skill  " + theme.Dim().Render("(default)"), Value: "merge"},
			{Label: "copy      real files  " + theme.Dim().Render("· for tools that can't follow links"), Value: "copy"},
			{Label: "symlink   link the whole folder", Value: "symlink"},
		}, p.mode)
		if err != nil {
			return "", err
		}
		p.mode = v
	case "git":
		current := p.gitScope
		if !p.git {
			current = "off"
		}
		v, err := ui.Select("What should git keep history of?", []ui.Option{
			{Label: "skills, agents, extras", Value: "root"},
			{Label: "skills only", Value: "skills"},
			{Label: "off", Value: "off"},
		}, current)
		if err != nil {
			return "", err
		}
		p.git = v != "off"
		if p.git {
			p.gitScope = v
		} else {
			p.remoteURL, p.remote, p.pull = "", nil, false
		}
	default:
		return "", nil
	}
	return what, nil
}

func nilIfCancelled(err error) error {
	if err == ui.ErrCancelled {
		return nil
	}
	return err
}

func toolStatus(d detectedDir) string {
	switch {
	case d.name == "universal" && len(d.via) > 0:
		return "shared folder · " + strings.Join(d.via, ", ")
	case d.name == "universal":
		return "shared folder"
	case d.skillCount > 0:
		return plural(d.skillCount, "skill")
	case d.exists:
		return "empty"
	default:
		return "not set up yet"
	}
}

func describeTools(names []string) string {
	switch len(names) {
	case 0:
		return "none"
	case 1, 2, 3:
		return strings.Join(names, ", ")
	default:
		return fmt.Sprintf("%d targets (%s, …)", len(names), strings.Join(names[:3], ", "))
	}
}

func describeImports(p *initPlan) string {
	if len(p.imports) == 0 {
		return "none"
	}
	if p.importAll {
		return fmt.Sprintf("all %d", len(p.imports))
	}
	return fmt.Sprintf("%d from %s", len(p.imports), p.imports[0].tool)
}

func describeGit(p *initPlan) string {
	switch {
	case !p.git:
		return "off"
	case p.gitScope == "root":
		return "skills, agents, extras"
	default:
		return p.gitScope + " only"
	}
}

func describeRemote(r *remoteRepo) string {
	parts := []string{plural(r.skills, "skill")}
	if r.agents > 0 {
		parts = append(parts, plural(r.agents, "agent"))
	}
	if r.extras > 0 {
		parts = append(parts, plural(r.extras, "extra"))
	}
	s := strings.Join(parts, ", ")
	if r.subdir != "" {
		s += " (in " + r.subdir + "/)"
	}
	return s
}

func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %s", n, pluralNoun(word))
}

// pluralNoun is the plural of a simple English noun: "category" becomes
// "categories", "day" stays regular.
func pluralNoun(word string) string {
	if n := len(word); n > 1 && word[n-1] == 'y' && !strings.ContainsRune("aeiou", rune(word[n-2])) {
		return word[:n-1] + "ies"
	}
	return word + "s"
}

// desktopAppHint returns how to get the desktop app on this platform, or
// "" when there is no build for it or it is already installed.
func desktopAppHint() string {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "darwin/arm64":
		home, _ := os.UserHomeDir()
		for _, dir := range []string{"/Applications", filepath.Join(home, "Applications")} {
			if _, err := os.Stat(filepath.Join(dir, "skillshare.app")); err == nil {
				return ""
			}
		}
		return "brew install --cask runkids/tap/skillshare-app"
	case "windows/amd64", "linux/amd64":
		return "https://skillshare.runkids.cc/docs/getting-started/desktop-app"
	}
	return ""
}
