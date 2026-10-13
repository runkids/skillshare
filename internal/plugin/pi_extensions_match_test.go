package plugin

import (
	"encoding/json"
	"os"
	"testing"
)

// The golden file is Pi 0.99.2's own minimatch output (scripts/pi/minimatch-golden.mjs).
func TestPiGlobAgreesWithPiMinimatch(t *testing.T) {
	data, err := os.ReadFile("testdata/pi_minimatch_golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Cases []struct {
			Pattern, Subject string
			Match            bool
		}
	}
	if err := json.Unmarshal(data, &golden); err != nil || len(golden.Cases) == 0 {
		t.Fatalf("golden: %v", err)
	}
	unknown := 0
	for _, c := range golden.Cases {
		if !piGlobSupported(c.Pattern) {
			t.Fatalf("golden pattern %q must be in the supported grammar", c.Pattern)
		}
		want := piOff
		if c.Match {
			want = piOn
		}
		// Unknown is allowed only where Go and JavaScript count characters
		// differently; a known answer must always be Pi's.
		switch got := piGlobMatch(c.Pattern, c.Subject); {
		case got == piUnknown && piCountsDiffer(c.Pattern, c.Subject):
			unknown++
		case got != want:
			t.Errorf("minimatch(%q, %q) = %v, Go = %v", c.Subject, c.Pattern, c.Match, got)
		}
	}
	if unknown == 0 {
		t.Error("the golden file has no astral case left unknown")
	}
}

// '?' is one UTF-16 code unit in minimatch: an astral character takes two.
func TestPiGlobLeavesAstralQuestionMarksUnknown(t *testing.T) {
	for _, c := range [][2]string{{"?.ts", "😀.ts"}, {"??.ts", "😀.ts"}, {"extensions/a?.ts", "extensions/a😀.ts"}, {"?.ts", "\xff.ts"}} {
		if got := piGlobMatch(c[0], c[1]); got != piUnknown {
			t.Errorf("piGlobMatch(%q, %q) = %v, want unknown", c[0], c[1], got)
		}
	}
	if got := piGlobMatch("?.ts", "é.ts"); got != piOn {
		t.Errorf("a BMP character is one code unit: got %v", got)
	}
}

// Anything outside the restricted grammar is reported, never approximated.
func TestPiGlobRefusesUnsupportedPatterns(t *testing.T) {
	for _, p := range []string{"", "**/*.ts", "extensions/**", "a.{ts,js}", "[ab].ts", "+(a|b).ts", "@(a).ts", "a\\*.ts", "#a", "!a.ts", "./a.ts", "extensions/../a.ts", "extensions//a.ts", "extensions/"} {
		if piGlobSupported(p) {
			t.Errorf("%q should be unsupported", p)
		}
	}
}

func TestPiEvaluateFollowsPiPatternOrder(t *testing.T) {
	files := []string{"extensions/a.ts", "extensions/b.ts", "extensions/c.ts"}
	cases := []struct {
		name  string
		rules []string
		want  []piTri
	}{
		{"exact exclude", []string{"-extensions/b.ts"}, []piTri{piOn, piOff, piOn}},
		{"include then exclude glob", []string{"extensions/*.ts", "!extensions/c.ts"}, []piTri{piOn, piOn, piOff}},
		{"+ beats !, - beats +", []string{"!extensions/*.ts", "+extensions/a.ts", "+extensions/b.ts", "-extensions/b.ts"}, []piTri{piOn, piOff, piOff}},
		{"unsupported glob is unknown unless an exact rule decides", []string{"!extensions/**", "+extensions/a.ts", "-extensions/b.ts"}, []piTri{piOn, piOff, piUnknown}},
		{"./ prefix on exact rules", []string{"-./extensions/c.ts"}, []piTri{piOn, piOn, piOff}},
		{".\\ prefix on exact rules", []string{`-.\extensions/c.ts`}, []piTri{piOn, piOn, piOff}},
		// Pi 1.2.0 rewrites these rules with normalizePath; older Pi compares them as written.
		{"~ exact exclude is unknown", []string{"-~/pkg/extensions/a.ts"}, []piTri{piUnknown, piUnknown, piUnknown}},
		{"~ exact include is unknown unless already on", []string{"!extensions/a.ts", "+~/pkg/extensions/a.ts"}, []piTri{piUnknown, piOn, piOn}},
		{"~ glob exclude is unknown", []string{"!~/pkg/extensions/*.ts"}, []piTri{piUnknown, piUnknown, piUnknown}},
		{"a known exact exclude still decides over ~", []string{"-~/pkg/extensions/a.ts", "-extensions/a.ts"}, []piTri{piOff, piUnknown, piUnknown}},
		{"bare ~ is a home path", []string{"-~"}, []piTri{piUnknown, piUnknown, piUnknown}},
		{"./~ is a home path", []string{"-./~/pkg/extensions/a.ts"}, []piTri{piUnknown, piUnknown, piUnknown}},
		{"~name stays literal", []string{"-~a.ts"}, []piTri{piOn, piOn, piOn}},
		{"~ include glob is unknown", []string{"~/pkg/extensions/*.ts"}, []piTri{piUnknown, piUnknown, piUnknown}},
		{"file:// exact is unknown", []string{"-file:///pkg/extensions/a.ts"}, []piTri{piUnknown, piUnknown, piUnknown}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for i, f := range files {
				if got := piEvaluate(c.rules, f, "/pkg/"+f).state; got != c.want[i] {
					t.Errorf("%s: got %v want %v", f, got, c.want[i])
				}
			}
		})
	}
}

func TestPiEvaluateDeltaLeavesHomeRulesUnknown(t *testing.T) {
	if state, named, _ := piEvaluateDelta([]string{"-~/pkg/extensions/a.ts"}, "extensions/a.ts", "/pkg/extensions/a.ts"); state != piUnknown || !named {
		t.Errorf("got %v named=%v, want unknown and named", state, named)
	}
}

func TestPiEvaluateNamesTheHomeRuleItLeavesUnknown(t *testing.T) {
	if got := piEvaluate([]string{"-~/pkg/extensions/a.ts"}, "extensions/a.ts", "/pkg/extensions/a.ts").exact; got != "-~/pkg/extensions/a.ts" {
		t.Errorf("exact = %q, want the home rule", got)
	}
}

func TestPiRewrittenPatternFollowsPiNormalizePath(t *testing.T) {
	cases := []struct {
		pattern, goos string
		want          bool
	}{
		{"~", "linux", true},
		{"~/x.ts", "linux", true},
		{"file:///pkg/x.ts", "linux", true},
		{`~\x.ts`, "linux", false},
		{"/c/Users/me/x.ts", "linux", false},
		{`~\x.ts`, "windows", true},
		{"/c/Users/me/x.ts", "windows", true},
		{"/mnt/d/x.ts", "windows", true},
		{"/cygdrive/C", "windows", true},
		{"//server/share/x.ts", "windows", false},
		{"/pkg/x.ts", "windows", false},
		{"~x.ts", "windows", false},
		{"extensions/x.ts", "windows", false},
	}
	for _, c := range cases {
		if got := piRewrittenPattern(c.pattern, c.goos); got != c.want {
			t.Errorf("piRewrittenPattern(%q, %s) = %v, want %v", c.pattern, c.goos, got, c.want)
		}
	}
}

func TestPiExactPatternFollowsPiNormalizeExactPattern(t *testing.T) {
	cases := []struct{ pattern, goos, want string }{
		{"./extensions/a.ts", "linux", "extensions/a.ts"},
		{`.\extensions/a.ts`, "linux", "extensions/a.ts"},
		{`extensions\a.ts`, "linux", `extensions\a.ts`},
		{`.\extensions\a.ts`, "windows", "extensions/a.ts"},
		{`C:\pkg\extensions\a.ts`, "windows", "C:/pkg/extensions/a.ts"},
		{"././a.ts", "windows", "./a.ts"},
	}
	for _, c := range cases {
		if got := piExactPattern(c.pattern, c.goos); got != c.want {
			t.Errorf("piExactPattern(%q, %s) = %q, want %q", c.pattern, c.goos, got, c.want)
		}
	}
}
