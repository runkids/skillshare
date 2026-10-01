package hooks

import (
	"fmt"
	"slices"
)

// CatalogEvent is one native event an Agent documents.
type CatalogEvent struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	// Matcher reports whether the event's registrations take a matcher.
	Matcher bool `json:"matcher"`
}

// AgentCatalog is a command Agent's documented events and the unit of its timeout field.
type AgentCatalog struct {
	Events      []CatalogEvent `json:"events"`
	TimeoutUnit string         `json:"timeoutUnit"` // seconds or milliseconds
	// aliases are other spellings the Agent documents for its events.
	aliases []string
}

func ev(name string, matcher bool, description string) CatalogEvent {
	return CatalogEvent{Name: name, Description: description, Matcher: matcher}
}

// Catalog lists the native events of each command Agent, taken from its official hooks
// reference (accessed 2026-09-30). Agents add events over time, so an unknown name is
// a warning, never an error. Code Agents (pi, amp, opencode) have no event map.
var Catalog = map[string]AgentCatalog{
	// Native Git githooks(5) events; Git appends event arguments to commands.
	"git": {Events: []CatalogEvent{
		ev("applypatch-msg", false, "Native applypatch-msg hook event."),
		ev("pre-applypatch", false, "Native pre-applypatch hook event."),
		ev("post-applypatch", false, "Native post-applypatch hook event."),
		ev("pre-commit", false, "Before a commit is created."),
		ev("pre-merge-commit", false, "Native pre-merge-commit hook event."),
		ev("prepare-commit-msg", false, "Native prepare-commit-msg hook event."),
		ev("commit-msg", false, "Native commit-msg hook event."),
		ev("post-commit", false, "After a commit is created."),
		ev("pre-rebase", false, "Native pre-rebase hook event."),
		ev("post-checkout", false, "Native post-checkout hook event."),
		ev("post-merge", false, "Native post-merge hook event."),
		ev("pre-push", false, "Before refs are pushed; receives ref updates on stdin."),
		ev("pre-receive", false, "Native pre-receive hook event."),
		ev("update", false, "Native update hook event."),
		ev("proc-receive", false, "Native proc-receive hook event."),
		ev("post-receive", false, "Native post-receive hook event."),
		ev("post-update", false, "Native post-update hook event."),
		ev("reference-transaction", false, "Native reference-transaction hook event."),
		ev("push-to-checkout", false, "Native push-to-checkout hook event."),
		ev("pre-auto-gc", false, "Native pre-auto-gc hook event."),
		ev("post-rewrite", false, "After commits are rewritten."),
		ev("sendemail-validate", false, "Native sendemail-validate hook event."),
		ev("fsmonitor-watchman", false, "Native fsmonitor-watchman hook event."),
		ev("p4-changelist", false, "Native p4-changelist hook event."),
		ev("p4-prepare-changelist", false, "Native p4-prepare-changelist hook event."),
		ev("p4-post-changelist", false, "Native p4-post-changelist hook event."),
		ev("p4-pre-submit", false, "Native p4-pre-submit hook event."),
		ev("post-index-change", false, "Native post-index-change hook event."),
	}},
	// https://code.claude.com/docs/en/hooks
	"claude": {TimeoutUnit: "seconds", Events: []CatalogEvent{
		ev("SessionStart", true, "A session starts or resumes."),
		ev("Setup", true, "Runs on --init-only, or --init/--maintenance in non-interactive mode."),
		ev("UserPromptSubmit", false, "A prompt is submitted, before Claude processes it."),
		ev("UserPromptExpansion", true, "A typed command expands into a prompt; can block it."),
		ev("PreToolUse", true, "Before a tool call runs; can block it."),
		ev("PermissionRequest", true, "A tool call needs a permission decision."),
		ev("PermissionDenied", true, "Auto mode denied a tool call."),
		ev("PostToolUse", true, "After a tool call succeeds."),
		ev("PostToolUseFailure", true, "After a tool call fails."),
		ev("PostToolBatch", false, "After a batch of parallel tool calls resolves."),
		ev("Notification", true, "Claude Code sends a notification."),
		ev("MessageDisplay", false, "While assistant message text is displayed."),
		ev("SubagentStart", true, "A subagent is spawned."),
		ev("SubagentStop", true, "A subagent finishes."),
		ev("TaskCreated", false, "A task is created."),
		ev("TaskCompleted", false, "A task is marked completed."),
		ev("Stop", false, "Claude finishes responding."),
		ev("StopFailure", true, "A turn ends because of an API error."),
		ev("TeammateIdle", false, "An agent-team teammate is about to go idle."),
		ev("InstructionsLoaded", true, "CLAUDE.md or a rules file is loaded into context."),
		ev("ConfigChange", true, "A config file changes during a session."),
		ev("CwdChanged", false, "The working directory changes."),
		ev("DirectoryAdded", true, "A working directory is added mid-session."),
		ev("FileChanged", true, "A watched file changes on disk."),
		ev("WorktreeCreate", false, "A worktree is being created."),
		ev("WorktreeRemove", false, "A worktree is being removed."),
		ev("PreCompact", true, "Before context compaction."),
		ev("PostCompact", true, "After context compaction completes."),
		ev("PreModelSwitch", true, "Before a requested model switch; can block it."),
		ev("PostModelSwitch", true, "After the session's model changes."),
		ev("Elicitation", true, "An MCP server requests user input."),
		ev("ElicitationResult", true, "The user answered an MCP elicitation."),
		ev("SessionEnd", true, "A session terminates."),
	}},
	// https://learn.chatgpt.com/docs/hooks
	"codex": {TimeoutUnit: "seconds", Events: []CatalogEvent{
		ev("SessionStart", true, "A session starts; can add context."),
		ev("SessionEnd", true, "A session ends; for cleanup."),
		ev("SubagentStart", true, "Before a subagent starts; can add context."),
		ev("PreToolUse", true, "Before a tool call; can block, allow or rewrite input."),
		ev("PermissionRequest", true, "An approval prompt; can allow or deny."),
		ev("PostToolUse", true, "After a tool completes; can block the result or add feedback."),
		ev("PreCompact", true, "Before compaction; can prevent it."),
		ev("PostCompact", true, "After compaction completes."),
		ev("UserPromptSubmit", false, "A prompt is submitted; can block it."),
		ev("SubagentStop", true, "A subagent finishes."),
		ev("Stop", false, "A turn stops; can force continuation."),
		ev("Interrupt", false, "The user interrupts an active turn."),
	}},
	// https://geminicli.com/docs/hooks/reference/
	"gemini": {TimeoutUnit: "milliseconds", Events: []CatalogEvent{
		ev("BeforeTool", true, "Before a tool call; can validate arguments or block it."),
		ev("AfterTool", true, "After a tool call; can audit or filter output."),
		ev("BeforeAgent", false, "After a prompt is submitted, before the agent plans."),
		ev("AfterAgent", false, "Once per turn after the final model response."),
		ev("BeforeModel", false, "Before a request is sent to the model."),
		ev("BeforeToolSelection", false, "Before the model picks tools; can filter them."),
		ev("AfterModel", false, "After each model response chunk; allows redaction."),
		ev("SessionStart", false, "On startup, resume or /clear."),
		ev("SessionEnd", false, "The CLI exits or the session is cleared."),
		ev("Notification", false, "The CLI emits a system alert."),
		ev("PreCompress", false, "Before history is summarized."),
	}},
	// https://docs.github.com/en/copilot/reference/hooks-reference
	"copilot": {TimeoutUnit: "seconds", aliases: []string{"PreToolUse", "PostToolUse", "PermissionRequest", "PreCompact"}, Events: []CatalogEvent{
		ev("sessionStart", false, "A new or resumed session begins."),
		ev("sessionEnd", false, "The session terminates."),
		ev("userPromptSubmitted", false, "The user submits a prompt."),
		ev("userPromptTransformed", false, "After the prompt is transformed, before the model gets it."),
		ev("preToolUse", true, "Before a tool runs; can allow, deny or modify it."),
		ev("postToolUse", true, "After a tool succeeds."),
		ev("postToolUseFailure", false, "After a tool fails."),
		ev("preCompact", true, "Before context compaction."),
		ev("agentStop", false, "The main agent finishes a turn."),
		ev("subagentStart", true, "Before a subagent is spawned."),
		ev("subagentStop", false, "A subagent completes."),
		ev("errorOccurred", false, "An error occurs during execution."),
		ev("notification", true, "The CLI emits a notification."),
		ev("permissionRequest", true, "Before the permission service decides."),
	}},
	// https://cursor.com/docs/hooks
	"cursor": {TimeoutUnit: "seconds", Events: []CatalogEvent{
		ev("sessionStart", false, "A conversation begins; can inject environment and context."),
		ev("sessionEnd", false, "A conversation ends."),
		ev("preToolUse", true, "Before any tool runs."),
		ev("postToolUse", true, "After a tool succeeds."),
		ev("postToolUseFailure", true, "A tool fails, times out or is denied."),
		ev("subagentStart", true, "Before a subagent spawns; can allow or deny it."),
		ev("subagentStop", true, "A subagent completes."),
		ev("beforeShellExecution", true, "Before a shell command runs."),
		ev("afterShellExecution", true, "After a shell command runs."),
		ev("beforeMCPExecution", true, "Before an MCP tool call."),
		ev("afterMCPExecution", false, "After an MCP tool call completes."),
		ev("beforeReadFile", true, "Before the agent reads a file."),
		ev("afterFileEdit", true, "After the agent edits a file."),
		ev("beforeSubmitPrompt", true, "Before the prompt is sent; can block it."),
		ev("preCompact", false, "Before compaction; observational only."),
		ev("stop", true, "The agent loop ends; can submit a follow-up."),
		ev("afterAgentResponse", true, "After an assistant message completes."),
		ev("afterAgentThought", true, "After a thinking block completes."),
		ev("beforeTabFileRead", true, "Before Tab reads a file."),
		ev("afterTabFileEdit", true, "After Tab edits a file."),
		ev("workspaceOpen", false, "A workspace opens or its folder changes."),
	}},
	// https://docs.factory.com/harness/hooks
	"droid": {TimeoutUnit: "seconds", Events: []CatalogEvent{
		ev("PreToolUse", true, "After tool parameters are built, before the tool runs."),
		ev("PostToolUse", true, "Right after a tool completes."),
		ev("UserPromptSubmit", false, "Before Droid processes a submitted prompt."),
		ev("Notification", false, "Droid sends a notification."),
		ev("Stop", false, "The main Droid is about to finish responding."),
		ev("SubagentStop", false, "A sub-droid launched by Task finishes."),
		ev("PreCompact", false, "Before manual or automatic compaction."),
		ev("SessionStart", false, "On start, resume, clear or after compaction."),
		ev("SessionEnd", false, "A session ends."),
	}},
	// https://antigravity.google/docs/hooks (the IDE and CLI tabs list the same events)
	"antigravity": {TimeoutUnit: "seconds", Events: []CatalogEvent{
		ev("PreToolUse", true, "Before a tool runs."),
		ev("PostToolUse", true, "After a tool completes."),
		ev("PreInvocation", false, "Before the model is called."),
		ev("PostInvocation", false, "Right after each model invocation completes."),
		ev("Stop", false, "Execution terminates."),
	}},
	// https://qwenlm.github.io/qwen-code-docs/en/users/features/hooks/
	"qwen": {TimeoutUnit: "seconds", Events: []CatalogEvent{
		ev("PreToolUse", true, "Before a tool runs; for permission checks and input validation."),
		ev("PostToolUse", true, "After a tool succeeds; can add context."),
		ev("PostToolUseFailure", true, "A tool execution fails."),
		ev("PostToolBatch", false, "Once after all tool calls in a batch resolve."),
		ev("UserPromptSubmit", false, "Before the model call; can validate or enrich the prompt."),
		ev("UserPromptExpansion", true, "A slash command expands into a prompt."),
		ev("SessionStart", true, "A new session starts."),
		ev("SessionEnd", true, "A session ends."),
		ev("SessionDelete", false, "A session is permanently deleted."),
		ev("MessageDisplay", false, "Repeatedly while the reply streams."),
		ev("Stop", false, "Before Qwen concludes its response."),
		ev("StopFailure", true, "Instead of Stop, on an API error or loop detection."),
		ev("SubagentStart", true, "A subagent starts."),
		ev("SubagentStop", true, "A subagent finishes."),
		ev("PreCompact", true, "Before conversation compaction."),
		ev("PostCompact", true, "After compaction succeeds."),
		ev("Notification", true, "A notification is sent."),
		ev("PermissionRequest", true, "A permission dialog is shown."),
		ev("PermissionDenied", true, "The auto-mode classifier blocks a tool call."),
		ev("TodoCreated", false, "A todo item is created."),
		ev("TodoCompleted", false, "A todo item is completed."),
		ev("InstructionsLoaded", true, "A context file is loaded into the system prompt."),
	}},
}

// knownEvent reports whether the Agent documents the event; Agents without a catalog
// know every name.
func knownEvent(target, event string) bool {
	c, ok := Catalog[target]
	if !ok {
		return true
	}
	return slices.Contains(c.aliases, event) || slices.ContainsFunc(c.Events, func(e CatalogEvent) bool { return e.Name == event })
}

// EventWarnings names the events of an entry its Agents do not document.
func EventWarnings(name string, e Entry) []string {
	return eventWarnings(name, e, func(key string) string { return key })
}

func eventWarnings(name string, e Entry, agentOf func(string) string) []string {
	var out []string
	for _, target := range sortedKeys(e.Bindings) {
		eventNames := sortedKeys(e.Bindings[target].Events)
		if target == "git" {
			for _, name := range sortedKeys(e.Bindings[target].Commands) {
				eventNames = append(eventNames, e.Bindings[target].Commands[name].Events...)
			}
		}
		for _, event := range eventNames {
			if !knownEvent(agentOf(target), event) {
				out = append(out, fmt.Sprintf("hook %s: %s does not document the event %q; check its spelling", name, target, event))
			}
		}
	}
	return out
}
