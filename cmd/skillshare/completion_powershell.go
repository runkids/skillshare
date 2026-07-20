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
            @{ Name = 'sync'; Desc = 'Sync skills/agents/extras to targets' }
            @{ Name = 'status'; Desc = 'Show status of all targets' }
            @{ Name = 'diff'; Desc = 'Show differences between source and targets' }
            @{ Name = 'backup'; Desc = 'Create backup of targets' }
            @{ Name = 'restore'; Desc = 'Restore target from backup' }
            @{ Name = 'collect'; Desc = 'Collect local skills/agents from targets' }
            @{ Name = 'adopt'; Desc = 'Adopt externally installed skills into skillshare' }
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
            @{ Name = 'mode'; Desc = 'Change sync mode or flatten' }
        )
        'backup' = @(
            @{ Name = 'restore'; Desc = 'Restore from backup' }
        )
        'audit' = @(
            @{ Name = 'rules'; Desc = 'Manage security rules' }
        )
        'completion' = @(
            @{ Name = 'bash'; Desc = 'Generate bash completions' }
            @{ Name = 'zsh'; Desc = 'Generate zsh completions' }
            @{ Name = 'fish'; Desc = 'Generate fish completions' }
            @{ Name = 'powershell'; Desc = 'Generate PowerShell completions' }
            @{ Name = 'nushell'; Desc = 'Generate Nushell completions' }
        )
        'tui' = @(
            @{ Name = 'on'; Desc = 'Enable TUI mode' }
            @{ Name = 'off'; Desc = 'Disable TUI mode' }
        )
    }

    $flags = @{
        'init' = '--source', '-s', '--remote', '--copy-from', '-c', '--no-copy', '--targets', '-t', '--all-targets', '--no-targets', '--mode', '-m', '--git', '--no-git', '--skill', '--no-skill', '--discover', '-d', '--select', '--subdir', '--dry-run', '-n', '--help', '-h', '--project', '-p', '--global', '-g'
        'install' = '--source', '-s', '--name', '--force', '-f', '--update', '-u', '--dry-run', '-n', '--skip-audit', '--audit-verbose', '--audit-threshold', '--threshold', '-T', '--branch', '-b', '--track', '-t', '--kind', '--agent', '-a', '--skill', '--exclude', '--into', '--all', '--yes', '-y', '--json', '--help', '-h', '--project', '-p', '--global', '-g'
        'uninstall' = '--all', '--force', '-f', '--dry-run', '-n', '--json', '--group', '-G', '--help', '-h', '--project', '-p', '--global', '-g'
        'list' = '--verbose', '-v', '--json', '-j', '--no-tui', '--type', '-t', '--sort', '-s', '--all', '--help', '-h', '--project', '-p', '--global', '-g'
        'sync' = '--all', '--dry-run', '-n', '--force', '-f', '--json', '--help', '-h', '--project', '-p', '--global', '-g'
        'diff' = '--no-tui', '--patch', '--stat', '--json', '--help', '-h', '--project', '-p', '--global', '-g'
        'backup' = '--list', '-l', '--cleanup', '-c', '--dry-run', '-n', '--target', '-t', '--help', '-h', '--project', '-p', '--global', '-g'
        'restore' = '--from', '-f', '--force', '--dry-run', '-n', '--no-tui', '--help', '-h', '--project', '-p', '--global', '-g'
        'collect' = '--all', '-a', '--dry-run', '-n', '--force', '-f', '--json', '--help', '-h', '--project', '-p', '--global', '-g'
        'adopt' = '--all', '-a', '--dry-run', '-n', '--force', '-f', '--json', '--help', '-h', '--project', '-p', '--global', '-g'
        'pull' = '--dry-run', '-n', '--force', '-f', '--project', '-p', '--global', '-g'
        'push' = '--dry-run', '-n', '--message', '-m', '--project', '-p', '--global', '-g'
        'commit' = '--dry-run', '-n', '--message', '-m', '--help', '-h', '--project', '-p', '--global', '-g'
        'doctor' = '--json', '--help', '-h'
        'target' = '--json', '--no-tui', '--help', '-h', '--mode', '--agent-mode', '--target-naming', '--add-include', '--add-exclude', '--remove-include', '--remove-exclude', '--add-agent-include', '--add-agent-exclude', '--remove-agent-include', '--remove-agent-exclude', '--project', '-p', '--global', '-g'
        'upgrade' = '--dry-run', '-n', '--force', '-f', '--skill', '--cli', '--help', '-h'
        'update' = '--all', '-a', '--dry-run', '-n', '--force', '-f', '--skip-audit', '--audit-threshold', '--threshold', '-T', '--diff', '--audit-verbose', '--prune', '--json', '--group', '-G', '--help', '-h', '--project', '-p', '--global', '-g'
        'check' = '--json', '--all', '--group', '-G', '--help', '-h', '--project', '-p', '--global', '-g'
        'trash' = '--no-tui', '--help', '-h', '--project', '-p', '--global', '-g'
        'audit' = '--init-rules', '--json', '--format', '--quiet', '-q', '--yes', '-y', '--no-tui', '--threshold', '-T', '--group', '-G', '--profile', '--dedupe', '--analyzer', '--help', '-h', '--project', '-p', '--global', '-g'
        'hub' = '--help', '-h', '--project', '-p', '--global', '-g'
        'log' = '--audit', '-a', '--clear', '-c', '--json', '--no-tui', '--stats', '--cmd', '--status', '--since', '--tail', '-t', '--help', '-h', '--project', '-p', '--global', '-g'
        'ui' = '--port', '--host', '--no-open', '--help', '-h'
        'enable' = '--dry-run', '-n', '--help', '-h', '--project', '-p', '--global', '-g'
        'disable' = '--dry-run', '-n', '--help', '-h', '--project', '-p', '--global', '-g'
        'analyze' = '--no-tui', '--json', '--help', '-h', '--project', '-p', '--global', '-g'
        'extras' = '--help', '-h', '--project', '-p', '--global', '-g'
        'completion' = '--install', '--help', '-h'
    }

    $elements = $commandAst.ToString().Split(' ', [StringSplitOptions]::RemoveEmptyEntries)
    $cmd = if ($elements.Count -gt 1) { $elements[1] } else { '' }
    $subcmd = if ($elements.Count -gt 2) { $elements[2] } else { '' }

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

    # Complete sub-subcommands
    if ($elements.Count -eq 3 -and $wordToComplete -ne '' -and $commands.ContainsKey($cmd)) {
        $commands[$cmd] | Where-Object { $_.Name -like "$wordToComplete*" } | ForEach-Object {
            [System.Management.Automation.CompletionResult]::new($_.Name, $_.Name, 'ParameterValue', $_.Desc)
        }
        return
    }

    # Complete flags
    if ($wordToComplete.StartsWith('-')) {
        $flagKey = if ($flags.ContainsKey($cmd)) { $cmd } else { '' }
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
