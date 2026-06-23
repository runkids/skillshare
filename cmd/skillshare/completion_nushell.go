package main

const nushellCompletionScript = `# skillshare Nushell completion

def "nu-complete skillshare commands" [] {
    [
        { value: "init", description: "Initialize skillshare" }
        { value: "install", description: "Install skills/agents from local path or git repo" }
        { value: "uninstall", description: "Remove skills/agents from source directory" }
        { value: "list", description: "List installed skills" }
        { value: "search", description: "Search or browse GitHub for skills" }
        { value: "sync", description: "Sync skills/agents/extras to targets" }
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
        { value: "mode", description: "Change sync mode or flatten" }
    ]
}

def "nu-complete skillshare audit" [] {
    [
        { value: "rules", description: "Manage security rules" }
    ]
}

def "nu-complete skillshare backup" [] {
    [
        { value: "restore", description: "Restore from backup" }
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
    --dry-run(-n)            # Preview without changes
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# Install
export extern "skillshare install" [
    source?: string          # Source path or git repo
    --source(-s): path       # Set source directory
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
    --skill: string          # Install specific skills
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
    scope?: string           # agents
    --verbose(-v)            # Show detailed information
    --json(-j)               # JSON output
    --no-tui                 # Skip interactive TUI
    --type(-t): string@"nu-complete skillshare list-type"
    --sort(-s): string@"nu-complete skillshare list-sort"
    --all                    # List skills + agents
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# Sync
export extern "skillshare sync" [
    scope?: string           # agents, extras
    --all                    # Sync skills + agents + extras
    --dry-run(-n)            # Preview changes
    --force(-f)              # Force sync
    --json                   # JSON output
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# Diff
export extern "skillshare diff" [
    target?: string          # Target name
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
    --dry-run(-n)            # Preview changes
    --target(-t): string     # Backup specific target
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# Collect
export extern "skillshare collect" [
    scope?: string           # agents or target name
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
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
]

# Push
export extern "skillshare push" [
    --dry-run(-n)            # Preview changes
    --message(-m): string    # Commit message
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
]

# Commit
export extern "skillshare commit" [
    --dry-run(-n)            # Preview changes
    --message(-m): string    # Commit message
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# Doctor
export extern "skillshare doctor" [
    --json                   # JSON output
    --help(-h)               # Show help
]

# Target
export extern "skillshare target" [
    subcommand?: string@"nu-complete skillshare target"
    --json                   # JSON output
    --no-tui                 # Skip interactive TUI
    --mode: string@"nu-complete skillshare sync-mode"
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
    name?: string            # Skill/agent name
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
    scope?: string           # agents
    --json                   # JSON output
    --all                    # Check all
    --group(-G): string      # Check by group
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# Trash
export extern "skillshare trash" [
    subcommand?: string@"nu-complete skillshare trash"
    name?: string            # Skill name
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

# Hub
export extern "skillshare hub" [
    subcommand?: string@"nu-complete skillshare hub"
    name?: string
    --source(-s): path       # Source directory (hub index)
    --output(-o): path       # Output path (hub index)
    --full                   # Full index (hub index)
    --audit-skills           # Audit skills (hub index)
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
    --port: int              # Set port
    --host: string           # Set host
    --no-open                # Do not open browser
    --help(-h)               # Show help
]

# TUI
export extern "skillshare tui" [
    state?: string@"nu-complete skillshare tui"
]

# Extras
export extern "skillshare extras" [
    subcommand?: string@"nu-complete skillshare extras"
    name?: string
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# Enable
export extern "skillshare enable" [
    name: string             # Skill/agent name or pattern
    --dry-run(-n)            # Preview changes
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# Disable
export extern "skillshare disable" [
    name: string             # Skill/agent name or pattern
    --dry-run(-n)            # Preview changes
    --project(-p)            # Use project-level config
    --global(-g)             # Use global config
    --help(-h)               # Show help
]

# Analyze
export extern "skillshare analyze" [
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
