package skillmove

import (
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/profile"
)

func TestMove_UnavailableProfileRefusesBeforeMove(t *testing.T) {
	f := newFixture(t)
	f.skill(t, "alpha")
	f.opts.Targets = []Target{{Name: "claude", Config: config.ResourceTargetConfig{
		Profile: profile.Resolve(profile.Catalog{}, profile.State{Active: "missing"}, "config.yaml", nil),
	}}}
	planned := Plan(f.discover(t), []string{"alpha"}, "group", f.opts)
	if len(planned) != 1 || planned[0].Err == nil || !strings.Contains(planned[0].Err.Error(), "profile use") {
		t.Fatalf("unavailable selection was ignored: %+v", planned)
	}
}
