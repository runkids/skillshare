package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/git"
	"skillshare/internal/install"
	"skillshare/internal/ui"
)

// checkRepoResult holds the check result for a tracked repo
type checkRepoResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "up_to_date", "behind", "dirty", "error"
	Behind  int    `json:"behind"`
	Message string `json:"message,omitempty"`
}

// checkSkillResult holds the check result for a regular skill
type checkSkillResult struct {
	Name        string `json:"name"`
	Source      string `json:"source"`
	Version     string `json:"version"`
	Status      string `json:"status"` // "up_to_date", "update_available", "local", "error"
	InstalledAt string `json:"installed_at,omitempty"`
}

// checkOutput is the JSON output structure
type checkOutput struct {
	TrackedRepos []checkRepoResult  `json:"tracked_repos"`
	Skills       []checkSkillResult `json:"skills"`
}

func cmdCheck(args []string) error {
	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}

	if mode == modeAuto {
		if projectConfigExists(cwd) {
			mode = modeProject
		} else {
			mode = modeGlobal
		}
	}

	applyModeLabel(mode)

	var jsonOutput bool
	for _, arg := range rest {
		switch {
		case arg == "--json":
			jsonOutput = true
		case arg == "--help" || arg == "-h":
			printCheckHelp()
			return nil
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unknown option: %s", arg)
		default:
			return fmt.Errorf("unexpected argument: %s", arg)
		}
	}

	if mode == modeProject {
		return cmdCheckProject(cwd, jsonOutput)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	return runCheck(cfg.Source, jsonOutput)
}

func runCheck(sourceDir string, jsonOutput bool) error {
	repos, err := install.GetTrackedRepos(sourceDir)
	if err != nil {
		repos = nil // Non-fatal: source dir might not exist yet
	}

	skills, err := install.GetUpdatableSkills(sourceDir)
	if err != nil {
		skills = nil
	}

	if len(repos) == 0 && len(skills) == 0 {
		if jsonOutput {
			out, _ := json.MarshalIndent(checkOutput{
				TrackedRepos: []checkRepoResult{},
				Skills:       []checkSkillResult{},
			}, "", "  ")
			fmt.Println(string(out))
			return nil
		}
		ui.Header(ui.WithModeLabel("Checking for updates"))
		ui.Info("No tracked repositories or updatable skills found")
		ui.Info("Use 'skillshare install <repo> --track' to add a tracked repository")
		return nil
	}

	if !jsonOutput {
		ui.Header(ui.WithModeLabel("Checking for updates"))
		ui.StepStart("Source", sourceDir)
		ui.StepContinue("Items", fmt.Sprintf("%d tracked repo(s), %d skill(s)", len(repos), len(skills)))
	}

	var repoResults []checkRepoResult
	var skillResults []checkSkillResult

	// Check tracked repos
	if len(repos) > 0 {
		if !jsonOutput {
			fmt.Println()
			spinner := ui.StartSpinner(fmt.Sprintf("Fetching updates for %d tracked repo(s)...", len(repos)))

			for _, repo := range repos {
				repoPath := filepath.Join(sourceDir, repo)
				result := checkTrackedRepo(repo, repoPath)
				repoResults = append(repoResults, result)
			}

			spinner.Success(fmt.Sprintf("Checked %d tracked repo(s)", len(repos)))
			fmt.Println()

			for _, r := range repoResults {
				switch r.Status {
				case "up_to_date":
					ui.ListItem("success", r.Name, "up to date")
				case "behind":
					ui.ListItem("info", r.Name, fmt.Sprintf("%d commit(s) behind", r.Behind))
				case "dirty":
					ui.ListItem("warning", r.Name, "has uncommitted changes")
				case "error":
					ui.ListItem("error", r.Name, fmt.Sprintf("error: %s", r.Message))
				}
			}
		} else {
			for _, repo := range repos {
				repoPath := filepath.Join(sourceDir, repo)
				result := checkTrackedRepo(repo, repoPath)
				repoResults = append(repoResults, result)
			}
		}
	}

	// Check regular skills
	if len(skills) > 0 {
		if !jsonOutput {
			fmt.Println()
			spinner := ui.StartSpinner(fmt.Sprintf("Checking %d installed skill(s)...", len(skills)))

			for _, skill := range skills {
				skillPath := filepath.Join(sourceDir, skill)
				result := checkRegularSkill(skill, skillPath)
				skillResults = append(skillResults, result)
			}

			spinner.Success(fmt.Sprintf("Checked %d installed skill(s)", len(skills)))
			fmt.Println()

			for _, s := range skillResults {
				switch s.Status {
				case "up_to_date":
					detail := "up to date"
					if s.Source != "" {
						detail += fmt.Sprintf("  %s", formatSourceShort(s.Source))
					}
					ui.ListItem("success", s.Name, detail)
				case "update_available":
					detail := "update available"
					if s.Source != "" {
						detail += fmt.Sprintf("  %s", formatSourceShort(s.Source))
					}
					ui.ListItem("info", s.Name, detail)
				case "local":
					ui.ListItem("info", s.Name, "local source")
				case "error":
					ui.ListItem("warning", s.Name, "cannot reach remote")
				}
			}
		} else {
			for _, skill := range skills {
				skillPath := filepath.Join(sourceDir, skill)
				result := checkRegularSkill(skill, skillPath)
				skillResults = append(skillResults, result)
			}
		}
	}

	if jsonOutput {
		output := checkOutput{
			TrackedRepos: repoResults,
			Skills:       skillResults,
		}
		if output.TrackedRepos == nil {
			output.TrackedRepos = []checkRepoResult{}
		}
		if output.Skills == nil {
			output.Skills = []checkSkillResult{}
		}
		out, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(out))
		return nil
	}

	// Summary
	updatableRepos := 0
	for _, r := range repoResults {
		if r.Status == "behind" {
			updatableRepos++
		}
	}
	updatableSkills := 0
	for _, s := range skillResults {
		if s.Status == "update_available" {
			updatableSkills++
		}
	}

	fmt.Println()
	if updatableRepos+updatableSkills == 0 {
		ui.SuccessMsg("Everything is up to date")
	} else {
		parts := []string{}
		if updatableRepos > 0 {
			parts = append(parts, fmt.Sprintf("%d repo(s)", updatableRepos))
		}
		if updatableSkills > 0 {
			parts = append(parts, fmt.Sprintf("%d skill(s)", updatableSkills))
		}
		ui.Info("%s have updates available", strings.Join(parts, " + "))
		ui.Info("Run 'skillshare update <name>' or 'skillshare update --all'")
	}

	return nil
}

func checkTrackedRepo(name, repoPath string) checkRepoResult {
	result := checkRepoResult{Name: name}

	// Check for uncommitted changes
	if isDirty, _ := git.IsDirty(repoPath); isDirty {
		result.Status = "dirty"
		result.Message = "has uncommitted changes"
		return result
	}

	// Fetch and compare
	behind, err := git.GetBehindCount(repoPath)
	if err != nil {
		result.Status = "error"
		result.Message = err.Error()
		return result
	}

	if behind == 0 {
		result.Status = "up_to_date"
	} else {
		result.Status = "behind"
		result.Behind = behind
	}

	return result
}

func checkRegularSkill(name, skillPath string) checkSkillResult {
	result := checkSkillResult{Name: name}

	meta, err := install.ReadMeta(skillPath)
	if err != nil || meta == nil {
		result.Status = "local"
		return result
	}

	result.Source = meta.Source
	result.Version = meta.Version
	if !meta.InstalledAt.IsZero() {
		result.InstalledAt = meta.InstalledAt.Format("2006-01-02")
	}

	// If no repo URL, it's a local source
	if meta.RepoURL == "" {
		result.Status = "local"
		return result
	}

	// Compare with remote
	remoteHash, err := git.GetRemoteHeadHash(meta.RepoURL)
	if err != nil {
		result.Status = "error"
		return result
	}

	if meta.Version == remoteHash {
		result.Status = "up_to_date"
	} else {
		result.Status = "update_available"
	}

	return result
}

// formatSourceShort returns a shortened source for display
func formatSourceShort(source string) string {
	// Remove common prefixes for shorter display
	source = strings.TrimPrefix(source, "https://")
	source = strings.TrimPrefix(source, "http://")
	source = strings.TrimSuffix(source, ".git")
	return source
}

func printCheckHelp() {
	fmt.Println(`Usage: skillshare check [options]

Check for available updates to tracked repositories and installed skills.

For tracked repos: fetches from origin and checks if behind
For regular skills: compares installed version with remote HEAD

Options:
  --project, -p  Check project-level skills (.skillshare/)
  --global, -g   Check global skills (~/.config/skillshare)
  --json         Output results as JSON
  --help, -h     Show this help

Examples:
  skillshare check               Check for updates
  skillshare check --json        Output as JSON (for CI)
  skillshare check -p            Check project skills`)
}
