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
        'sync:Sync skills/agents/extras/MCP to targets'
        'plugin:Manage complete native plugins'
        'mcp:Manage MCP connections'
        'hooks:Manage Agent hooks'
        'status:Show status of all targets'
        'diff:Show differences between source and targets'
        'backup:Create backup of targets'
        'restore:Restore target from backup'
        'collect:Collect local skills/agents from targets'
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
                        '--git-root[Git repository scope]:scope:(skills agents extras root)' \
                        '--visible[Project: create visible skillshare/ directory]' \
                        '--config[Project: gitignore config.yaml]:value:(local)' \
                        '--dry-run[Preview without changes]' \
                        '-n[Preview without changes]' \
                        $global_flags \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
                mcp)
                    _arguments \
                        '1:command:(add check edit import list remove restore)' \
                        '--tools-allow[Only these tools, comma-separated, * matches any characters; empty clears]:tools:' \
                        '--tools-deny[Never these tools, comma-separated, * matches any characters; empty clears]:tools:' \
                        '--pi-options[Other Pi built-in per-server fields as JSON]:json:' \
                        '--target[Receiving client]:target:(claude codex cursor vscode opencode kilocode grok antigravity amp claude-desktop cline copilot factory gemini goose junie kiro lmstudio warp windsurf pi omp)' \
                        '--from[Import client]:target:(claude codex cursor vscode opencode kilocode grok antigravity amp claude-desktop cline copilot factory gemini goose junie kiro lmstudio warp windsurf pi omp)' \
                        '--url[MCP endpoint]:url:' \
                        '--file[Import file]:file:_files' \
                        '--sync[Sync after saving]' \
                        '--replace[Replace an existing entry]' \
                        '--disabled[add: turn off the server in the project]' \
                        '--keep-files[remove: leave Agent entries as they are]' \
                        '--revision[Preview revision]:revision:' \
                        '--dry-run[Preview changes]' \
                        '-n[Preview changes]' \
                        '--json[JSON output]' \
                        '--no-dns[Skip host lookups]' \
                        '--live[check: start or call each server]' \
                        '--timeout[check --live: per-server timeout]:duration:' \
                        '--no-tui[Disable interactive menus]' \
                        $global_flags \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
                hooks)
                    _arguments \
                        '1:command:(add disable edit enable import list remove restore sync)' \
                        '--file[Entry or native file]:file:_files' \
                        '--from[Import Agent]:agent:' \
                        '--sync[Sync after saving]' \
                        '--replace[Replace an existing entry]' \
                        '--keep-files[remove: leave Agent entries as they are]' \
                        '--revision[Preview revision]:revision:' \
                        '--dry-run[Preview changes]' \
                        '-n[Preview changes]' \
                        '--json[JSON output]' \
                        $global_flags \
                        '--help[Show help]'
                    ;;
                plugin)
                    _arguments \
                        '1:command:(add discover import list inspect sync check update enable disable remove)' \
                        '--target[Receiving target]:target:(claude codex cursor antigravity agy antigravity-cli copilot grok kimi hermes devin pi opencode omp)' \
                        '--from[Import target]:target:(claude codex cursor antigravity agy antigravity-cli copilot grok kimi hermes devin pi opencode omp)' \
                        '--plugin[Source plugin]:name:' \
                        '--name[Logical package name]:name:' \
                        '--source-ref[Git branch, tag or commit]:ref:' \
                        '--entry[OpenCode entry path]:path:' \
                        '--revision[Preview revision]:revision:' \
                        '--dry-run[Preview changes]' \
                        '-n[Preview changes]' \
                        '--json[JSON output]' \
                        '--no-tui[Disable interactive menus]' \
                        $global_flags \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
                install)
                    _arguments \
                        '1:source:_files' \
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
                        '-s[Install specific skills]:skills:' \
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
                        '1:scope:(agents)' \
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
                        '--status[Filter by status]:status:(all enabled disabled)' \
                        '--sort[Sort by]:order:(name newest oldest)' \
                        '-s[Sort by]:order:(name newest oldest)' \
                        '--all[List skills + agents]' \
                        $global_flags \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
                sync)
                    _arguments \
                        '1:scope:(agents extras mcp hooks plugins)' \
                        '--all[Sync skills + agents + extras]' \
                        '--dry-run[Preview changes]' \
                        '-n[Preview changes]' \
                        '--force[Force sync]' \
                        '-f[Force sync]' \
                        '--json[JSON output]' \
                        '--quiet[Suppress token summary]' \
                        '-q[Suppress token summary]' \
                        $global_flags \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
                status)
                    _arguments \
                        '--json[JSON output]' \
                        $global_flags \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
                diff)
                    _arguments \
                        '1:scope:(agents)' \
                        '--no-tui[Skip interactive TUI]' \
                        '--patch[Show unified diff patch]' \
                        '--stat[Show statistics]' \
                        '--json[JSON output]' \
                        $global_flags \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
                backup)
                    if (( CURRENT > 2 )) && [[ ${words[2]} == files ]]; then
                        shift words; (( CURRENT-- ))
                        _arguments \
                            '1:files command:(list show restore)' \
                            '--unlink[restore: replace a symlink with a regular file]' \
                            '--dry-run[Preview changes]' \
                            '-n[Preview changes]' \
                            $global_flags \
                            '--help[Show help]' \
                            '-h[Show help]'
                        return
                    fi
                    local -a backup_subcmds
                    backup_subcmds=(
                        'files:Versions of single files skillshare rewrote'
                        'agents:Back up agents'
                    )
                    _arguments -C \
                        '1:subcommand:->subcmd' \
                        '--list[List existing backups]' \
                        '-l[List existing backups]' \
                        '--cleanup[Remove old backups]' \
                        '-c[Remove old backups]' \
                        '--delete[Delete one backup]:timestamp:' \
                        '--all[Back up skills + agents]' \
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
                restore)
                    _arguments \
                        '1:scope:(agents)' \
                        '--from[Restore from timestamp]:timestamp:' \
                        '-f[Restore from timestamp]:timestamp:' \
                        '--force[Overwrite without confirmation]' \
                        '--all[Restore skills + agents]' \
                        '--dry-run[Preview changes]' \
                        '-n[Preview changes]' \
                        '--no-tui[Skip interactive TUI]' \
                        $global_flags \
                        '--help[Show help]' \
                        '-h[Show help]'
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
                pull)
                    _arguments \
                        '--dry-run[Preview changes]' \
                        '-n[Preview changes]' \
                        '--force[Force pull]' \
                        '-f[Force pull]' \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
                push)
                    _arguments \
                        '--dry-run[Preview changes]' \
                        '-n[Preview changes]' \
                        '--pull[Merge remote changes before pushing, then sync]' \
                        '--message[Commit message]:message:' \
                        '-m[Commit message]:message:' \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
                commit)
                    _arguments \
                        '--dry-run[Preview changes]' \
                        '-n[Preview changes]' \
                        '--message[Commit message]:message:' \
                        '-m[Commit message]:message:' \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
                doctor)
                    _arguments \
                        '--json[JSON output]' \
                        $global_flags \
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
                        '-m[Set sync mode]:mode:(merge copy symlink)' \
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
                        '--agent[With add: the Agent this is another account of]:agent:(claude codex pi omp)' \
                        '--config-dir[With add: the config directory of that account]:dir:_files -/' \
                        '--cli[With add: the executable that runs its plugin commands]:executable:_command_names' \
                        '--skills=[Sync skills to this target]:enabled:(true false)' \
                        '--no-skills[With add: do not sync skills]' \
                        '--all[With remove: remove all targets]' \
                        '-a[With remove: remove all targets]' \
                        '--dry-run[Preview changes]' \
                        '-n[Preview changes]' \
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
                        '1:scope:(agents)' \
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
                new)
                    _arguments \
                        '--pattern[Use a design pattern]:pattern:(tool-wrapper generator reviewer inversion pipeline none)' \
                        '-P[Use a design pattern]:pattern:(tool-wrapper generator reviewer inversion pipeline none)' \
                        '--dry-run[Preview changes]' \
                        '-n[Preview changes]' \
                        $global_flags \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
                search)
                    _arguments \
                        '--json[JSON output]' \
                        '--list[List results only]' \
                        '-l[List results only]' \
                        '--hub[Search a hub index]:url:' \
                        '--limit[Maximum results]:count:' \
                        '-n[Maximum results]:count:' \
                        $global_flags \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
                trash)
                    local -a trash_subcmds
                    trash_subcmds=(
                        'agents:Trashed agents'
                        'list:List trashed items'
                        'restore:Restore from trash'
                        'delete:Delete permanently'
                        'empty:Clear all trash'
                    )
                    _arguments -C \
                        '1:subcommand:->subcmd' \
                        '--all[Include skills + agents]' \
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
                    if (( CURRENT > 2 )) && [[ ${words[2]} == rules ]]; then
                        shift words; (( CURRENT-- ))
                        _arguments \
                            '1:rules command:(disable enable severity reset init)' \
                            '--pattern[Filter by pattern name]:pattern:' \
                            '--severity[Filter by minimum severity]:level:(critical high medium low info)' \
                            '--disabled[Only show disabled rules]' \
                            '--format[Output format]:format:(json)' \
                            '--no-tui[Skip interactive TUI]' \
                            $global_flags \
                            '--help[Show help]' \
                            '-h[Show help]'
                        return
                    fi
                    local -a audit_subcmds
                    audit_subcmds=(
                        'rules:Manage security rules'
                        'agents:Audit agents'
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
                    if (( CURRENT > 2 )); then
                        local -a hub_sub_flags
                        case ${words[2]} in
                            index)
                                hub_sub_flags=(
                                    '--source[Source directory to scan]:path:_files -/'
                                    '-s[Source directory to scan]:path:_files -/'
                                    '--output[Output path]:path:_files'
                                    '-o[Output path]:path:_files'
                                    '--full[Full index]'
                                    '--audit[Include audit risk scores]'
                                )
                                ;;
                            add)
                                hub_sub_flags=(
                                    '--label[Label for the hub]:label:'
                                    '-l[Label for the hub]:label:'
                                )
                                ;;
                            default)
                                hub_sub_flags=(
                                    '--reset[Clear default hub]'
                                )
                                ;;
                        esac
                        shift words; (( CURRENT-- ))
                        _arguments $hub_sub_flags $global_flags '--help[Show help]' '-h[Show help]'
                        return
                    fi
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
                        '1:subcommand:(start stop)' \
                        '--port[Set port]:port:' \
                        '--host[Set host]:host:' \
                        '--base-path[Base path prefix for reverse proxy]:path:' \
                        '-b[Base path prefix for reverse proxy]:path:' \
                        '--no-open[Do not open browser]' \
                        '--clear-cache[Clear cached UI assets]' \
                        '--app[Open as app window (start only)]' \
                        $global_flags \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
                tui)
                    _arguments \
                        '1:state:(on off)' \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
                extras)
                    if (( CURRENT > 2 )); then
                        local -a extras_sub_flags
                        if [[ ${words[2]} == memory ]]; then
                            shift words; (( CURRENT-- ))
                            if (( CURRENT == 2 )); then
                                _arguments '1:command:(init list show write delete instructions)' $global_flags '--help[Show help]' '-h[Show help]'
                            else
                                case ${words[2]} in
                                    list) extras_sub_flags=('--search[Search names and content]:text:') ;;
                                    delete) extras_sub_flags=('--version[Last read hash]:hash:') ;;
                                    write) extras_sub_flags=('--from[Input file or stdin]:file:_files' '--version[Last read hash]:hash:') ;;
                                    instructions) extras_sub_flags=('--update-mode[How agents update notes]:mode:(passive active)') ;;
                                esac
                                shift words; (( CURRENT-- ))
                                _arguments '1:note: ' $extras_sub_flags '--json[JSON output]' $global_flags '--help[Show help]' '-h[Show help]'
                            fi
                            return
                        fi
                        case ${words[2]} in
                            init)
                                extras_sub_flags=(
                                    '--target[Target directory]:path:_files -/'
                                    '--mode[Sync mode]:mode:(merge copy symlink import)'
                                    '--source[Custom source directory]:path:_files -/'
                                    '--file[Single-file extra]:file:_files'
                                    '--as[Target filename]:filename:'
                                    '--flatten[Flatten subdirectory files]'
                                    '--force[Overwrite existing extra]'
                                    '--no-tui[Skip interactive TUI]'
                                )
                                ;;
                            list)
                                extras_sub_flags=(
                                    '--json[JSON output]'
                                    '--no-tui[Skip interactive TUI]'
                                )
                                ;;
                            remove)
                                extras_sub_flags=(
                                    '--force[Skip confirmation]'
                                    '-f[Skip confirmation]'
                                )
                                ;;
                            collect)
                                extras_sub_flags=(
                                    '--from[Target directory to collect from]:path:_files -/'
                                    '--dry-run[Preview changes]'
                                    '--force[Overwrite existing files]'
                                    '-f[Overwrite existing files]'
                                )
                                ;;
                            source)
                                ;;
                            *)
                                extras_sub_flags=(
                                    '--mode[Change sync mode]:mode:(merge copy symlink import)'
                                    '--target[Target for --mode]:path:_files -/'
                                    '--flatten[Enable flatten]'
                                    '--no-flatten[Disable flatten]'
                                    '--add-target[Add a target]:path:_files -/'
                                    '--as[Target filename]:filename:'
                                    '--remove-target[Detach a target]:path:_files -/'
                                    '--prune[Also delete managed files]'
                                )
                                ;;
                        esac
                        shift words; (( CURRENT-- ))
                        _arguments $extras_sub_flags $global_flags '--help[Show help]' '-h[Show help]'
                        return
                    fi
                    local -a extras_subcmds
                    extras_subcmds=(
                        'init:Create extra resource type'
                        'list:List extras with sync status'
                        'remove:Remove extra resource type'
                        'collect:Collect local files into extras'
                        'source:Show/set extras source'
                        'memory:Manage shared Markdown notes'
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
                        '--kind[Resource kind]:kind:(skill agent)' \
                        '--dry-run[Preview changes]' \
                        '-n[Preview changes]' \
                        $global_flags \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
                disable)
                    _arguments \
                        '--kind[Resource kind]:kind:(skill agent)' \
                        '--dry-run[Preview changes]' \
                        '-n[Preview changes]' \
                        $global_flags \
                        '--help[Show help]' \
                        '-h[Show help]'
                    ;;
                analyze)
                    _arguments \
                        '--verbose[Show detailed information]' \
                        '-v[Show detailed information]' \
                        '--filter[Filter skills by name or path]:text:' \
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
