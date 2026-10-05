package main

const powershellCompletionScript = `# skillshare PowerShell completion

$_skillshareCompleter = {
    param($wordToComplete, $commandAst, $cursorPosition)

    $commands = @{
        '' = @(
            @{ Name = 'init'; Desc = 'Initialize skillshare' }
            @{ Name = 'install'; Desc = 'Install skills/agents from local path or git repo' }
            @{ Name = 'uninstall'; Desc = 'Remove skills/agents from source directory' }
            @{ Name = 'list'; Desc = 'List installed skills' }
            @{ Name = 'search'; Desc = 'Search or browse GitHub for skills' }
            @{ Name = 'sync'; Desc = 'Sync skills/agents/extras/MCP to targets' }
            @{ Name = 'plugin'; Desc = 'Manage complete native plugins' }
            @{ Name = 'mcp'; Desc = 'Manage MCP connections' }
            @{ Name = 'hooks'; Desc = 'Manage Agent hooks' }
            @{ Name = 'status'; Desc = 'Show status of all targets' }
            @{ Name = 'diff'; Desc = 'Show differences between source and targets' }
            @{ Name = 'backup'; Desc = 'Create backup of targets' }
            @{ Name = 'restore'; Desc = 'Restore target from backup' }
            @{ Name = 'collect'; Desc = 'Collect local skills/agents from targets' }
            @{ Name = 'pull'; Desc = 'Pull from git remote and sync' }
            @{ Name = 'push'; Desc = 'Commit and push source to git remote' }
            @{ Name = 'commit'; Desc = 'Create local git commit without pushing' }
            @{ Name = 'doctor'; Desc = 'Check environment and diagnose issues' }
            @{ Name = 'target'; Desc = 'Manage targets' }
            @{ Name = 'upgrade'; Desc = 'Upgrade CLI and/or skillshare skill' }
            @{ Name = 'update'; Desc = 'Update skills/agents or tracked repositories' }
            @{ Name = 'check'; Desc = 'Check for available updates' }
            @{ Name = 'new'; Desc = 'Create a new skill' }
            @{ Name = 'trash'; Desc = 'Manage trashed skills/agents' }
            @{ Name = 'analyze'; Desc = 'Analyze skills/agents' }
            @{ Name = 'audit'; Desc = 'Scan skills/agents for security threats' }
            @{ Name = 'hub'; Desc = 'Manage hubs' }
            @{ Name = 'log'; Desc = 'View operation log' }
            @{ Name = 'ui'; Desc = 'Launch web dashboard' }
            @{ Name = 'tui'; Desc = 'Toggle interactive TUI mode' }
            @{ Name = 'extras'; Desc = 'Manage extra resource types' }
            @{ Name = 'enable'; Desc = 'Enable a disabled skill/agent' }
            @{ Name = 'disable'; Desc = 'Disable a skill/agent' }
            @{ Name = 'completion'; Desc = 'Generate shell completion scripts' }
            @{ Name = 'version'; Desc = 'Show version' }
            @{ Name = 'help'; Desc = 'Show help' }
        )
        'mcp' = @(
            @{ Name = 'add'; Desc = 'MCP add' }
            @{ Name = 'check'; Desc = 'MCP check' }
            @{ Name = 'edit'; Desc = 'MCP edit' }
            @{ Name = 'import'; Desc = 'MCP import' }
            @{ Name = 'list'; Desc = 'MCP list' }
            @{ Name = 'remove'; Desc = 'MCP remove' }
            @{ Name = 'restore'; Desc = 'MCP restore' }
        )
        'hooks' = @(
            @{ Name = 'add'; Desc = 'Hooks add' }
            @{ Name = 'disable'; Desc = 'Hooks disable' }
            @{ Name = 'edit'; Desc = 'Hooks edit' }
            @{ Name = 'enable'; Desc = 'Hooks enable' }
            @{ Name = 'import'; Desc = 'Hooks import' }
            @{ Name = 'list'; Desc = 'Hooks list' }
            @{ Name = 'remove'; Desc = 'Hooks remove' }
            @{ Name = 'restore'; Desc = 'Hooks restore' }
            @{ Name = 'sync'; Desc = 'Hooks sync' }
        )
        'plugin' = @(
            @{ Name = 'add'; Desc = 'Plugin add' }
            @{ Name = 'discover'; Desc = 'Plugin discover' }
            @{ Name = 'import'; Desc = 'Plugin import' }
            @{ Name = 'list'; Desc = 'Plugin list' }
            @{ Name = 'inspect'; Desc = 'Plugin inspect' }
            @{ Name = 'sync'; Desc = 'Plugin sync' }
            @{ Name = 'check'; Desc = 'Plugin check' }
            @{ Name = 'update'; Desc = 'Plugin update' }
            @{ Name = 'enable'; Desc = 'Plugin enable' }
            @{ Name = 'disable'; Desc = 'Plugin disable' }
            @{ Name = 'remove'; Desc = 'Plugin remove' }
        )
        'target' = @(
            @{ Name = 'add'; Desc = 'Add a target' }
            @{ Name = 'remove'; Desc = 'Unlink target and restore skills' }
            @{ Name = 'list'; Desc = 'List all targets' }
        )
        'trash' = @(
            @{ Name = 'list'; Desc = 'List trashed items' }
            @{ Name = 'restore'; Desc = 'Restore from trash' }
            @{ Name = 'delete'; Desc = 'Delete permanently' }
            @{ Name = 'empty'; Desc = 'Clear all trash' }
            @{ Name = 'agents'; Desc = 'Trashed agents' }
        )
        'hub' = @(
            @{ Name = 'add'; Desc = 'Add hub' }
            @{ Name = 'list'; Desc = 'List hubs' }
            @{ Name = 'remove'; Desc = 'Remove hub' }
            @{ Name = 'default'; Desc = 'Set default hub' }
            @{ Name = 'index'; Desc = 'Create skill index' }
        )
        'extras' = @(
            @{ Name = 'init'; Desc = 'Create extra resource type' }
            @{ Name = 'list'; Desc = 'List extras with sync status' }
            @{ Name = 'remove'; Desc = 'Remove extra resource type' }
            @{ Name = 'collect'; Desc = 'Collect local files into extras' }
            @{ Name = 'source'; Desc = 'Show/set extras source' }
            @{ Name = 'memory'; Desc = 'Manage shared Markdown notes' }
        )
        'extras memory' = @(
            @{ Name = 'init'; Desc = 'Memory init' }
            @{ Name = 'list'; Desc = 'Memory list' }
            @{ Name = 'show'; Desc = 'Memory show' }
            @{ Name = 'write'; Desc = 'Memory write' }
            @{ Name = 'delete'; Desc = 'Memory delete' }
            @{ Name = 'instructions'; Desc = 'Memory instructions' }
        )
        'backup' = @(
            @{ Name = 'files'; Desc = 'Versions of single files skillshare rewrote' }
            @{ Name = 'agents'; Desc = 'Back up agents' }
        )
        'audit' = @(
            @{ Name = 'rules'; Desc = 'Manage security rules' }
            @{ Name = 'agents'; Desc = 'Audit agents' }
        )
        'completion' = @(
            @{ Name = 'bash'; Desc = 'Generate bash completions' }
            @{ Name = 'zsh'; Desc = 'Generate zsh completions' }
            @{ Name = 'fish'; Desc = 'Generate fish completions' }
            @{ Name = 'powershell'; Desc = 'Generate PowerShell completions' }
            @{ Name = 'nushell'; Desc = 'Generate Nushell completions' }
        )
        'backup files' = @(
            @{ Name = 'list'; Desc = 'List files with saved versions' }
            @{ Name = 'show'; Desc = 'Show versions of one file' }
            @{ Name = 'restore'; Desc = 'Restore a saved version' }
        )
        'audit rules' = @(
            @{ Name = 'disable'; Desc = 'Disable a rule' }
            @{ Name = 'enable'; Desc = 'Enable a rule' }
            @{ Name = 'severity'; Desc = 'Override rule severity' }
            @{ Name = 'reset'; Desc = 'Reset rule overrides' }
            @{ Name = 'init'; Desc = 'Create rules file' }
        )
        'ui' = @(
            @{ Name = 'start'; Desc = 'Start a background UI server' }
            @{ Name = 'stop'; Desc = 'Stop a background UI server' }
        )
        'sync' = @(
            @{ Name = 'agents'; Desc = 'Sync agents' }
            @{ Name = 'extras'; Desc = 'Sync extras' }
            @{ Name = 'mcp'; Desc = 'Sync MCP connections' }
            @{ Name = 'hooks'; Desc = 'Sync hooks' }
            @{ Name = 'plugins'; Desc = 'Sync plugins' }
        )
        'list' = @(
            @{ Name = 'agents'; Desc = 'List agents' }
        )
        'uninstall' = @(
            @{ Name = 'agents'; Desc = 'Uninstall agents' }
        )
        'diff' = @(
            @{ Name = 'agents'; Desc = 'Diff agents' }
        )
        'restore' = @(
            @{ Name = 'agents'; Desc = 'Restore agents' }
        )
        'collect' = @(
            @{ Name = 'agents'; Desc = 'Collect agents' }
        )
        'check' = @(
            @{ Name = 'agents'; Desc = 'Check agents' }
        )
        'update' = @(
            @{ Name = 'agents'; Desc = 'Update agents' }
        )
        'tui' = @(
            @{ Name = 'on'; Desc = 'Enable TUI mode' }
            @{ Name = 'off'; Desc = 'Disable TUI mode' }
        )
    }

    $flags = @{
        'init' = '--source', '-s', '--remote', '--copy-from', '-c', '--no-copy', '--targets', '-t', '--all-targets', '--no-targets', '--mode', '-m', '--git', '--no-git', '--git-root', '--skill', '--no-skill', '--discover', '-d', '--select', '--subdir', '--visible', '--config', '--dry-run', '-n', '--help', '-h', '--project', '-p', '--global', '-g'
        'install' = '--name', '--force', '-f', '--update', '-u', '--dry-run', '-n', '--skip-audit', '--audit-verbose', '--audit-threshold', '--threshold', '-T', '--branch', '-b', '--track', '-t', '--kind', '--agent', '-a', '--skill', '--exclude', '--into', '--all', '--yes', '-y', '--json', '--help', '-h', '--project', '-p', '--global', '-g'
        'uninstall' = '--all', '--force', '-f', '--dry-run', '-n', '--json', '--group', '-G', '--help', '-h', '--project', '-p', '--global', '-g'
        'list' = '--verbose', '-v', '--json', '-j', '--no-tui', '--type', '-t', '--status', '--sort', '-s', '--all', '--help', '-h', '--project', '-p', '--global', '-g'
        'sync' = '--all', '--dry-run', '-n', '--force', '-f', '--json', '--quiet', '-q', '--help', '-h', '--project', '-p', '--global', '-g'
        'diff' = '--no-tui', '--patch', '--stat', '--json', '--help', '-h', '--project', '-p', '--global', '-g'
        'backup' = '--list', '-l', '--cleanup', '-c', '--delete', '--all', '--unlink', '--dry-run', '-n', '--target', '-t', '--help', '-h', '--project', '-p', '--global', '-g'
        'restore' = '--from', '-f', '--force', '--all', '--dry-run', '-n', '--no-tui', '--help', '-h', '--project', '-p', '--global', '-g'
        'collect' = '--all', '-a', '--dry-run', '-n', '--force', '-f', '--json', '--help', '-h', '--project', '-p', '--global', '-g'
        'pull' = '--dry-run', '-n', '--force', '-f', '--help', '-h'
        'push' = '--dry-run', '-n', '--pull', '--message', '-m', '--help', '-h'
        'commit' = '--dry-run', '-n', '--message', '-m', '--help', '-h'
        'doctor' = '--json', '--help', '-h', '--project', '-p', '--global', '-g'
        'target' = '--json', '--no-tui', '--help', '-h', '--mode', '-m', '--agent-mode', '--target-naming', '--add-include', '--add-exclude', '--remove-include', '--remove-exclude', '--add-agent-include', '--add-agent-exclude', '--remove-agent-include', '--remove-agent-exclude', '--agent', '--config-dir', '--cli', '--skills', '--no-skills', '--all', '-a', '--dry-run', '-n', '--project', '-p', '--global', '-g'
        'upgrade' = '--dry-run', '-n', '--force', '-f', '--skill', '--cli', '--help', '-h'
        'update' = '--all', '-a', '--dry-run', '-n', '--force', '-f', '--skip-audit', '--audit-threshold', '--threshold', '-T', '--diff', '--audit-verbose', '--prune', '--json', '--group', '-G', '--help', '-h', '--project', '-p', '--global', '-g'
        'check' = '--json', '--all', '--group', '-G', '--help', '-h', '--project', '-p', '--global', '-g'
        'trash' = '--all', '--no-tui', '--help', '-h', '--project', '-p', '--global', '-g'
        'audit' = '--init-rules', '--json', '--format', '--quiet', '-q', '--yes', '-y', '--no-tui', '--threshold', '-T', '--group', '-G', '--profile', '--dedupe', '--analyzer', '--pattern', '--severity', '--disabled', '--help', '-h', '--project', '-p', '--global', '-g'
        'hub' = '--source', '-s', '--output', '-o', '--full', '--audit', '--label', '-l', '--reset', '--help', '-h', '--project', '-p', '--global', '-g'
        'log' = '--audit', '-a', '--clear', '-c', '--json', '--no-tui', '--stats', '--cmd', '--status', '--since', '--tail', '-t', '--help', '-h', '--project', '-p', '--global', '-g'
        'ui' = '--port', '--host', '--base-path', '-b', '--no-open', '--clear-cache', '--app', '--help', '-h', '--project', '-p', '--global', '-g'
        'enable' = '--dry-run', '-n', '--kind', '--help', '-h', '--project', '-p', '--global', '-g'
        'disable' = '--dry-run', '-n', '--kind', '--help', '-h', '--project', '-p', '--global', '-g'
        'analyze' = '--verbose', '-v', '--filter', '--no-tui', '--json', '--help', '-h', '--project', '-p', '--global', '-g'
        'mcp' = '--tools-allow', '--tools-deny', '--pi-options', '--target', '--from', '--url', '--file', '--sync', '--replace', '--disabled', '--keep-files', '--revision', '--dry-run', '-n', '--json', '--no-dns', '--live', '--timeout', '--no-tui', '--help', '-h', '--project', '-p', '--global', '-g'
        'plugin' = '--target', '--from', '--plugin', '--name', '--source-ref', '--entry', '--revision', '--dry-run', '-n', '--json', '--no-tui', '--help', '-h', '--project', '-p', '--global', '-g'
        'extras memory' = '--from', '--version', '--search', '--update-mode', '--json', '--help', '-h', '--project', '-p', '--global', '-g'
        'extras' = '--mode', '--target', '--source', '--file', '--as', '--flatten', '--no-flatten', '--add-target', '--remove-target', '--prune', '--from', '--dry-run', '--force', '-f', '--json', '--no-tui', '--help', '-h', '--project', '-p', '--global', '-g'
        'status' = '--json', '--help', '-h', '--project', '-p', '--global', '-g'
        'new' = '--pattern', '-P', '--dry-run', '-n', '--help', '-h', '--project', '-p', '--global', '-g'
        'search' = '--json', '--list', '-l', '--hub', '--limit', '-n', '--help', '-h', '--project', '-p', '--global', '-g'
        'tui' = '--help', '-h'
        'hooks' = '--file', '--from', '--sync', '--replace', '--keep-files', '--revision', '--dry-run', '-n', '--json', '--help', '-h', '--project', '-p', '--global', '-g'
        'completion' = '--install', '--help', '-h'
    }

    $elements = $commandAst.ToString().Split(' ', [StringSplitOptions]::RemoveEmptyEntries)
    $cmd = if ($elements.Count -gt 1) { $elements[1] } else { '' }
    $subcmd = if ($elements.Count -gt 2) { $elements[2] } else { '' }

    $previousIndex = $elements.Count - 1
    if ($wordToComplete -ne '') { $previousIndex-- }
    if ($cmd -eq 'mcp' -and $previousIndex -ge 0 -and $elements[$previousIndex] -in @('--target', '--from')) {
        @('claude', 'codex', 'cursor', 'vscode', 'opencode', 'kilocode', 'grok', 'antigravity', 'amp', 'claude-desktop', 'cline', 'copilot', 'factory', 'gemini', 'goose', 'junie', 'kiro', 'lmstudio', 'warp', 'windsurf', 'pi', 'omp') | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
            [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
        }
        return
    }
    if ($cmd -eq 'target' -and $previousIndex -ge 0 -and $elements[$previousIndex] -eq '--agent') {
        @('claude', 'codex', 'pi', 'omp') | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
            [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
        }
        return
    }
    if ($cmd -eq 'plugin' -and $previousIndex -ge 0 -and $elements[$previousIndex] -in @('--target', '--from')) {
        @('claude', 'codex', 'cursor', 'antigravity', 'agy', 'antigravity-cli', 'copilot', 'grok', 'kimi', 'hermes', 'devin', 'pi', 'opencode', 'omp') | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
            [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
        }
        return
    }

    # Complete subcommands
    if ($elements.Count -le 2 -or ($elements.Count -eq 2 -and $wordToComplete -ne '')) {
        $key = if ($elements.Count -le 1 -or ($elements.Count -eq 2 -and $wordToComplete -ne '')) { '' } else { $cmd }
        if ($commands.ContainsKey($key)) {
            $commands[$key] | Where-Object { $_.Name -like "$wordToComplete*" } | ForEach-Object {
                [System.Management.Automation.CompletionResult]::new($_.Name, $_.Name, 'ParameterValue', $_.Desc)
            }
            return
        }
    }

    # Complete a partially typed subcommand
    if ($elements.Count -eq 3 -and $wordToComplete -ne '' -and -not $wordToComplete.StartsWith('-') -and $commands.ContainsKey($cmd)) {
        $commands[$cmd] | Where-Object { $_.Name -like "$wordToComplete*" } | ForEach-Object {
            [System.Management.Automation.CompletionResult]::new($_.Name, $_.Name, 'ParameterValue', $_.Desc)
        }
        return
    }

    # Complete nested subcommands (backup files, audit rules)
    $nested = "$cmd $subcmd"
    if ((($elements.Count -eq 3 -and $wordToComplete -eq '') -or ($elements.Count -eq 4 -and $wordToComplete -ne '')) -and -not $wordToComplete.StartsWith('-') -and $commands.ContainsKey($nested)) {
        $commands[$nested] | Where-Object { $_.Name -like "$wordToComplete*" } | ForEach-Object {
            [System.Management.Automation.CompletionResult]::new($_.Name, $_.Name, 'ParameterValue', $_.Desc)
        }
        return
    }

    # Complete flags
    if ($wordToComplete.StartsWith('-')) {
        $flagKey = if ($flags.ContainsKey($nested)) { $nested } elseif ($flags.ContainsKey($cmd)) { $cmd } else { '' }
        if ($flags.ContainsKey($flagKey)) {
            $flags[$flagKey] | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterName', $_)
            }
        }
    }
}

Register-ArgumentCompleter -Native -CommandName skillshare -ScriptBlock $_skillshareCompleter

# Auto-detect aliases pointing to skillshare and register completion for them
Get-Alias -ErrorAction SilentlyContinue | Where-Object { $_.Definition -eq 'skillshare' } | ForEach-Object {
    Register-ArgumentCompleter -Native -CommandName $_.Name -ScriptBlock $_skillshareCompleter
}
`
