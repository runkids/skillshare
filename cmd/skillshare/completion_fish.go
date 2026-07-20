package main

const fishCompletionScript = `# skillshare fish completion

# Disable file completions by default
complete -c skillshare -f

# Helper: no subcommand yet
function __fish_skillshare_no_subcommand
    set -l cmd (commandline -opc)
    test (count $cmd) -eq 1
end

# Helper: current subcommand matches
function __fish_skillshare_using_command
    set -l cmd (commandline -opc)
    test (count $cmd) -gt 1; and test "$cmd[2]" = "$argv[1]"
end

# Helper: subcommand at position 3 matches
function __fish_skillshare_using_subcommand
    set -l cmd (commandline -opc)
    test (count $cmd) -gt 2; and test "$cmd[2]" = "$argv[1]"; and test "$cmd[3]" = "$argv[2]"
end

# Commands
complete -c skillshare -n __fish_skillshare_no_subcommand -a init -d 'Initialize skillshare'
complete -c skillshare -n __fish_skillshare_no_subcommand -a install -d 'Install skills/agents from local path or git repo'
complete -c skillshare -n __fish_skillshare_no_subcommand -a uninstall -d 'Remove skills/agents from source directory'
complete -c skillshare -n __fish_skillshare_no_subcommand -a list -d 'List installed skills'
complete -c skillshare -n __fish_skillshare_no_subcommand -a search -d 'Search or browse GitHub for skills'
complete -c skillshare -n __fish_skillshare_no_subcommand -a sync -d 'Sync skills/agents/extras to targets'
complete -c skillshare -n __fish_skillshare_no_subcommand -a status -d 'Show status of all targets'
complete -c skillshare -n __fish_skillshare_no_subcommand -a diff -d 'Show differences between source and targets'
complete -c skillshare -n __fish_skillshare_no_subcommand -a backup -d 'Create backup of targets'
complete -c skillshare -n __fish_skillshare_no_subcommand -a restore -d 'Restore target from backup'
complete -c skillshare -n __fish_skillshare_no_subcommand -a collect -d 'Collect local skills/agents from targets'
complete -c skillshare -n __fish_skillshare_no_subcommand -a adopt -d 'Adopt externally installed skills into skillshare'
complete -c skillshare -n __fish_skillshare_no_subcommand -a pull -d 'Pull from git remote and sync'
complete -c skillshare -n __fish_skillshare_no_subcommand -a push -d 'Commit and push source to git remote'
complete -c skillshare -n __fish_skillshare_no_subcommand -a commit -d 'Create local git commit without pushing'
complete -c skillshare -n __fish_skillshare_no_subcommand -a doctor -d 'Check environment and diagnose issues'
complete -c skillshare -n __fish_skillshare_no_subcommand -a target -d 'Manage targets'
complete -c skillshare -n __fish_skillshare_no_subcommand -a upgrade -d 'Upgrade CLI and/or skillshare skill'
complete -c skillshare -n __fish_skillshare_no_subcommand -a update -d 'Update skills/agents or tracked repositories'
complete -c skillshare -n __fish_skillshare_no_subcommand -a check -d 'Check for available updates'
complete -c skillshare -n __fish_skillshare_no_subcommand -a new -d 'Create a new skill'
complete -c skillshare -n __fish_skillshare_no_subcommand -a trash -d 'Manage trashed skills/agents'
complete -c skillshare -n __fish_skillshare_no_subcommand -a analyze -d 'Analyze skills/agents'
complete -c skillshare -n __fish_skillshare_no_subcommand -a audit -d 'Scan skills/agents for security threats'
complete -c skillshare -n __fish_skillshare_no_subcommand -a hub -d 'Manage hubs'
complete -c skillshare -n __fish_skillshare_no_subcommand -a log -d 'View operation log'
complete -c skillshare -n __fish_skillshare_no_subcommand -a ui -d 'Launch web dashboard'
complete -c skillshare -n __fish_skillshare_no_subcommand -a tui -d 'Toggle interactive TUI mode'
complete -c skillshare -n __fish_skillshare_no_subcommand -a extras -d 'Manage extra resource types'
complete -c skillshare -n __fish_skillshare_no_subcommand -a enable -d 'Enable a disabled skill/agent'
complete -c skillshare -n __fish_skillshare_no_subcommand -a disable -d 'Disable a skill/agent'
complete -c skillshare -n __fish_skillshare_no_subcommand -a completion -d 'Generate shell completion scripts'
complete -c skillshare -n __fish_skillshare_no_subcommand -a version -d 'Show version'
complete -c skillshare -n __fish_skillshare_no_subcommand -a help -d 'Show help'

# Global flags
complete -c skillshare -l project -s p -d 'Use project-level config'
complete -c skillshare -l global -s g -d 'Use global config'

# init
complete -c skillshare -n '__fish_skillshare_using_command init' -l source -s s -r -d 'Set source directory'
complete -c skillshare -n '__fish_skillshare_using_command init' -l remote -r -d 'Set git remote'
complete -c skillshare -n '__fish_skillshare_using_command init' -l copy-from -s c -r -d 'Copy skills from CLI directory'
complete -c skillshare -n '__fish_skillshare_using_command init' -l no-copy -d 'Start with empty source'
complete -c skillshare -n '__fish_skillshare_using_command init' -l targets -s t -r -d 'Comma-separated target names'
complete -c skillshare -n '__fish_skillshare_using_command init' -l all-targets -d 'Add all detected targets'
complete -c skillshare -n '__fish_skillshare_using_command init' -l no-targets -d 'Skip target setup'
complete -c skillshare -n '__fish_skillshare_using_command init' -l mode -s m -r -a 'merge copy symlink' -d 'Set sync mode'
complete -c skillshare -n '__fish_skillshare_using_command init' -l git -d 'Initialize git'
complete -c skillshare -n '__fish_skillshare_using_command init' -l no-git -d 'Skip git initialization'
complete -c skillshare -n '__fish_skillshare_using_command init' -l skill -d 'Install built-in skillshare skill'
complete -c skillshare -n '__fish_skillshare_using_command init' -l no-skill -d 'Skip built-in skill installation'
complete -c skillshare -n '__fish_skillshare_using_command init' -l discover -s d -d 'Detect new AI CLI agents'
complete -c skillshare -n '__fish_skillshare_using_command init' -l select -r -d 'Select specific agents'
complete -c skillshare -n '__fish_skillshare_using_command init' -l subdir -r -d 'Use subdirectory as source'
complete -c skillshare -n '__fish_skillshare_using_command init' -l dry-run -s n -d 'Preview without changes'
complete -c skillshare -n '__fish_skillshare_using_command init' -l help -s h -d 'Show help'

# install
complete -c skillshare -n '__fish_skillshare_using_command install' -l source -s s -r -d 'Set source directory'
complete -c skillshare -n '__fish_skillshare_using_command install' -l name -r -d 'Custom skill name'
complete -c skillshare -n '__fish_skillshare_using_command install' -l force -s f -d 'Overwrite existing'
complete -c skillshare -n '__fish_skillshare_using_command install' -l update -s u -d 'Update if exists'
complete -c skillshare -n '__fish_skillshare_using_command install' -l dry-run -s n -d 'Preview changes'
complete -c skillshare -n '__fish_skillshare_using_command install' -l skip-audit -d 'Skip security audit'
complete -c skillshare -n '__fish_skillshare_using_command install' -l audit-verbose -d 'Verbose audit output'
complete -c skillshare -n '__fish_skillshare_using_command install' -l audit-threshold -r -a 'low medium high critical' -d 'Set audit threshold'
complete -c skillshare -n '__fish_skillshare_using_command install' -l threshold -s T -r -a 'low medium high critical' -d 'Set audit threshold'
complete -c skillshare -n '__fish_skillshare_using_command install' -l branch -s b -r -d 'Checkout specific branch'
complete -c skillshare -n '__fish_skillshare_using_command install' -l track -s t -d 'Track the repository'
complete -c skillshare -n '__fish_skillshare_using_command install' -l kind -r -a 'skill agent' -d 'Filter by kind'
complete -c skillshare -n '__fish_skillshare_using_command install' -l agent -s a -r -d 'Install specific agents'
complete -c skillshare -n '__fish_skillshare_using_command install' -l skill -r -d 'Install specific skills'
complete -c skillshare -n '__fish_skillshare_using_command install' -l exclude -r -d 'Exclude items'
complete -c skillshare -n '__fish_skillshare_using_command install' -l into -r -d 'Custom destination path'
complete -c skillshare -n '__fish_skillshare_using_command install' -l all -d 'Install all items'
complete -c skillshare -n '__fish_skillshare_using_command install' -l yes -s y -d 'Skip confirmation'
complete -c skillshare -n '__fish_skillshare_using_command install' -l json -d 'JSON output'
complete -c skillshare -n '__fish_skillshare_using_command install' -l help -s h -d 'Show help'

# uninstall
complete -c skillshare -n '__fish_skillshare_using_command uninstall' -l all -d 'Remove all skills'
complete -c skillshare -n '__fish_skillshare_using_command uninstall' -l force -s f -d 'Skip confirmation'
complete -c skillshare -n '__fish_skillshare_using_command uninstall' -l dry-run -s n -d 'Preview changes'
complete -c skillshare -n '__fish_skillshare_using_command uninstall' -l json -d 'JSON output'
complete -c skillshare -n '__fish_skillshare_using_command uninstall' -l group -s G -r -d 'Uninstall by group'
complete -c skillshare -n '__fish_skillshare_using_command uninstall' -l help -s h -d 'Show help'

# list
complete -c skillshare -n '__fish_skillshare_using_command list' -a agents -d 'List agents'
complete -c skillshare -n '__fish_skillshare_using_command list' -l verbose -s v -d 'Show detailed information'
complete -c skillshare -n '__fish_skillshare_using_command list' -l json -s j -d 'JSON output'
complete -c skillshare -n '__fish_skillshare_using_command list' -l no-tui -d 'Skip interactive TUI'
complete -c skillshare -n '__fish_skillshare_using_command list' -l type -s t -r -a 'tracked local github' -d 'Filter by type'
complete -c skillshare -n '__fish_skillshare_using_command list' -l sort -s s -r -a 'name newest oldest' -d 'Sort by'
complete -c skillshare -n '__fish_skillshare_using_command list' -l all -d 'List skills + agents'
complete -c skillshare -n '__fish_skillshare_using_command list' -l help -s h -d 'Show help'

# sync
complete -c skillshare -n '__fish_skillshare_using_command sync' -a 'agents extras' -d 'Sync scope'
complete -c skillshare -n '__fish_skillshare_using_command sync' -l all -d 'Sync skills + agents + extras'
complete -c skillshare -n '__fish_skillshare_using_command sync' -l dry-run -s n -d 'Preview changes'
complete -c skillshare -n '__fish_skillshare_using_command sync' -l force -s f -d 'Force sync'
complete -c skillshare -n '__fish_skillshare_using_command sync' -l json -d 'JSON output'
complete -c skillshare -n '__fish_skillshare_using_command sync' -l help -s h -d 'Show help'

# diff
complete -c skillshare -n '__fish_skillshare_using_command diff' -l no-tui -d 'Skip interactive TUI'
complete -c skillshare -n '__fish_skillshare_using_command diff' -l patch -d 'Show unified diff patch'
complete -c skillshare -n '__fish_skillshare_using_command diff' -l stat -d 'Show statistics'
complete -c skillshare -n '__fish_skillshare_using_command diff' -l json -d 'JSON output'
complete -c skillshare -n '__fish_skillshare_using_command diff' -l help -s h -d 'Show help'

# backup
complete -c skillshare -n '__fish_skillshare_using_command backup' -a restore -d 'Restore from backup'
complete -c skillshare -n '__fish_skillshare_using_command backup' -l list -s l -d 'List existing backups'
complete -c skillshare -n '__fish_skillshare_using_command backup' -l cleanup -s c -d 'Remove old backups'
complete -c skillshare -n '__fish_skillshare_using_command backup' -l dry-run -s n -d 'Preview changes'
complete -c skillshare -n '__fish_skillshare_using_command backup' -l target -s t -r -d 'Backup specific target'
complete -c skillshare -n '__fish_skillshare_using_command backup' -l help -s h -d 'Show help'

# collect
complete -c skillshare -n '__fish_skillshare_using_command collect' -a agents -d 'Collect agents'
complete -c skillshare -n '__fish_skillshare_using_command collect' -l all -s a -d 'Collect from all targets'
complete -c skillshare -n '__fish_skillshare_using_command collect' -l dry-run -s n -d 'Preview changes'
complete -c skillshare -n '__fish_skillshare_using_command collect' -l force -s f -d 'Overwrite existing'
complete -c skillshare -n '__fish_skillshare_using_command collect' -l json -d 'JSON output'
complete -c skillshare -n '__fish_skillshare_using_command collect' -l help -s h -d 'Show help'

# adopt
complete -c skillshare -n '__fish_skillshare_using_command adopt' -l all -s a -d 'Adopt all detected skills'
complete -c skillshare -n '__fish_skillshare_using_command adopt' -l dry-run -s n -d 'Preview without changes'
complete -c skillshare -n '__fish_skillshare_using_command adopt' -l force -s f -d 'Overwrite same-name source skills'
complete -c skillshare -n '__fish_skillshare_using_command adopt' -l json -d 'JSON output without prompting'
complete -c skillshare -n '__fish_skillshare_using_command adopt' -l help -s h -d 'Show help'

# pull
complete -c skillshare -n '__fish_skillshare_using_command pull' -l dry-run -s n -d 'Preview changes'
complete -c skillshare -n '__fish_skillshare_using_command pull' -l force -s f -d 'Force pull'

# push
complete -c skillshare -n '__fish_skillshare_using_command push' -l dry-run -s n -d 'Preview changes'
complete -c skillshare -n '__fish_skillshare_using_command push' -l message -s m -r -d 'Commit message'

# commit
complete -c skillshare -n '__fish_skillshare_using_command commit' -l dry-run -s n -d 'Preview changes'
complete -c skillshare -n '__fish_skillshare_using_command commit' -l message -s m -r -d 'Commit message'
complete -c skillshare -n '__fish_skillshare_using_command commit' -l help -s h -d 'Show help'

# doctor
complete -c skillshare -n '__fish_skillshare_using_command doctor' -l json -d 'JSON output'
complete -c skillshare -n '__fish_skillshare_using_command doctor' -l help -s h -d 'Show help'

# target subcommands
complete -c skillshare -n '__fish_skillshare_using_command target' -a add -d 'Add a target'
complete -c skillshare -n '__fish_skillshare_using_command target' -a remove -d 'Unlink target and restore skills'
complete -c skillshare -n '__fish_skillshare_using_command target' -a list -d 'List all targets'
complete -c skillshare -n '__fish_skillshare_using_command target' -l json -d 'JSON output'
complete -c skillshare -n '__fish_skillshare_using_command target' -l no-tui -d 'Skip interactive TUI'
complete -c skillshare -n '__fish_skillshare_using_command target' -l mode -r -a 'merge copy symlink' -d 'Set sync mode'
complete -c skillshare -n '__fish_skillshare_using_command target' -l agent-mode -r -a 'merge copy symlink' -d 'Set agents sync mode'
complete -c skillshare -n '__fish_skillshare_using_command target' -l target-naming -r -a 'flat standard' -d 'Set naming'
complete -c skillshare -n '__fish_skillshare_using_command target' -l help -s h -d 'Show help'

# target remove
complete -c skillshare -n '__fish_skillshare_using_subcommand target remove' -l all -s a -d 'Remove all targets'
complete -c skillshare -n '__fish_skillshare_using_subcommand target remove' -l dry-run -s n -d 'Preview changes'

# upgrade
complete -c skillshare -n '__fish_skillshare_using_command upgrade' -l dry-run -s n -d 'Preview changes'
complete -c skillshare -n '__fish_skillshare_using_command upgrade' -l force -s f -d 'Force upgrade'
complete -c skillshare -n '__fish_skillshare_using_command upgrade' -l skill -d 'Install built-in skill'
complete -c skillshare -n '__fish_skillshare_using_command upgrade' -l cli -d 'Upgrade CLI'
complete -c skillshare -n '__fish_skillshare_using_command upgrade' -l help -s h -d 'Show help'

# update
complete -c skillshare -n '__fish_skillshare_using_command update' -a agents -d 'Update agents'
complete -c skillshare -n '__fish_skillshare_using_command update' -l all -s a -d 'Update all'
complete -c skillshare -n '__fish_skillshare_using_command update' -l dry-run -s n -d 'Preview changes'
complete -c skillshare -n '__fish_skillshare_using_command update' -l force -s f -d 'Force update'
complete -c skillshare -n '__fish_skillshare_using_command update' -l skip-audit -d 'Skip security audit'
complete -c skillshare -n '__fish_skillshare_using_command update' -l audit-threshold -r -a 'low medium high critical' -d 'Set audit threshold'
complete -c skillshare -n '__fish_skillshare_using_command update' -l threshold -s T -r -a 'low medium high critical' -d 'Set audit threshold'
complete -c skillshare -n '__fish_skillshare_using_command update' -l diff -d 'Show changes'
complete -c skillshare -n '__fish_skillshare_using_command update' -l audit-verbose -d 'Verbose audit output'
complete -c skillshare -n '__fish_skillshare_using_command update' -l prune -d 'Prune items'
complete -c skillshare -n '__fish_skillshare_using_command update' -l json -d 'JSON output'
complete -c skillshare -n '__fish_skillshare_using_command update' -l group -s G -r -d 'Update by group'
complete -c skillshare -n '__fish_skillshare_using_command update' -l help -s h -d 'Show help'

# check
complete -c skillshare -n '__fish_skillshare_using_command check' -a agents -d 'Check agents'
complete -c skillshare -n '__fish_skillshare_using_command check' -l json -d 'JSON output'
complete -c skillshare -n '__fish_skillshare_using_command check' -l all -d 'Check all'
complete -c skillshare -n '__fish_skillshare_using_command check' -l group -s G -r -d 'Check by group'
complete -c skillshare -n '__fish_skillshare_using_command check' -l help -s h -d 'Show help'

# trash subcommands
complete -c skillshare -n '__fish_skillshare_using_command trash' -a list -d 'List trashed items'
complete -c skillshare -n '__fish_skillshare_using_command trash' -a restore -d 'Restore from trash'
complete -c skillshare -n '__fish_skillshare_using_command trash' -a delete -d 'Delete permanently'
complete -c skillshare -n '__fish_skillshare_using_command trash' -a empty -d 'Clear all trash'
complete -c skillshare -n '__fish_skillshare_using_command trash' -l no-tui -d 'Skip interactive TUI'
complete -c skillshare -n '__fish_skillshare_using_command trash' -l help -s h -d 'Show help'

# audit
complete -c skillshare -n '__fish_skillshare_using_command audit' -a rules -d 'Manage security rules'
complete -c skillshare -n '__fish_skillshare_using_command audit' -a agents -d 'Audit agents'
complete -c skillshare -n '__fish_skillshare_using_command audit' -l init-rules -d 'Initialize audit rules'
complete -c skillshare -n '__fish_skillshare_using_command audit' -l json -d 'JSON output'
complete -c skillshare -n '__fish_skillshare_using_command audit' -l format -r -a 'text json sarif markdown' -d 'Output format'
complete -c skillshare -n '__fish_skillshare_using_command audit' -l quiet -s q -d 'Suppress output'
complete -c skillshare -n '__fish_skillshare_using_command audit' -l yes -s y -d 'Skip confirmation'
complete -c skillshare -n '__fish_skillshare_using_command audit' -l no-tui -d 'Skip interactive TUI'
complete -c skillshare -n '__fish_skillshare_using_command audit' -l threshold -s T -r -a 'low medium high critical' -d 'Block threshold'
complete -c skillshare -n '__fish_skillshare_using_command audit' -l group -s G -r -d 'Filter by group'
complete -c skillshare -n '__fish_skillshare_using_command audit' -l profile -r -a 'default strict permissive' -d 'Security profile'
complete -c skillshare -n '__fish_skillshare_using_command audit' -l dedupe -r -a 'legacy global' -d 'Deduplication mode'
complete -c skillshare -n '__fish_skillshare_using_command audit' -l analyzer -r -a 'static dataflow tier integrity metadata structure cross-skill' -d 'Enable analyzer'
complete -c skillshare -n '__fish_skillshare_using_command audit' -l help -s h -d 'Show help'

# hub subcommands
complete -c skillshare -n '__fish_skillshare_using_command hub' -a add -d 'Add hub'
complete -c skillshare -n '__fish_skillshare_using_command hub' -a list -d 'List hubs'
complete -c skillshare -n '__fish_skillshare_using_command hub' -a remove -d 'Remove hub'
complete -c skillshare -n '__fish_skillshare_using_command hub' -a default -d 'Set default hub'
complete -c skillshare -n '__fish_skillshare_using_command hub' -a index -d 'Create skill index'
complete -c skillshare -n '__fish_skillshare_using_command hub' -a help -d 'Show help'
complete -c skillshare -n '__fish_skillshare_using_command hub' -l help -s h -d 'Show help'

# hub index
complete -c skillshare -n '__fish_skillshare_using_subcommand hub index' -l source -s s -r -d 'Source directory'
complete -c skillshare -n '__fish_skillshare_using_subcommand hub index' -l output -s o -r -d 'Output path'
complete -c skillshare -n '__fish_skillshare_using_subcommand hub index' -l full -d 'Full index'
complete -c skillshare -n '__fish_skillshare_using_subcommand hub index' -l audit-skills -d 'Audit skills'

# log
complete -c skillshare -n '__fish_skillshare_using_command log' -l audit -s a -d 'Show audit logs'
complete -c skillshare -n '__fish_skillshare_using_command log' -l clear -s c -d 'Clear logs'
complete -c skillshare -n '__fish_skillshare_using_command log' -l json -d 'JSON output'
complete -c skillshare -n '__fish_skillshare_using_command log' -l no-tui -d 'Skip interactive TUI'
complete -c skillshare -n '__fish_skillshare_using_command log' -l stats -d 'Show statistics'
complete -c skillshare -n '__fish_skillshare_using_command log' -l cmd -r -d 'Filter by command'
complete -c skillshare -n '__fish_skillshare_using_command log' -l status -r -a 'ok error' -d 'Filter by status'
complete -c skillshare -n '__fish_skillshare_using_command log' -l since -r -d 'Filter by date'
complete -c skillshare -n '__fish_skillshare_using_command log' -l tail -s t -r -d 'Show last N entries'
complete -c skillshare -n '__fish_skillshare_using_command log' -l help -s h -d 'Show help'

# ui
complete -c skillshare -n '__fish_skillshare_using_command ui' -l port -r -d 'Set port'
complete -c skillshare -n '__fish_skillshare_using_command ui' -l host -r -d 'Set host'
complete -c skillshare -n '__fish_skillshare_using_command ui' -l no-open -d 'Do not open browser'
complete -c skillshare -n '__fish_skillshare_using_command ui' -l help -s h -d 'Show help'

# tui
complete -c skillshare -n '__fish_skillshare_using_command tui' -a 'on off' -d 'Toggle TUI mode'

# extras subcommands
complete -c skillshare -n '__fish_skillshare_using_command extras' -a init -d 'Create extra resource type'
complete -c skillshare -n '__fish_skillshare_using_command extras' -a list -d 'List extras with sync status'
complete -c skillshare -n '__fish_skillshare_using_command extras' -a remove -d 'Remove extra resource type'
complete -c skillshare -n '__fish_skillshare_using_command extras' -a collect -d 'Collect local files into extras'
complete -c skillshare -n '__fish_skillshare_using_command extras' -a source -d 'Show/set extras source'
complete -c skillshare -n '__fish_skillshare_using_command extras' -a mode -d 'Change sync mode or flatten'
complete -c skillshare -n '__fish_skillshare_using_command extras' -l help -s h -d 'Show help'

# enable
complete -c skillshare -n '__fish_skillshare_using_command enable' -l dry-run -s n -d 'Preview changes'
complete -c skillshare -n '__fish_skillshare_using_command enable' -l help -s h -d 'Show help'

# disable
complete -c skillshare -n '__fish_skillshare_using_command disable' -l dry-run -s n -d 'Preview changes'
complete -c skillshare -n '__fish_skillshare_using_command disable' -l help -s h -d 'Show help'

# analyze
complete -c skillshare -n '__fish_skillshare_using_command analyze' -l no-tui -d 'Skip interactive TUI'
complete -c skillshare -n '__fish_skillshare_using_command analyze' -l json -d 'JSON output'
complete -c skillshare -n '__fish_skillshare_using_command analyze' -l help -s h -d 'Show help'

# completion
complete -c skillshare -n '__fish_skillshare_using_command completion' -a 'bash zsh fish powershell nushell' -d 'Shell type'
complete -c skillshare -n '__fish_skillshare_using_command completion' -l install -d 'Install completion script'
complete -c skillshare -n '__fish_skillshare_using_command completion' -l help -s h -d 'Show help'
`
