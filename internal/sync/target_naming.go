package sync

import (
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"skillshare/internal/config"
	"skillshare/internal/skillpkg"
	"skillshare/internal/utils"
)

// ResolvedTargetSkill represents a discovered skill together with the target
// entry name that should be used for a specific target.
type ResolvedTargetSkill struct {
	Skill      DiscoveredSkill
	TargetName string
	SkillName  string
}

// TargetSkillResolution contains the resolved target-visible skills for one
// target after include/exclude filters, target filters, standard validation,
// and collision handling are applied.
type TargetSkillResolution struct {
	Naming            string
	Skills            []ResolvedTargetSkill
	Warnings          []string
	Collisions        []NameCollision
	UnmatchedIncludes []UnmatchedInclude
}

// UnmatchedIncludeWarnings renders one diagnostic per include pattern that
// selects no skill, pointing at the source-path filter that would have.
func (r *TargetSkillResolution) UnmatchedIncludeWarnings() []string {
	if len(r.UnmatchedIncludes) == 0 {
		return nil
	}

	warnings := make([]string, 0, len(r.UnmatchedIncludes))
	for _, unmatched := range r.UnmatchedIncludes {
		warnings = append(warnings, unmatched.Warning())
	}
	return warnings
}

// Warning describes the pattern for the CLI.
func (u UnmatchedInclude) Warning() string {
	message := fmt.Sprintf("include filter %q matches no skill in the source, so it adds nothing to this target", u.Pattern)
	if len(u.Suggestions) > 0 {
		quoted := make([]string, 0, len(u.Suggestions))
		for _, name := range u.Suggestions {
			quoted = append(quoted, fmt.Sprintf("%q", name))
		}
		message += fmt.Sprintf(" (filters use the source path name; did you mean %s?)", strings.Join(quoted, ", "))
	}
	return message
}

// ResolveTargetSkillsForTarget applies a target's filters and target_naming
// policy to discovered skills and returns the effective target-visible skills.
func ResolveTargetSkillsForTarget(targetName string, sc config.ResourceTargetConfig, allSkills []DiscoveredSkill) (*TargetSkillResolution, error) {
	filtered, err := SelectTargetSkills(allSkills, targetName, sc)
	if err != nil {
		return nil, fmt.Errorf("failed to apply filters for target %s: %w", targetName, err)
	}

	naming := config.EffectiveTargetNaming(sc.TargetNaming)
	result := &TargetSkillResolution{
		Naming:            naming,
		UnmatchedIncludes: FindUnmatchedIncludes(sc.Include, allSkills),
	}

	if naming == "flat" {
		result.Skills = make([]ResolvedTargetSkill, 0, len(filtered))
		for _, skill := range filtered {
			result.Skills = append(result.Skills, ResolvedTargetSkill{
				Skill:      skill,
				TargetName: skill.FlatName,
			})
		}
		return result, nil
	}

	candidates := make([]ResolvedTargetSkill, 0, len(filtered))
	collisionMap := make(map[string][]string)

	for _, skill := range filtered {
		skillName, nameErr := utils.ParseSkillName(skill.SourcePath)
		if nameErr != nil {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Target '%s': skipped %s because SKILL.md name could not be read: %v", targetName, skill.RelPath, nameErr))
			continue
		}

		if reason := validateStandardTargetSkill(skill, skillName); reason != "" {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Target '%s': skipped %s because %s", targetName, skill.RelPath, reason))
			continue
		}

		entryName := skillName
		if naming == "prefixed" {
			entryName = PrefixedTargetName(skill, skillName)
			if reason := skillpkg.ValidateName(entryName, entryName); reason != "" {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("Target '%s': skipped %s because its prefixed name is invalid: %s", targetName, skill.RelPath, reason))
				continue
			}
		}

		candidates = append(candidates, ResolvedTargetSkill{
			Skill:      skill,
			TargetName: entryName,
			SkillName:  skillName,
		})
		collisionMap[entryName] = append(collisionMap[entryName], skill.RelPath)
	}

	collisionNames := make(map[string]bool)
	for name, paths := range collisionMap {
		if len(paths) <= 1 {
			continue
		}
		slices.Sort(paths)
		collisionNames[name] = true
		result.Collisions = append(result.Collisions, NameCollision{
			Name:  name,
			Paths: paths,
		})
	}

	if len(result.Collisions) > 0 {
		slices.SortFunc(result.Collisions, func(a, b NameCollision) int {
			return strings.Compare(a.Name, b.Name)
		})
	}

	result.Skills = make([]ResolvedTargetSkill, 0, len(candidates))
	for _, candidate := range candidates {
		if collisionNames[candidate.TargetName] {
			continue
		}
		result.Skills = append(result.Skills, candidate)
	}

	return result, nil
}

// ValidTargetNames returns the target-visible names that are valid for the
// current target after naming resolution.
func (r *TargetSkillResolution) ValidTargetNames() map[string]bool {
	names := make(map[string]bool, len(r.Skills))
	for _, skill := range r.Skills {
		names[skill.TargetName] = true
	}
	return names
}

// ExpectedSkillCount is how many skills sync places in a target: the ones its
// filters select, less those naming validation skips and name collisions
// exclude. Running sync again cannot add the rest, so status and doctor must
// not count them as missing.
func ExpectedSkillCount(targetName string, sc config.ResourceTargetConfig, allSkills []DiscoveredSkill) (int, error) {
	resolution, err := ResolveTargetSkillsForTarget(targetName, sc, allSkills)
	if err != nil {
		return 0, err
	}
	return len(resolution.Skills), nil
}

// LegacyNames returns the entries in targetPath that a skill still holds under
// the name another target naming gave it, keyed by that name. Sync renames such
// an entry unless the new name is taken, so prune and diff must not treat it
// as an orphan.
func (r *TargetSkillResolution) LegacyNames(mode, targetPath string, manifest *Manifest) map[string]ResolvedTargetSkill {
	legacy := make(map[string]ResolvedTargetSkill)
	if r == nil {
		return legacy
	}
	taken := r.ValidTargetNames()
	for _, skill := range r.Skills {
		if old, err := findLegacyTargetEntry(mode, targetPath, skill, taken, manifest); err == nil && old.name != "" {
			legacy[old.name] = skill
		}
	}
	return legacy
}

// maxListed caps how many names a summary line spells out.
const maxListed = 5

// printResolutionSummary reports skipped skills and collisions. It names only
// the first few skipped skills, so a source with thousands of invalid names
// does not flood the output.
func printResolutionSummary(r *TargetSkillResolution) {
	if n := len(r.Warnings); n > 0 {
		fmt.Fprintf(DiagOutput, "  %d skill(s) skipped (naming validation)\n", n)
		for _, w := range r.Warnings[:min(n, maxListed)] {
			fmt.Fprintf(DiagOutput, "    %s\n", w)
		}
		if n > maxListed {
			fmt.Fprintf(DiagOutput, "    ... and %d more\n", n-maxListed)
		}
	}
	if n := len(r.Collisions); n > 0 {
		fmt.Fprintf(DiagOutput, "  %d name collision(s) excluded\n", n)
	}
}

// keptLocalWarning tells which target entries sync left alone because a folder
// the user made already holds the name of a source skill. Like the skipped
// list, it names only the first few.
func keptLocalWarning(names []string) string {
	msg := "kept local: " + strings.Join(names[:min(len(names), maxListed)], ", ")
	if n := len(names) - maxListed; n > 0 {
		msg += fmt.Sprintf(" ... and %d more", n)
	}
	return msg + " (sync --force replaces them)"
}

// RenamedFrom inverts LegacyNames: for each target name sync will move a
// legacy entry into, the entry's current name.
func RenamedFrom(legacy map[string]ResolvedTargetSkill) map[string]string {
	renamed := make(map[string]string, len(legacy))
	for old, skill := range legacy {
		renamed[skill.TargetName] = old
	}
	return renamed
}

// KeptLegacyReason is the diff reason for a local folder that holds a skill's
// new name. Sync leaves the folder alone and keeps the skill under old.
func KeptLegacyReason(old string) string {
	return "local folder; the skill stays at " + old
}

// NamingChangedReason is the diff reason for a managed copy made under another
// target naming, which sync copies again so its name: follows the current naming.
const NamingChangedReason = "target naming changed"

// RenameReason is the diff reason for an entry sync renames after a target
// naming change.
func RenameReason(old string) string {
	return "renamed from " + old + " (target naming changed)"
}

// namedTarget is the entry name a skill gets under one target naming.
type namedTarget struct{ naming, name string }

// targetNameCandidates returns the entry name a skill gets under each target
// naming, in the order a manifest without naming records is checked. The
// standard and prefixed names come from the folder name, which standard naming
// requires name: to match, so no SKILL.md has to be read.
func targetNameCandidates(skill DiscoveredSkill) []namedTarget {
	base := filepath.Base(filepath.Clean(skill.SourcePath))
	return []namedTarget{
		{"flat", skill.FlatName},
		{"standard", base},
		{"prefixed", PrefixedTargetName(skill, base)},
	}
}

// PrefixedTargetName returns "<repo>-<name>" for a skill inside a tracked repo,
// where <repo> is the repo folder without its leading "_", reduced to the
// characters a skill name allows. Other skills, and names that already start
// with the repo, keep their name.
func PrefixedTargetName(skill DiscoveredSkill, skillName string) string {
	if !skill.IsInRepo {
		return skillName
	}
	repo := normalizeSpecName(strings.TrimPrefix(path.Base(filepath.ToSlash(skill.RepoRelPath)), "_"))
	if repo == "" || skillName == repo || strings.HasPrefix(skillName, repo+"-") {
		return skillName
	}
	return repo + "-" + skillName
}

// normalizeSpecName applies NFKC and lowercases s, as skillpkg.ValidateName reads a
// name, turns every character other than a letter or number (in any script) into
// "-", collapses repeated hyphens and trims them from both ends.
func normalizeSpecName(s string) string {
	mapped := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return r
		}
		return '-'
	}, strings.ToLower(norm.NFKC.String(s)))
	parts := strings.FieldsFunc(mapped, func(r rune) bool { return r == '-' })
	return strings.Join(parts, "-")
}

func validateStandardTargetSkill(skill DiscoveredSkill, skillName string) string {
	return skillpkg.ValidateName(skillName, filepath.Base(filepath.Clean(skill.SourcePath)))
}
