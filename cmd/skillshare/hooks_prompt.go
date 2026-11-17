package main

import (
	"fmt"
	"time"

	"skillshare/internal/hooks"
	"skillshare/internal/ui"
)

// trustCodexHooks lists the Codex hooks that need trust and records it once the user
// agrees. A person has to see each command first, so without a terminal only --yes
// is consent; a hook that is already trusted needs no answer.
func trustCodexHooks(service *hooks.Service, o hooksOptions) (trust []hooks.CodexTrust, declined bool, err error) {
	plan, err := service.PreviewCodexTrust()
	if plan == nil || err != nil {
		return nil, false, err
	}
	needed := plan.Needed()
	if len(needed) == 0 {
		return plan.Hooks, false, plan.Err()
	}
	if !o.yes {
		if o.json || !runningInInteractiveTTY() {
			return plan.Hooks, false, fmt.Errorf("%s trust; review the commands and pass --yes to record it", needVerb(len(needed), "Codex hook"))
		}
		printHooksTrust(needed)
		for _, f := range plan.Failed {
			ui.Warning("Codex could not list the hooks of %s, so they are left out: %s", f.Target, f.Error)
		}
		fmt.Println()
		ok, err := ui.ConfirmAction("Trust "+plural(len(needed), "hook")+" in Codex?", false)
		if err != nil {
			return plan.Hooks, false, err
		}
		if !ok {
			ui.Cancelled("trusted")
			return plan.Hooks, true, plan.Err()
		}
	}
	start := time.Now()
	plan, err = service.TrustCodex(plan.Revision)
	logHooksOp(service.ConfigPath, "hooks trust", start, err)
	if plan == nil {
		return nil, false, err
	}
	return plan.Hooks, false, err
}

// needVerb is "1 Codex hook needs" or "2 Codex hooks need".
func needVerb(n int, noun string) string {
	if n == 1 {
		return plural(n, noun) + " needs"
	}
	return plural(n, noun) + " need"
}
