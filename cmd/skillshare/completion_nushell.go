package main

const nushellCompletionScript = `# skillshare Nushell completion

def "nu-complete skillshare commands" [] {
    [
        { value: "init", description: "Initialize skillshare" }
        { value: "install", description: "Install skills/agents from local path or git repo" }
        { value: "uninstall", description: "Remove skills/agents from source directory" }
        { value: "list", description: "List installed skills" }
        { value: "search", description: "Search or browse GitHub for skills" }
        { value: "sync", description: "Sync skills/agents/extras/MCP to targets" }
        { value: "plugin", description: "Manage complete native plugins" }
        { value: "mcp", description: "Manage MCP connections" }
        { value: "hooks", description: "Manage Agent hooks" }
        { value: "status", description: "Show status of all targets" }
        { value: "diff", description: "Show differences between source and targets" }
        { value: "backup", description: "Create backup of targets" }
        { value: "restore", description: "Restore target from backup" }
        { value: "collect", description: "Collect local skills/agents from targets" }
        { value: "pull", description: "Pull from git remote and sync" }
        { value: "push", description: "Commit and push source to git remote" }
        { value: "commit", description: "Create local git commit without pushing" }
        { value: "doctor", description: "Check environment and diagnose issues" }
        { value: "target", description: "Manage targets" }
        { value: "upgrade", description: "Upgrade CLI and/or skillshare skill" }
        { value: "update", description: "Update skills/agents or tracked repositories" }
        { value: "check", description: "Check for available updates" }
        { value: "new", description: "Create a new skill" }
        { value: "trash", description: "Manage trashed skills/agents" }
        { value: "analyze", description: "Analyze skills/agents" }
        { value: "audit", description: "Scan skills/agents for security threats" }
        { value: "hub", description: "Manage hubs" }
        { value: "log", description: "View operation log" }
        { value: "ui", description: "Launch web dashboard" }
        { value: "tui", description: "Toggle interactive TUI mode" }
        { value: "extras", description: "Manage extra resource types" }
        { value: "enable", description: "Enable a disabled skill/agent" }
        { value: "disable", description: "Disable a skill/agent" }
        { value: "completion", description: "Generate shell completion scripts" }
        { value: "version", description: "Show version" }
        { value: "help", description: "Show help" }
    ]
}

def "nu-complete skillshare plugin" [] {
    [add discover import list inspect sync check update enable disable remove]
}

def "nu-complete skillshare mcp" [] {
    [add check edit import list remove restore]
}

def "nu-complete skillshare mcp-target" [] {
    [claude codex cursor vscode opencode kilocode grok antigravity amp claude-desktop cline copilot factory gemini goose junie kiro lmstudio warp windsurf pi omp]
}

def "nu-complete skillshare account-agent" [] {
    [claude codex pi omp]
}

def "nu-complete skillshare hooks" [] {
    [add disable edit enable import list remove restore sync]
}

def "nu-complete skillshare plugin-target" [] {
    [claude codex cursor antigravity agy antigravity-cli copilot grok kimi hermes devin pi opencode omp]
}

export extern "skillshare mcp" [
    command?: string@"nu-complete skillshare mcp"
    name?: string
    --tools-allow: string # Only these tools, comma-separated, * matches any characters; empty clears
    --tools-deny: string # Never these tools, comma-separated, * matches any characters; empty clears
    --pi-options: string # Other Pi built-in per-server fields as JSON
    --target: string@"nu-complete skillshare mcp-target"
    --from: string@"nu-complete skillshare mcp-target"
    --url: string
    --file: string
    --revision: string
    --sync
    --replace
    --disabled # add: turn off the server in the project
    --keep-files # remove: leave Agent entries as they are
    --dry-run(-n)
    --json
    --no-dns # Skip host lookups
    --live # check: start or call each server
    --timeout: string # check --live: per-server timeout, such as 10s
    --no-tui
    --project(-p)
    --global(-g)
    --help(-h)
]

export extern "skillshare hooks" [
    command?: string@"nu-complete skillshare hooks"
    name?: string
    --file: string # Entry or native file
    --from: string # Import Agent
    --revision: string
    --sync
    --replace
    --keep-files # remove: leave Agent entries as they are
    --dry-run(-n)
    --json
    --project(-p)
    --global(-g)
    --help(-h)
]

export extern "skillshare plugin" [
    command?: string@"nu-complete skillshare plugin"
    value?: string
    --target: string@"nu-complete skillshare plugin-target"
    --from: string@"nu-complete skillshare plugin-target"
    --plugin: string
    --name: string
    --source-ref: string
    --entry: string
    --revision: string
    --dry-run(-n)
    --json
    --no-tui
    --global(-g)
    --project(-p)
    --help(-h)
]

def "nu-complete skillshare target" [] {
    [
        { value: "add", description: "Add a target" }
        { value: "remove", description: "Unlink target and restore skills" }
        { value: "list", description: "List all targets" }
    ]
}

def "nu-complete skillshare trash" [] {
    [
        { value: "list", description: "List trashed items" }
        { value: "restore", description: "Restore from trash" }
        { value: "delete", description: "Delete permanently" }
        { value: "empty", description: "Clear all trash" }
        { value: "agents", description: "Trashed agents" }
    ]
}

def "nu-complete skillshare hub" [] {
    [
        { value: "add", description: "Add hub" }
        { value: "list", description: "List hubs" }
        { value: "remove", description: "Remove hub" }
        { value: "default", description: "Set default hub" }
        { value: "index", description: "Create skill index" }
    ]
}

def "nu-complete skillshare extras" [] {
    [
        { value: "init", description: "Create extra resource type" }
        { value: "list", description: "List extras with sync status" }
        { value: "remove", description: "Remove extra resource type" }
        { value: "collect", description: "Collect local files into extras" }
        { value: "source", description: "Show/set extras source" }
        { value: "memory", description: "Manage shared Markdown notes" }
    ]
}

def "nu-complete skillshare audit" [] {
    [
        { value: "rules", description: "Manage security rules" }
        { value: "agents", description: "Audit agents" }
    ]
}

def "nu-complete skillshare backup" [] {
    [
        { value: "files", description: "Versions of single files skillshare rewrote" }
        { value: "agents", description: "Back up agents" }
    ]
}

def "nu-complete skillshare completion" [] {
    [
        { value: "bash", description: "Generate bash completions" }
        { value: "zsh", description: "Generate zsh completions" }
        { value: "fish", description: "Generate fish completions" }
        { value: "powershell", description: "Generate PowerShell completions" }
        { value: "nushell", description: "Generate Nushell completions" }
    ]
}

def "nu-complete skillshare tui" [] {
    [
        { value: "on", description: "Enable TUI mode" }
        { value: "off", description: "Disable TUI mode" }
    ]
}

def "nu-complete skillshare backup-files" [] {
    [
        { value: "list", description: "List files with saved versions" }
        { value: "show", description: "Show versions of one file" }
        { value: "restore", description: "Restore a saved version" }
    ]
}

def "nu-complete skillshare audit-rules" [] {
    [
        { value: "disable", description: "Disable a rule" }
        { value: "enable", description: "Enable a rule" }
        { value: "severity", description: "Override rule severity" }
        { value: "reset", description: "Reset rule overrides" }
        { value: "init", description: "Create rules file" }
    ]
}

def "nu-complete skillshare ui" [] {
    [
        { value: "start", description: "Start a background UI server" }
        { value: "stop", description: "Stop a background UI server" }
    ]
}

def "nu-complete skillshare sync-scope" [] {
    ["agents" "extras" "mcp" "hooks" "plugins"]
}

def "nu-complete skillshare kind" [] {
    ["agents"]
}

def "nu-complete skillshare kind-flag" [] {
    ["skill" "agent"]
}

def "nu-complete skillshare new-pattern" [] {
    ["tool-wrapper" "generator" "reviewer" "inversion" "pipeline" "none"]
}

def "nu-complete skillshare sync-mode" [] {
    ["merge" "copy" "symlink"]
}

def "nu-complete skillshare audit-threshold" [] {
    ["low" "medium" "high" "critical"]
}

def "nu-complete skillshare audit-format" [] {
    ["text" "json" "sarif" "markdown"]
}

def "nu-complete skillshare audit-profile" [] {
    ["default" "strict" "permissive"]
}

def "nu-complete skillshare list-type" [] {
    ["tracked" "local" "github"]
}

def "nu-complete skillshare list-sort" [] {
    ["name" "newest" "oldest"]
}

def "nu-complete skillshare list-status" [] {
    ["all" "enabled" "disabled"]
}

# Main command
export extern "skillshare" [
    command?: string@"nu-complete skillshare commands"
    --project(-p)    # Use project-level config
    --global(-g)     # Use global config
    --help(-h)       # Show help
]

# Init
export extern "skillshare init" [
    --source(-s): path       # Set source directory
    --remote: string         # Set git remote
    --copy-from(-c): string  # Copy skills from CLI directory
    --no-copy                # Start with empty source
    --targets(-t): string    # Comma-separated target names
    --all-targets            # Add all detected targets
    --no-targets             # Skip target setup
    --mode(-m): string@"nu-complete skillshare sync-mode"
    --git                    # Initialize git
    --no-git                 # Skip git initialization
    --skill                  # Install built-in skillshare skill
    --no-skill               # Skip built-in skill installation
    --discover(-d)           # Detect new AI CLI agents
    --select: string         # Select specific agents
    --subdir: string         # Use subdirectory as source
    --git-root: string       # Git repository scope (skills, agents, extras, root)
    --visible                # Project: create visible skillshare/ directory
    --config: string         # Project: "local" gitignores config.yaml
    --dry-run(-n)            # Preview without changes
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# Install
export extern "skillshare install" [
    source?: string          # Source path or git repo
    --name: string           # Custom skill name
    --force(-f)              # Overwrite existing
    --update(-u)             # Update if exists
    --dry-run(-n)            # Preview changes
    --skip-audit             # Skip security audit
    --audit-verbose          # Verbose audit output
    --audit-threshold: string@"nu-complete skillshare audit-threshold"
    --threshold(-T): string@"nu-complete skillshare audit-threshold"
    --branch(-b): string     # Checkout specific branch
    --track(-t)              # Track the repository
    --kind: string           # Filter by kind (skill or agent)
    --agent(-a): string      # Install specific agents
    --skill(-s): string      # Install specific skills
    --exclude: string        # Exclude items
    --into: string           # Custom destination path
    --all                    # Install all items
    --yes(-y)                # Skip confirmation
    --json                   # JSON output
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# Uninstall
export extern "skillshare uninstall" [
    scope?: string@"nu-complete skillshare kind" # agents or a skill name
    ...names: string         # Skill names to remove
    --all                    # Remove all skills
    --force(-f)              # Skip confirmation
    --dry-run(-n)            # Preview changes
    --json                   # JSON output
    --group(-G): string      # Uninstall by group
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# List
export extern "skillshare list" [
    scope?: string@"nu-complete skillshare kind"
    --verbose(-v)            # Show detailed information
    --json(-j)               # JSON output
    --no-tui                 # Skip interactive TUI
    --type(-t): string@"nu-complete skillshare list-type"
    --status: string@"nu-complete skillshare list-status"
    --sort(-s): string@"nu-complete skillshare list-sort"
    --all                    # List skills + agents
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# Sync
export extern "skillshare sync" [
    scope?: string@"nu-complete skillshare sync-scope"
    --all                    # Sync skills + agents + extras
    --dry-run(-n)            # Preview changes
    --force(-f)              # Force sync
    --json                   # JSON output
    --quiet(-q)              # Suppress token summary
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# Status
export extern "skillshare status" [
    --json                   # JSON output
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# Diff
export extern "skillshare diff" [
    target?: string@"nu-complete skillshare kind" # agents or a target name
    --no-tui                 # Skip interactive TUI
    --patch                  # Show unified diff patch
    --stat                   # Show statistics
    --json                   # JSON output
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# Backup
export extern "skillshare backup" [
    subcommand?: string@"nu-complete skillshare backup"
    --list(-l)               # List existing backups
    --cleanup(-c)            # Remove old backups
    --delete: string         # Delete one backup (timestamp)
    --all                    # Back up skills + agents
    --dry-run(-n)            # Preview changes
    --target(-t): string     # Backup specific target
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# Backup files
export extern "skillshare backup files" [
    command?: string@"nu-complete skillshare backup-files"
    path?: string
    id?: string
    --unlink                 # restore: replace a symlink with a regular file
    --dry-run(-n)            # Preview changes
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# Restore
export extern "skillshare restore" [
    target?: string@"nu-complete skillshare kind" # agents or a target name
    --from(-f): string       # Restore from timestamp
    --force                  # Overwrite without confirmation
    --all                    # Restore skills + agents
    --dry-run(-n)            # Preview changes
    --no-tui                 # Skip interactive TUI
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# Collect
export extern "skillshare collect" [
    scope?: string@"nu-complete skillshare kind" # agents or target name
    --all(-a)                # Collect from all targets
    --dry-run(-n)            # Preview changes
    --force(-f)              # Overwrite existing
    --json                   # JSON output
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# Pull
export extern "skillshare pull" [
    --dry-run(-n)            # Preview changes
    --force(-f)              # Force pull
    --help(-h)               # Show help
]

# Push
export extern "skillshare push" [
    --dry-run(-n)            # Preview changes
    --pull                   # Merge remote changes before pushing, then sync
    --message(-m): string    # Commit message
    --help(-h)               # Show help
]

# Commit
export extern "skillshare commit" [
    --dry-run(-n)            # Preview changes
    --message(-m): string    # Commit message
    --help(-h)               # Show help
]

# Doctor
export extern "skillshare doctor" [
    --json                   # JSON output
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# Target
export extern "skillshare target" [
    subcommand?: string@"nu-complete skillshare target"
    --json                   # JSON output
    --no-tui                 # Skip interactive TUI
    --mode(-m): string@"nu-complete skillshare sync-mode"
    --agent-mode: string@"nu-complete skillshare sync-mode"
    --target-naming: string  # Set naming (flat or standard)
    --add-include: string    # Add include filter
    --add-exclude: string    # Add exclude filter
    --remove-include: string # Remove include filter
    --remove-exclude: string # Remove exclude filter
    --add-agent-include: string    # Add agent include filter
    --add-agent-exclude: string    # Add agent exclude filter
    --remove-agent-include: string # Remove agent include filter
    --remove-agent-exclude: string # Remove agent exclude filter
    --agent: string@"nu-complete skillshare account-agent" # With add: the Agent this is another account of
    --config-dir: string     # With add: that account's config directory
    --cli: string            # With add: the executable that runs its plugin commands
    --skills: string         # Sync skills to this target (true or false)
    --no-skills              # With add: do not sync skills to the new target
    --all(-a)                # With remove: remove all targets
    --dry-run(-n)            # With --skills=false: preview removed links
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# Upgrade
export extern "skillshare upgrade" [
    --dry-run(-n)            # Preview changes
    --force(-f)              # Force upgrade
    --skill                  # Install built-in skill
    --cli                    # Upgrade CLI
    --help(-h)               # Show help
]

# Update
export extern "skillshare update" [
    name?: string@"nu-complete skillshare kind" # agents or a skill/agent name
    --all(-a)                # Update all
    --dry-run(-n)            # Preview changes
    --force(-f)              # Force update
    --skip-audit             # Skip security audit
    --audit-threshold: string@"nu-complete skillshare audit-threshold"
    --threshold(-T): string@"nu-complete skillshare audit-threshold"
    --diff                   # Show changes
    --audit-verbose          # Verbose audit output
    --prune                  # Prune items
    --json                   # JSON output
    --group(-G): string      # Update by group
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# Check
export extern "skillshare check" [
    scope?: string@"nu-complete skillshare kind"
    --json                   # JSON output
    --all                    # Check all
    --group(-G): string      # Check by group
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# New
export extern "skillshare new" [
    name?: string            # Skill name
    --pattern(-P): string@"nu-complete skillshare new-pattern"
    --dry-run(-n)            # Preview changes
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# Search
export extern "skillshare search" [
    query?: string           # Search query
    --json                   # JSON output
    --list(-l)               # List results only
    --hub                    # Search a hub index (URL optional)
    --limit(-n): int         # Maximum results
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# Trash
export extern "skillshare trash" [
    subcommand?: string@"nu-complete skillshare trash"
    name?: string            # Skill name
    --all                    # Include skills + agents
    --no-tui                 # Skip interactive TUI
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# Audit
export extern "skillshare audit" [
    subcommand?: string@"nu-complete skillshare audit"
    --init-rules             # Initialize audit rules
    --json                   # JSON output
    --format: string@"nu-complete skillshare audit-format"
    --quiet(-q)              # Suppress output
    --yes(-y)                # Skip confirmation
    --no-tui                 # Skip interactive TUI
    --threshold(-T): string@"nu-complete skillshare audit-threshold"
    --group(-G): string      # Filter by group
    --profile: string@"nu-complete skillshare audit-profile"
    --dedupe: string         # Deduplication mode (legacy, global)
    --analyzer: string       # Enable analyzer
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# Audit rules
export extern "skillshare audit rules" [
    command?: string@"nu-complete skillshare audit-rules"
    id?: string
    level?: string
    --pattern: string        # Filter by pattern name
    --severity: string       # Filter by minimum severity
    --disabled               # Only show disabled rules
    --format: string         # Output format (json)
    --no-tui                 # Skip interactive TUI
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# Hub
export extern "skillshare hub" [
    subcommand?: string@"nu-complete skillshare hub"
    name?: string
    --source(-s): path       # Source directory (hub index)
    --output(-o): path       # Output path (hub index)
    --full                   # Full index (hub index)
    --audit                  # Include audit risk scores (hub index)
    --label(-l): string      # Label for the hub (hub add)
    --reset                  # Clear default hub (hub default)
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# Log
export extern "skillshare log" [
    --audit(-a)              # Show audit logs
    --clear(-c)              # Clear logs
    --json                   # JSON output
    --no-tui                 # Skip interactive TUI
    --stats                  # Show statistics
    --cmd: string            # Filter by command
    --status: string         # Filter by status (ok, error)
    --since: string          # Filter by date
    --tail(-t): int          # Show last N entries
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# UI
export extern "skillshare ui" [
    subcommand?: string@"nu-complete skillshare ui"
    --port: int              # Set port
    --host: string           # Set host
    --base-path(-b): string  # Base path prefix for reverse proxy
    --no-open                # Do not open browser
    --clear-cache            # Clear cached UI assets
    --app                    # Open as app window (start only)
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# TUI
export extern "skillshare tui" [
    state?: string@"nu-complete skillshare tui"
    --help(-h)               # Show help
]

# Extras
export extern "skillshare extras" [
    subcommand?: string@"nu-complete skillshare extras"
    name?: string
    --mode: string           # Sync mode (merge, copy, symlink, import)
    --target: string         # Target directory
    --source: string         # init: custom source directory
    --file: string           # init: single-file extra
    --as: string             # Target filename
    --flatten                # Flatten subdirectory files
    --no-flatten             # Disable flatten
    --add-target: string     # Add a target
    --remove-target: string  # Detach a target
    --prune                  # With --remove-target: delete managed files
    --from: string           # collect: target directory
    --dry-run                # collect: preview changes
    --force(-f)              # Overwrite existing
    --json                   # list: JSON output
    --no-tui                 # Skip interactive TUI
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

export extern "skillshare extras memory" [
    command?: string@[init list show write delete instructions]
    note?: string            # Relative Markdown path
    --from: string           # Input file or - for stdin
    --version: string        # Last read hash for updates
    --search: string         # list: search names and content
    --update-mode: string    # instructions: passive or active
    --json                   # JSON output
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# Enable
export extern "skillshare enable" [
    name: string             # Skill/agent name or pattern
    --kind: string@"nu-complete skillshare kind-flag"
    --dry-run(-n)            # Preview changes
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# Disable
export extern "skillshare disable" [
    name: string             # Skill/agent name or pattern
    --kind: string@"nu-complete skillshare kind-flag"
    --dry-run(-n)            # Preview changes
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# Analyze
export extern "skillshare analyze" [
    --verbose(-v)            # Show detailed information
    --filter: string         # Filter skills by name or path
    --no-tui                 # Skip interactive TUI
    --json                   # JSON output
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# Completion
export extern "skillshare completion" [
    shell?: string@"nu-complete skillshare completion"
    --install                # Install completion script
    --help(-h)               # Show help
]

# Version
export extern "skillshare version" []

# Help
export extern "skillshare help" []
`
