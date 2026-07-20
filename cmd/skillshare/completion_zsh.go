package main

const zshCompletionScript = `#compdef skillshare

_skillshare() {
    local -a commands
    commands=(
        'init:Initialize skillshare'
        'install:Install skills/agents from local path or git repo'
        'uninstall:Remove skills/agents from source directory'
        'list:List installed skills'
        'search:Search or browse GitHub for skills'
        'sync:Sync skills/agents/extras to targets'
        'status:Show status of all targets'
        'diff:Show differences between source and targets'
        'backup:Create backup of targets'
        'restore:Restore target from backup'
        'collect:Collect local skills/agents from targets'
        'adopt:Adopt externally installed skills into skillshare'
        'pull:Pull from git remote and sync'
        'push:Commit and push source to git remote'
        'commit:Create local git commit without pushing'
        'doctor:Check environment and diagnose issues'
        'target:Manage targets'
        'upgrade:Upgrade CLI and/or skillshare skill'
        'update:Update skills/agents or tracked repositories'
        'check:Check for available updates'
        'new:Create a new skill'
        'trash:Manage trashed skills/agents'
        'analyze:Analyze skills/agents'
        'audit:Scan skills/agents for security threats'
        'hub:Manage hubs'
        'log:View operation log'
        'ui:Launch web dashboard'
        'tui:Toggle interactive TUI mode'
        'extras:Manage extra resource types'
        'enable:Enable a disabled skill/agent'
        'disable:Disable a skill/agent'
        'completion:Generate shell completion scripts'
        'version:Show version'
        'help:Show help'
    )

    local -a global_flags
    global_flags=(
        '--project[Use project-level config]'
        '-p[Use project-level config]'
        '--global[Use global config]'
        '-g[Use global config]'
    )

    _arguments -C \
        '1:command:->command' \
        '*::arg:->args'

    case $state in
        command)
            _describe 'command' commands
            ;;
        args)
            case ${words[1]} in
                init)
                    _arguments \
                        '--source[Set source directory]:path:_files -/' \
                        '-s[Set source directory]:path:_files -/' \
                        '--remote[Set git remote]:url:' \
                        '--copy-from[Copy skills from existing CLI directory]:name:' \
                        '-c[Copy skills from existing CLI directory]:name:' \
                        '--no-copy[Start with empty source]' \
                        '--targets[Comma-separated target names]:targets:' \
                        '-t[Comma-separated target names]:targets:' \
                        '--all-targets[Add all detected targets]' \
                        '--no-targets[Skip target setup]' \
                        '--mode[Set sync mode]:mode:(merge copy symlink)' \
                        '-m[Set sync mode]:mode:(merge copy symlink)' \
                        '--git[Initialize git]' \
                        '--no-git[Skip git initialization]' \
                        '--skill[Install built-in skillshare skill]' \
                        '--no-skill[Skip built-in skill installation]' \
                        '--discover[Detect and add new AI CLI agents]' \
                        '-d[Detect and add new AI CLI agents]' \
                        '--select[Select specific agents]:agents:' \
                        '--subdir[Use subdirectory as source]:name:' \
                        '--dry-run[Preview without changes]' \
                        '-n[Preview without changes]' \
                        $global_flags \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
                install)
                    _arguments \
                        '1:source:_files' \
                        '--source[Set source directory]:path:_files -/' \
                        '-s[Set source directory]:path:_files -/' \
                        '--name[Custom skill name]:name:' \
                        '--force[Overwrite existing]' \
                        '-f[Overwrite existing]' \
                        '--update[Update if exists]' \
                        '-u[Update if exists]' \
                        '--dry-run[Preview changes]' \
                        '-n[Preview changes]' \
                        '--skip-audit[Skip security audit]' \
                        '--audit-verbose[Verbose audit output]' \
                        '--audit-threshold[Set audit threshold]:level:(low medium high critical)' \
                        '--threshold[Set audit threshold]:level:(low medium high critical)' \
                        '-T[Set audit threshold]:level:(low medium high critical)' \
                        '--branch[Checkout specific branch]:branch:' \
                        '-b[Checkout specific branch]:branch:' \
                        '--track[Track the repository]' \
                        '-t[Track the repository]' \
                        '--kind[Filter by kind]:kind:(skill agent)' \
                        '--agent[Install specific agents]:agents:' \
                        '-a[Install specific agents]:agents:' \
                        '--skill[Install specific skills]:skills:' \
                        '--exclude[Exclude items]:names:' \
                        '--into[Custom destination path]:path:' \
                        '--all[Install all items]' \
                        '--yes[Skip confirmation]' \
                        '-y[Skip confirmation]' \
                        '--json[JSON output]' \
                        $global_flags \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
                uninstall)
                    _arguments \
                        '--all[Remove all skills]' \
                        '--force[Skip confirmation]' \
                        '-f[Skip confirmation]' \
                        '--dry-run[Preview changes]' \
                        '-n[Preview changes]' \
                        '--json[JSON output]' \
                        '--group[Uninstall by group]:group:' \
                        '-G[Uninstall by group]:group:' \
                        $global_flags \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
                list)
                    _arguments \
                        '1:type:(agents)' \
                        '--verbose[Show detailed information]' \
                        '-v[Show detailed information]' \
                        '--json[JSON output]' \
                        '-j[JSON output]' \
                        '--no-tui[Skip interactive TUI]' \
                        '--type[Filter by type]:type:(tracked local github)' \
                        '-t[Filter by type]:type:(tracked local github)' \
                        '--sort[Sort by]:order:(name newest oldest)' \
                        '-s[Sort by]:order:(name newest oldest)' \
                        '--all[List skills + agents]' \
                        $global_flags \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
                sync)
                    _arguments \
                        '1:scope:(agents extras)' \
                        '--all[Sync skills + agents + extras]' \
                        '--dry-run[Preview changes]' \
                        '-n[Preview changes]' \
                        '--force[Force sync]' \
                        '-f[Force sync]' \
                        '--json[JSON output]' \
                        $global_flags \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
                diff)
                    _arguments \
                        '--no-tui[Skip interactive TUI]' \
                        '--patch[Show unified diff patch]' \
                        '--stat[Show statistics]' \
                        '--json[JSON output]' \
                        $global_flags \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
                backup)
                    local -a backup_subcmds
                    backup_subcmds=(
                        'restore:Restore from backup'
                    )
                    _arguments -C \
                        '1:subcommand:->subcmd' \
                        '--list[List existing backups]' \
                        '-l[List existing backups]' \
                        '--cleanup[Remove old backups]' \
                        '-c[Remove old backups]' \
                        '--dry-run[Preview changes]' \
                        '-n[Preview changes]' \
                        '--target[Backup specific target]:target:' \
                        '-t[Backup specific target]:target:' \
                        $global_flags \
                        '--help[Show help]' \
                        '-h[Show help]'
                    case $state in
                        subcmd)
                            _describe 'subcommand' backup_subcmds
                            ;;
                    esac
                    ;;
                collect)
                    _arguments \
                        '1:scope:(agents)' \
                        '--all[Collect from all targets]' \
                        '-a[Collect from all targets]' \
                        '--dry-run[Preview changes]' \
                        '-n[Preview changes]' \
                        '--force[Overwrite existing]' \
                        '-f[Overwrite existing]' \
                        '--json[JSON output]' \
                        $global_flags \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
                adopt)
                    _arguments \
                        '--all[Adopt all detected skills]' \
                        '-a[Adopt all detected skills]' \
                        '--dry-run[Preview without changes]' \
                        '-n[Preview without changes]' \
                        '--force[Overwrite same-name source skills]' \
                        '-f[Overwrite same-name source skills]' \
                        '--json[JSON output without prompting]' \
                        $global_flags \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
                pull)
                    _arguments \
                        '--dry-run[Preview changes]' \
                        '-n[Preview changes]' \
                        '--force[Force pull]' \
                        '-f[Force pull]' \
                        $global_flags
                    ;;
                push)
                    _arguments \
                        '--dry-run[Preview changes]' \
                        '-n[Preview changes]' \
                        '--message[Commit message]:message:' \
                        '-m[Commit message]:message:' \
                        $global_flags
                    ;;
                commit)
                    _arguments \
                        '--dry-run[Preview changes]' \
                        '-n[Preview changes]' \
                        '--message[Commit message]:message:' \
                        '-m[Commit message]:message:' \
                        '--help[Show help]' \
                        '-h[Show help]' \
                        $global_flags
                    ;;
                doctor)
                    _arguments \
                        '--json[JSON output]' \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
                target)
                    local -a target_subcmds
                    target_subcmds=(
                        'add:Add a target'
                        'remove:Unlink target and restore skills'
                        'list:List all targets'
                    )
                    _arguments -C \
                        '1:subcommand:->subcmd' \
                        '--json[JSON output]' \
                        '--no-tui[Skip interactive TUI]' \
                        '--mode[Set sync mode]:mode:(merge copy symlink)' \
                        '--agent-mode[Set agents sync mode]:mode:(merge copy symlink)' \
                        '--target-naming[Set naming]:naming:(flat standard)' \
                        '--add-include[Add include filter]:pattern:' \
                        '--add-exclude[Add exclude filter]:pattern:' \
                        '--remove-include[Remove include filter]:pattern:' \
                        '--remove-exclude[Remove exclude filter]:pattern:' \
                        '--add-agent-include[Add agent include filter]:pattern:' \
                        '--add-agent-exclude[Add agent exclude filter]:pattern:' \
                        '--remove-agent-include[Remove agent include filter]:pattern:' \
                        '--remove-agent-exclude[Remove agent exclude filter]:pattern:' \
                        $global_flags \
                        '--help[Show help]' \
                        '-h[Show help]'
                    case $state in
                        subcmd)
                            _describe 'subcommand' target_subcmds
                            ;;
                    esac
                    ;;
                upgrade)
                    _arguments \
                        '--dry-run[Preview changes]' \
                        '-n[Preview changes]' \
                        '--force[Force upgrade]' \
                        '-f[Force upgrade]' \
                        '--skill[Install built-in skill]' \
                        '--cli[Upgrade CLI]' \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
                update)
                    _arguments \
                        '--all[Update all]' \
                        '-a[Update all]' \
                        '--dry-run[Preview changes]' \
                        '-n[Preview changes]' \
                        '--force[Force update]' \
                        '-f[Force update]' \
                        '--skip-audit[Skip security audit]' \
                        '--audit-threshold[Set audit threshold]:level:(low medium high critical)' \
                        '--threshold[Set audit threshold]:level:(low medium high critical)' \
                        '-T[Set audit threshold]:level:(low medium high critical)' \
                        '--diff[Show changes]' \
                        '--audit-verbose[Verbose audit output]' \
                        '--prune[Prune items]' \
                        '--json[JSON output]' \
                        '--group[Update by group]:group:' \
                        '-G[Update by group]:group:' \
                        $global_flags \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
                check)
                    _arguments \
                        '1:scope:(agents)' \
                        '--json[JSON output]' \
                        '--all[Check all]' \
                        '--group[Check by group]:group:' \
                        '-G[Check by group]:group:' \
                        $global_flags \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
                trash)
                    local -a trash_subcmds
                    trash_subcmds=(
                        'list:List trashed items'
                        'restore:Restore from trash'
                        'delete:Delete permanently'
                        'empty:Clear all trash'
                    )
                    _arguments -C \
                        '1:subcommand:->subcmd' \
                        '--no-tui[Skip interactive TUI]' \
                        $global_flags \
                        '--help[Show help]' \
                        '-h[Show help]'
                    case $state in
                        subcmd)
                            _describe 'subcommand' trash_subcmds
                            ;;
                    esac
                    ;;
                audit)
                    local -a audit_subcmds
                    audit_subcmds=(
                        'rules:Manage security rules'
                    )
                    _arguments -C \
                        '1:subcommand:->subcmd' \
                        '--init-rules[Initialize audit rules]' \
                        '--json[JSON output]' \
                        '--format[Output format]:format:(text json sarif markdown)' \
                        '--quiet[Suppress output]' \
                        '-q[Suppress output]' \
                        '--yes[Skip confirmation]' \
                        '-y[Skip confirmation]' \
                        '--no-tui[Skip interactive TUI]' \
                        '--threshold[Block threshold]:level:(low medium high critical)' \
                        '-T[Block threshold]:level:(low medium high critical)' \
                        '--group[Filter by group]:group:' \
                        '-G[Filter by group]:group:' \
                        '--profile[Security profile]:profile:(default strict permissive)' \
                        '--dedupe[Deduplication mode]:mode:(legacy global)' \
                        '--analyzer[Enable analyzer]:analyzer:(static dataflow tier integrity metadata structure cross-skill)' \
                        $global_flags \
                        '--help[Show help]' \
                        '-h[Show help]'
                    case $state in
                        subcmd)
                            _describe 'subcommand' audit_subcmds
                            ;;
                    esac
                    ;;
                hub)
                    local -a hub_subcmds
                    hub_subcmds=(
                        'add:Add hub'
                        'list:List hubs'
                        'remove:Remove hub'
                        'default:Set default hub'
                        'index:Create skill index'
                        'help:Show help'
                    )
                    _arguments -C \
                        '1:subcommand:->subcmd' \
                        $global_flags \
                        '--help[Show help]' \
                        '-h[Show help]'
                    case $state in
                        subcmd)
                            _describe 'subcommand' hub_subcmds
                            ;;
                    esac
                    ;;
                log)
                    _arguments \
                        '--audit[Show audit logs]' \
                        '-a[Show audit logs]' \
                        '--clear[Clear logs]' \
                        '-c[Clear logs]' \
                        '--json[JSON output]' \
                        '--no-tui[Skip interactive TUI]' \
                        '--stats[Show statistics]' \
                        '--cmd[Filter by command]:command:' \
                        '--status[Filter by status]:status:(ok error)' \
                        '--since[Filter by date]:date:' \
                        '--tail[Show last N entries]:count:' \
                        '-t[Show last N entries]:count:' \
                        $global_flags \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
                ui)
                    _arguments \
                        '--port[Set port]:port:' \
                        '--host[Set host]:host:' \
                        '--no-open[Do not open browser]' \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
                tui)
                    _arguments \
                        '1:state:(on off)'
                    ;;
                extras)
                    local -a extras_subcmds
                    extras_subcmds=(
                        'init:Create extra resource type'
                        'list:List extras with sync status'
                        'remove:Remove extra resource type'
                        'collect:Collect local files into extras'
                        'source:Show/set extras source'
                        'mode:Change sync mode or flatten'
                    )
                    _arguments -C \
                        '1:subcommand:->subcmd' \
                        $global_flags \
                        '--help[Show help]' \
                        '-h[Show help]'
                    case $state in
                        subcmd)
                            _describe 'subcommand' extras_subcmds
                            ;;
                    esac
                    ;;
                enable)
                    _arguments \
                        '--dry-run[Preview changes]' \
                        '-n[Preview changes]' \
                        $global_flags \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
                disable)
                    _arguments \
                        '--dry-run[Preview changes]' \
                        '-n[Preview changes]' \
                        $global_flags \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
                analyze)
                    _arguments \
                        '--no-tui[Skip interactive TUI]' \
                        '--json[JSON output]' \
                        $global_flags \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
                completion)
                    _arguments \
                        '1:shell:(bash zsh fish powershell nushell)' \
                        '--install[Install completion script]' \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
            esac
            ;;
    esac
}

_skillshare "$@"

# Auto-detect aliases and register completion for them
() {
    local _ss_name
    for _ss_name in ${(k)aliases}; do
        [[ "${aliases[$_ss_name]}" == "skillshare" ]] && compdef _skillshare $_ss_name
    done
}
`
