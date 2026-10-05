package main

const bashCompletionScript = `#!/bin/bash
# skillshare bash completion

_skillshare() {
    local cur prev words cword
    if declare -F _init_completion >/dev/null 2>&1; then
        _init_completion || return
    else
        cur="${COMP_WORDS[COMP_CWORD]}"
        prev="${COMP_WORDS[COMP_CWORD-1]}"
        words=("${COMP_WORDS[@]}")
        cword=$COMP_CWORD
    fi

    local commands="init install uninstall list search sync mcp hooks plugin status diff backup restore collect pull push commit doctor target upgrade update check new trash analyze audit hub log ui tui extras enable disable completion version help"

    local global_flags="--project -p --global -g"

    # Subcommands
    local target_subcmds="add remove list"
    local trash_subcmds="agents list restore delete empty"
    local hub_subcmds="add list remove default index help"
    local extras_subcmds="init list remove collect source memory"
    local backup_subcmds="files agents"
    local backup_files_subcmds="list show restore"
    local audit_subcmds="rules agents"
    local audit_rules_subcmds="disable enable severity reset init"
    local ui_subcmds="start stop"
    local completion_subcmds="bash zsh fish powershell nushell"

    # Per-command flags
    local init_flags="--source -s --remote --copy-from -c --no-copy --targets -t --all-targets --no-targets --mode -m --git --no-git --git-root --skill --no-skill --discover -d --select --subdir --visible --config --dry-run -n --help -h"
    local install_flags="--name --force -f --update -u --dry-run -n --skip-audit --audit-verbose --audit-threshold --threshold -T --branch -b --track -t --kind --agent -a --skill -s --exclude --into --all --yes -y --json --help -h"
    local uninstall_flags="--all --force -f --dry-run -n --json --group -G --help -h"
    local list_flags="--verbose -v --json -j --no-tui --type -t --status --sort -s --all --help -h"
    local sync_flags="--all --dry-run -n --force -f --json --quiet -q --help -h"
    local mcp_flags="--tools-allow --tools-deny --pi-options --url --target --from --file --sync --replace --disabled --keep-files --revision --dry-run -n --json --no-tui --no-dns --live --timeout --help -h"
    local status_flags="--json --help -h"
    local hooks_flags="--file --from --sync --replace --keep-files --revision --dry-run -n --json --help -h"
    local diff_flags="--no-tui --patch --stat --json --help -h"
    local backup_flags="--list -l --cleanup -c --delete --all --dry-run -n --target -t --help -h"
    local backup_files_restore_flags="--unlink --dry-run -n --help -h"
    local restore_flags="--from -f --force --all --dry-run -n --no-tui --help -h"
    local collect_flags="--all -a --dry-run -n --force -f --json --help -h"
    local pull_flags="--dry-run -n --force -f --help -h"
    local push_flags="--dry-run -n --pull --message -m --help -h"
    local commit_flags="--dry-run -n --message -m --help -h"
    local doctor_flags="--json --help -h"
    local target_flags="--json --no-tui --help -h --mode -m --agent-mode --target-naming --add-include --add-exclude --remove-include --remove-exclude --add-agent-include --add-agent-exclude --remove-agent-include --remove-agent-exclude --agent --config-dir --cli --skills --no-skills --dry-run"
    local target_remove_flags="--all -a --dry-run -n"
    local upgrade_flags="--dry-run -n --force -f --skill --cli --help -h"
    local update_flags="--all -a --dry-run -n --force -f --skip-audit --audit-threshold --threshold -T --diff --audit-verbose --prune --json --group -G --help -h"
    local check_flags="--json --all --group -G --help -h"
    local new_flags="--pattern -P --dry-run -n --help -h"
    local search_flags="--json --list -l --hub --limit -n --help -h"
    local trash_flags="--all --no-tui --help -h"
    local audit_flags="--init-rules --json --format --quiet -q --yes -y --no-tui --threshold -T --group -G --profile --dedupe --analyzer --help -h"
    local audit_rules_flags="--pattern --severity --disabled --format --no-tui --help -h"
    local hub_flags="--help -h"
    local hub_index_flags="--source -s --output -o --full --audit --help -h"
    local hub_add_flags="--label -l --help -h"
    local hub_default_flags="--reset --help -h"
    local log_flags="--audit -a --clear -c --json --no-tui --stats --cmd --status --since --tail -t --help -h"
    local ui_flags="--port --host --base-path -b --no-open --clear-cache --app --help -h"
    local tui_flags="--help -h"
    local enable_flags="--dry-run -n --kind --help -h"
    local disable_flags="--dry-run -n --kind --help -h"
    local analyze_flags="--verbose -v --filter --no-tui --json --help -h"
    local extras_flags="--mode --target --flatten --no-flatten --add-target --as --remove-target --prune --help -h"
    local extras_init_flags="--target --mode --source --file --as --flatten --force --no-tui --help -h"
    local extras_list_flags="--json --no-tui --help -h"
    local extras_remove_flags="--force -f --help -h"
    local extras_collect_flags="--from --dry-run --force -f --help -h"
    local completion_flags="--install --help -h"

    case "${cword}" in
        1)
            COMPREPLY=($(compgen -W "${commands}" -- "${cur}"))
            return
            ;;
    esac

    local cmd="${words[1]}"

    if [[ "${cmd}" == mcp && ( "${prev}" == --target || "${prev}" == --from ) ]]; then
        COMPREPLY=($(compgen -W "claude codex cursor vscode opencode kilocode grok antigravity amp claude-desktop cline copilot factory gemini goose junie kiro lmstudio warp windsurf pi omp" -- "${cur}"))
        return
    fi
    if [[ "${cmd}" == target && "${prev}" == --agent ]]; then
        COMPREPLY=($(compgen -W "claude codex pi omp" -- "${cur}"))
        return
    fi

    if [[ "${cmd}" == plugin && ( "${prev}" == --target || "${prev}" == --from ) ]]; then
        COMPREPLY=($(compgen -W "claude codex cursor antigravity agy antigravity-cli copilot grok kimi hermes devin pi opencode omp" -- "${cur}"))
        return
    fi

    # Handle subcommands (cword == 2)
    if [[ ${cword} -eq 2 ]]; then
        case "${cmd}" in
            target)
                COMPREPLY=($(compgen -W "${target_subcmds} ${target_flags} ${global_flags}" -- "${cur}"))
                return
                ;;
            trash)
                COMPREPLY=($(compgen -W "${trash_subcmds} ${trash_flags} ${global_flags}" -- "${cur}"))
                return
                ;;
            hub)
                COMPREPLY=($(compgen -W "${hub_subcmds} ${hub_flags} ${global_flags}" -- "${cur}"))
                return
                ;;
            extras)
                COMPREPLY=($(compgen -W "${extras_subcmds} --help -h ${global_flags}" -- "${cur}"))
                return
                ;;
            audit)
                COMPREPLY=($(compgen -W "${audit_subcmds} ${audit_flags} ${global_flags}" -- "${cur}"))
                return
                ;;
            backup)
                COMPREPLY=($(compgen -W "${backup_subcmds} ${backup_flags} ${global_flags}" -- "${cur}"))
                return
                ;;
            completion)
                COMPREPLY=($(compgen -W "${completion_subcmds} ${completion_flags}" -- "${cur}"))
                return
                ;;
            sync)
                COMPREPLY=($(compgen -W "agents extras mcp hooks plugins ${sync_flags} ${global_flags}" -- "${cur}"))
                return
                ;;
            list)
                COMPREPLY=($(compgen -W "agents ${list_flags} ${global_flags}" -- "${cur}"))
                return
                ;;
            ui)
                COMPREPLY=($(compgen -W "${ui_subcmds} ${ui_flags} ${global_flags}" -- "${cur}"))
                return
                ;;
            uninstall) COMPREPLY=($(compgen -W "agents ${uninstall_flags} ${global_flags}" -- "${cur}")); return ;;
            diff)      COMPREPLY=($(compgen -W "agents ${diff_flags} ${global_flags}" -- "${cur}")); return ;;
            restore)   COMPREPLY=($(compgen -W "agents ${restore_flags} ${global_flags}" -- "${cur}")); return ;;
            collect)   COMPREPLY=($(compgen -W "agents ${collect_flags} ${global_flags}" -- "${cur}")); return ;;
            update)    COMPREPLY=($(compgen -W "agents ${update_flags} ${global_flags}" -- "${cur}")); return ;;
            check)     COMPREPLY=($(compgen -W "agents ${check_flags} ${global_flags}" -- "${cur}")); return ;;
        esac
    fi

    # Handle flags for subcommands (cword >= 3)
    if [[ ${cword} -ge 3 ]]; then
        local subcmd="${words[2]}"
        case "${cmd}" in
            target)
                case "${subcmd}" in
                    remove)
                        COMPREPLY=($(compgen -W "${target_remove_flags} ${global_flags}" -- "${cur}"))
                        return
                        ;;
                    add|list)
                        COMPREPLY=($(compgen -W "${target_flags} ${global_flags}" -- "${cur}"))
                        return
                        ;;
                esac
                ;;
            hub)
                case "${subcmd}" in
                    index)
                        COMPREPLY=($(compgen -W "${hub_index_flags} ${global_flags}" -- "${cur}"))
                        return
                        ;;
                    add)
                        COMPREPLY=($(compgen -W "${hub_add_flags} ${global_flags}" -- "${cur}"))
                        return
                        ;;
                    default)
                        COMPREPLY=($(compgen -W "${hub_default_flags} ${global_flags}" -- "${cur}"))
                        return
                        ;;
                esac
                ;;
            backup)
                case "${subcmd}" in
                    files)
                        if [[ ${cword} -eq 3 ]]; then
                            COMPREPLY=($(compgen -W "${backup_files_subcmds} ${global_flags}" -- "${cur}"))
                        elif [[ "${words[3]}" == restore ]]; then
                            COMPREPLY=($(compgen -W "${backup_files_restore_flags} ${global_flags}" -- "${cur}"))
                        fi
                        return
                        ;;
                esac
                ;;
            audit)
                case "${subcmd}" in
                    rules)
                        if [[ ${cword} -eq 3 ]]; then
                            COMPREPLY=($(compgen -W "${audit_rules_subcmds} ${audit_rules_flags} ${global_flags}" -- "${cur}"))
                        else
                            COMPREPLY=($(compgen -W "${audit_rules_flags} ${global_flags}" -- "${cur}"))
                        fi
                        return
                        ;;
                esac
                ;;
            trash)
                case "${subcmd}" in
                    agents)
                        if [[ ${cword} -eq 3 ]]; then
                            COMPREPLY=($(compgen -W "list restore delete empty ${trash_flags} ${global_flags}" -- "${cur}"))
                            return
                        fi
                        ;;
                esac
                ;;
            extras)
                case "${subcmd}" in
                    memory)
                        if [[ ${cword} -eq 3 ]]; then
                            COMPREPLY=($(compgen -W "init list show write delete instructions --help -h ${global_flags}" -- "${cur}"))
                        else
                            local memory_flags="--json --help -h"
                            case "${words[3]}" in
                                list) memory_flags="${memory_flags} --search" ;;
                                delete) memory_flags="${memory_flags} --version" ;;
                                write) memory_flags="${memory_flags} --from --version" ;;
                                instructions) memory_flags="${memory_flags} --update-mode" ;;
                            esac
                            COMPREPLY=($(compgen -W "${memory_flags} ${global_flags}" -- "${cur}"))
                        fi
                        ;;
                    init)    COMPREPLY=($(compgen -W "${extras_init_flags} ${global_flags}" -- "${cur}")) ;;
                    list)    COMPREPLY=($(compgen -W "${extras_list_flags} ${global_flags}" -- "${cur}")) ;;
                    remove)  COMPREPLY=($(compgen -W "${extras_remove_flags} ${global_flags}" -- "${cur}")) ;;
                    collect) COMPREPLY=($(compgen -W "${extras_collect_flags} ${global_flags}" -- "${cur}")) ;;
                    source)  COMPREPLY=($(compgen -W "--help -h ${global_flags}" -- "${cur}")) ;;
                    *)       COMPREPLY=($(compgen -W "${extras_flags} ${global_flags}" -- "${cur}")) ;;
                esac
                return
                ;;
        esac
    fi

    # Default: per-command flags + global flags
    case "${cmd}" in
        init)       COMPREPLY=($(compgen -W "${init_flags} ${global_flags}" -- "${cur}")) ;;
        install)    COMPREPLY=($(compgen -W "${install_flags} ${global_flags}" -- "${cur}")) ;;
        uninstall)  COMPREPLY=($(compgen -W "${uninstall_flags} ${global_flags}" -- "${cur}")) ;;
        list)       COMPREPLY=($(compgen -W "${list_flags} ${global_flags}" -- "${cur}")) ;;
        sync)       COMPREPLY=($(compgen -W "${sync_flags} ${global_flags}" -- "${cur}")) ;;
        plugin)     COMPREPLY=($(compgen -W "add discover import list inspect sync check update enable disable remove --target --from --plugin --name --source-ref --entry --revision --dry-run -n --json --no-tui --help -h ${global_flags}" -- "${cur}")) ;;
        mcp)        COMPREPLY=($(compgen -W "add check edit import list remove restore ${mcp_flags} ${global_flags}" -- "${cur}")) ;;
        status)     COMPREPLY=($(compgen -W "${status_flags} ${global_flags}" -- "${cur}")) ;;
        hooks)      COMPREPLY=($(compgen -W "add disable edit enable import list remove restore sync ${hooks_flags} ${global_flags}" -- "${cur}")) ;;
        diff)       COMPREPLY=($(compgen -W "${diff_flags} ${global_flags}" -- "${cur}")) ;;
        backup)     COMPREPLY=($(compgen -W "${backup_flags} ${global_flags}" -- "${cur}")) ;;
        restore)    COMPREPLY=($(compgen -W "${restore_flags} ${global_flags}" -- "${cur}")) ;;
        collect)    COMPREPLY=($(compgen -W "${collect_flags} ${global_flags}" -- "${cur}")) ;;
        pull)       COMPREPLY=($(compgen -W "${pull_flags}" -- "${cur}")) ;;
        push)       COMPREPLY=($(compgen -W "${push_flags}" -- "${cur}")) ;;
        commit)     COMPREPLY=($(compgen -W "${commit_flags}" -- "${cur}")) ;;
        doctor)     COMPREPLY=($(compgen -W "${doctor_flags} ${global_flags}" -- "${cur}")) ;;
        target)     COMPREPLY=($(compgen -W "${target_flags} ${global_flags}" -- "${cur}")) ;;
        upgrade)    COMPREPLY=($(compgen -W "${upgrade_flags} ${global_flags}" -- "${cur}")) ;;
        update)     COMPREPLY=($(compgen -W "${update_flags} ${global_flags}" -- "${cur}")) ;;
        check)      COMPREPLY=($(compgen -W "${check_flags} ${global_flags}" -- "${cur}")) ;;
        trash)      COMPREPLY=($(compgen -W "${trash_flags} ${global_flags}" -- "${cur}")) ;;
        audit)      COMPREPLY=($(compgen -W "${audit_flags} ${global_flags}" -- "${cur}")) ;;
        hub)        COMPREPLY=($(compgen -W "${hub_flags} ${global_flags}" -- "${cur}")) ;;
        log)        COMPREPLY=($(compgen -W "${log_flags} ${global_flags}" -- "${cur}")) ;;
        ui)         COMPREPLY=($(compgen -W "${ui_flags} ${global_flags}" -- "${cur}")) ;;
        tui)        COMPREPLY=($(compgen -W "on off ${tui_flags}" -- "${cur}")) ;;
        enable)     COMPREPLY=($(compgen -W "${enable_flags} ${global_flags}" -- "${cur}")) ;;
        disable)    COMPREPLY=($(compgen -W "${disable_flags} ${global_flags}" -- "${cur}")) ;;
        analyze)    COMPREPLY=($(compgen -W "${analyze_flags} ${global_flags}" -- "${cur}")) ;;
        extras)     COMPREPLY=($(compgen -W "${extras_flags} ${global_flags}" -- "${cur}")) ;;
        completion) COMPREPLY=($(compgen -W "${completion_flags}" -- "${cur}")) ;;
        new)        COMPREPLY=($(compgen -W "${new_flags} ${global_flags}" -- "${cur}")) ;;
        search)     COMPREPLY=($(compgen -W "${search_flags} ${global_flags}" -- "${cur}")) ;;
    esac
}

complete -F _skillshare skillshare

# Auto-detect aliases pointing to skillshare and register completion for them
if command -v alias >/dev/null 2>&1; then
    while IFS= read -r _ss_alias; do
        complete -F _skillshare "$_ss_alias"
    done < <(alias 2>/dev/null | sed -n "s/^alias \([^=]*\)=['\"].*skillshare['\"]$/\1/p")
    unset _ss_alias
fi
`
