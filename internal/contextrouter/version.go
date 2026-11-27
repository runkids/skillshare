package contextrouter

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
)

// Reference is the agents-context-router release the size rule follows, and
// SemanticsVersion is its version number.
const (
	Reference        = "v0.6.0"
	SemanticsVersion = "0.6.0"
)

// ScriptRel is where a folder keeps its copy of the router script.
const ScriptRel = "scripts/ai-context.py"

// How the folder's own script relates to the rule applied.
const (
	VersionOK      = "ok"      // the script's rule is the one applied
	VersionLegacy  = "legacy"  // an older script: the options it does not know are ignored, as it ignores them
	VersionNewer   = "newer"   // newer than this port knows: measured by the current rule, unverified
	VersionUnknown = "unknown" // no script, or no version in it: measured by the current rule
)

var scriptVersionRe = regexp.MustCompile(`(?m)^VERSION\s*=\s*['"](\d+)\.(\d+)\.(\d+)['"]`)

type version [3]int

func (v version) less(o version) bool {
	for i := range v {
		if v[i] != o[i] {
			return v[i] < o[i]
		}
	}
	return false
}

func (v version) String() string { return fmt.Sprintf("%d.%d.%d", v[0], v[1], v[2]) }

// Releases where the size rule changed, and the first one this port has not seen.
var (
	sourceFrontmatterSince = version{0, 4, 0} // a source's "frontmatter": true
	stripAndCompactSince   = version{0, 5, 0} // stripFrontmatter and compactPathLinks
	firstUnseen            = version{0, 7, 0}
)

// parseScriptVersion finds the VERSION assignment in the router script's text.
func parseScriptVersion(text string) (version, bool) {
	match := scriptVersionRe.FindStringSubmatch(text)
	if match == nil {
		return version{}, false
	}
	var v version
	for i := range v {
		n, err := strconv.Atoi(match[i+1])
		if err != nil {
			return version{}, false
		}
		v[i] = n
	}
	return v, true
}

// scriptVersion reads the version of the folder's own router script. The
// script is read as text, within the same bounds as any other file, and is
// never run.
func scriptVersion(root *os.Root, max int64) (version, bool) {
	text, code := readRegular(root, ScriptRel, max)
	if code != "" {
		return version{}, false
	}
	return parseScriptVersion(text)
}
