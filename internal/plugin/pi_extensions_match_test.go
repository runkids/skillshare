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
