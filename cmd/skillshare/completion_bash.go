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

    local commands="init install uninstall list search sync status diff backup restore collect pull push commit doctor target upgrade update check new trash analyze audit hub log ui tui extras enable disable completion version help"

    local global_flags="--project -p --global -g"

    # Subcommands
    local target_subcmds="add remove list"
    local trash_subcmds="list restore delete empty"
    local hub_subcmds="add list remove default index help"
    local extras_subcmds="init list remove collect source mode"
    local backup_subcmds="restore"
    local audit_subcmds="rules"
    local completion_subcmds="bash zsh fish powershell nushell"

    # Per-command flags
    local init_flags="--source -s --remote --copy-from -c --no-copy --targets -t --all-targets --no-targets --mode -m --git --no-git --skill --no-skill --discover -d --select --subdir --dry-run -n --help -h"
    local install_flags="--source -s --name --force -f --update -u --dry-run -n --skip-audit --audit-verbose --audit-threshold --threshold -T --branch -b --track -t --kind --agent -a --skill --exclude --into --all --yes -y --json --help -h"
    local uninstall_flags="--all --force -f --dry-run -n --json --group -G --help -h"
    local list_flags="--verbose -v --json -j --no-tui --type -t --sort -s --all --help -h"
    local sync_flags="--all --dry-run -n --force -f --json --help -h"
    local diff_flags="--no-tui --patch --stat --json --help -h"
    local backup_flags="--list -l --cleanup -c --dry-run -n --target -t --help -h"
    local restore_flags="--from -f --force --dry-run -n --no-tui --help -h"
    local collect_flags="--all -a --dry-run -n --force -f --json --help -h"
    local pull_flags="--dry-run -n --force -f"
    local push_flags="--dry-run -n --message -m"
    local commit_flags="--dry-run -n --message -m --help -h"
    local doctor_flags="--json --help -h"
    local target_flags="--json --no-tui --help -h --mode --agent-mode --target-naming --add-include --add-exclude --remove-include --remove-exclude --add-agent-include --add-agent-exclude --remove-agent-include --remove-agent-exclude"
    local target_remove_flags="--all -a --dry-run -n"
    local upgrade_flags="--dry-run -n --force -f --skill --cli --help -h"
    local update_flags="--all -a --dry-run -n --force -f --skip-audit --audit-threshold --threshold -T --diff --audit-verbose --prune --json --group -G --help -h"
    local check_flags="--json --all --group -G --help -h"
    local trash_flags="--no-tui --help -h"
    local audit_flags="--init-rules --json --format --quiet -q --yes -y --no-tui --threshold -T --group -G --profile --dedupe --analyzer --help -h"
    local hub_flags="--help -h"
    local hub_index_flags="--source -s --output -o --full --audit-skills"
    local log_flags="--audit -a --clear -c --json --no-tui --stats --cmd --status --since --tail -t --help -h"
    local ui_flags="--port --host --no-open --help -h"
    local enable_flags="--dry-run -n --help -h"
    local disable_flags="--dry-run -n --help -h"
    local analyze_flags="--no-tui --json --help -h"
    local extras_flags="--help -h"
    local completion_flags="--install --help -h"

    case "${cword}" in
        1)
            COMPREPLY=($(compgen -W "${commands}" -- "${cur}"))
            return
            ;;
    esac

    local cmd="${words[1]}"

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
                COMPREPLY=($(compgen -W "${extras_subcmds} ${extras_flags} ${global_flags}" -- "${cur}"))
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
                COMPREPLY=($(compgen -W "agents extras ${sync_flags} ${global_flags}" -- "${cur}"))
                return
                ;;
            list)
                COMPREPLY=($(compgen -W "agents ${list_flags} ${global_flags}" -- "${cur}"))
                return
                ;;
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
                esac
                ;;
            backup)
                case "${subcmd}" in
                    restore)
                        COMPREPLY=($(compgen -W "${restore_flags} ${global_flags}" -- "${cur}"))
                        return
                        ;;
                esac
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
        status)     COMPREPLY=($(compgen -W "${global_flags}" -- "${cur}")) ;;
        diff)       COMPREPLY=($(compgen -W "${diff_flags} ${global_flags}" -- "${cur}")) ;;
        backup)     COMPREPLY=($(compgen -W "${backup_flags} ${global_flags}" -- "${cur}")) ;;
        restore)    COMPREPLY=($(compgen -W "${restore_flags} ${global_flags}" -- "${cur}")) ;;
        collect)    COMPREPLY=($(compgen -W "${collect_flags} ${global_flags}" -- "${cur}")) ;;
        pull)       COMPREPLY=($(compgen -W "${pull_flags} ${global_flags}" -- "${cur}")) ;;
        push)       COMPREPLY=($(compgen -W "${push_flags} ${global_flags}" -- "${cur}")) ;;
        commit)     COMPREPLY=($(compgen -W "${commit_flags} ${global_flags}" -- "${cur}")) ;;
        doctor)     COMPREPLY=($(compgen -W "${doctor_flags} ${global_flags}" -- "${cur}")) ;;
        target)     COMPREPLY=($(compgen -W "${target_flags} ${global_flags}" -- "${cur}")) ;;
        upgrade)    COMPREPLY=($(compgen -W "${upgrade_flags} ${global_flags}" -- "${cur}")) ;;
        update)     COMPREPLY=($(compgen -W "${update_flags} ${global_flags}" -- "${cur}")) ;;
        check)      COMPREPLY=($(compgen -W "${check_flags} ${global_flags}" -- "${cur}")) ;;
        trash)      COMPREPLY=($(compgen -W "${trash_flags} ${global_flags}" -- "${cur}")) ;;
        audit)      COMPREPLY=($(compgen -W "${audit_flags} ${global_flags}" -- "${cur}")) ;;
        hub)        COMPREPLY=($(compgen -W "${hub_flags} ${global_flags}" -- "${cur}")) ;;
        log)        COMPREPLY=($(compgen -W "${log_flags} ${global_flags}" -- "${cur}")) ;;
        ui)         COMPREPLY=($(compgen -W "${ui_flags}" -- "${cur}")) ;;
        tui)        COMPREPLY=($(compgen -W "on off" -- "${cur}")) ;;
        enable)     COMPREPLY=($(compgen -W "${enable_flags} ${global_flags}" -- "${cur}")) ;;
        disable)    COMPREPLY=($(compgen -W "${disable_flags} ${global_flags}" -- "${cur}")) ;;
        analyze)    COMPREPLY=($(compgen -W "${analyze_flags} ${global_flags}" -- "${cur}")) ;;
        extras)     COMPREPLY=($(compgen -W "${extras_flags} ${global_flags}" -- "${cur}")) ;;
        completion) COMPREPLY=($(compgen -W "${completion_flags}" -- "${cur}")) ;;
        new)        COMPREPLY=() ;;
        search)     COMPREPLY=() ;;
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
