package main

import (
	"fmt"
	"os"
	"path/filepath"

	"skillshare/internal/ui"
)

func cmdCompletion(args []string) error {
	var shell string
	var install bool

	for _, a := range args {
		switch a {
		case "--install":
			install = true
		case "--help", "-h":
			printCompletionUsage()
			return nil
		default:
			if shell == "" {
				shell = a
			}
		}
	}

	if shell == "" {
		printCompletionUsage()
		return nil
	}

	var script string
	switch shell {
	case "bash":
		script = bashCompletionScript
	case "zsh":
		script = zshCompletionScript
	case "fish":
		script = fishCompletionScript
	case "powershell":
		script = powershellCompletionScript
	case "nushell":
		script = nushellCompletionScript
	default:
		return fmt.Errorf("unsupported shell: %s (supported: bash, zsh, fish, powershell, nushell)", shell)
	}

	if !install {
		fmt.Print(script)
		return nil
	}

	return installCompletion(shell, script)
}

func installCompletion(shell, script string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	var destPath string
	switch shell {
	case "bash":
		destPath = filepath.Join(home, ".local", "share", "bash-completion", "completions", "skillshare")
	case "zsh":
		destPath = filepath.Join(home, ".zsh", "completions", "_skillshare")
	case "fish":
		destPath = filepath.Join(home, ".config", "fish", "completions", "skillshare.fish")
	case "powershell":
		destPath = filepath.Join(home, ".config", "powershell", "completions", "skillshare.ps1")
	case "nushell":
		destPath = filepath.Join(home, ".config", "nushell", "completions", "skillshare.nu")
	}

	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("cannot create directory %s: %w", dir, err)
	}

	if err := os.WriteFile(destPath, []byte(script), 0o644); err != nil {
		return fmt.Errorf("cannot write completion script: %w", err)
	}

	ui.Success("Completion script installed to %s", destPath)
	fmt.Println()

	switch shell {
	case "bash":
		ui.Info("Restart your shell or run:")
		fmt.Printf("  source %s\n", destPath)
	case "zsh":
		ui.Info("Add the following to your .zshrc (if not already present):")
		fmt.Printf("  fpath=(~/.zsh/completions $fpath)\n")
		fmt.Printf("  autoload -Uz compinit && compinit\n")
		fmt.Println()
		ui.Info("Then restart your shell or run: exec zsh")
	case "fish":
		ui.Info("Completions will be available in new fish sessions automatically.")
	case "powershell":
		ui.Info("Add the following to your PowerShell profile:")
		fmt.Printf("  . %s\n", destPath)
		fmt.Println()
		ui.Info("To find your profile path, run: echo $PROFILE")
	case "nushell":
		ui.Info("Add the following to your Nushell config:")
		fmt.Printf("  source %s\n", destPath)
		fmt.Println()
		ui.Info("Or add it to $nu.config-path")
	}

	return nil
}

func printCompletionUsage() {
	fmt.Println("Generate shell completion scripts")
	fmt.Println()
	fmt.Println("USAGE")
	fmt.Println("  skillshare completion <shell>             Output completion script to stdout")
	fmt.Println("  skillshare completion <shell> --install   Install completion script")
	fmt.Println()
	fmt.Println("SHELLS")
	fmt.Println("  bash, zsh, fish, powershell, nushell")
	fmt.Println()
	fmt.Println("EXAMPLES")
	fmt.Println("  skillshare completion bash --install        Install bash completions")
	fmt.Println("  skillshare completion zsh --install         Install zsh completions")
	fmt.Println("  skillshare completion fish --install        Install fish completions")
	fmt.Println("  skillshare completion powershell --install  Install PowerShell completions")
	fmt.Println("  skillshare completion nushell --install     Install Nushell completions")
	fmt.Println("  skillshare completion bash                  Print script to stdout")
}
