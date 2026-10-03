package main

import (
	"fmt"
	"os"
	"path/filepath"

	"skillshare/internal/ui"
)

type shellDef struct {
	script      string
	installPath func(home string) string
	postInstall func(destPath string)
}

var shells = map[string]shellDef{
	"bash": {
		script: bashCompletionScript,
		installPath: func(home string) string {
			return filepath.Join(home, ".local", "share", "bash-completion", "completions", "skillshare")
		},
		postInstall: func(p string) {
			ui.Info("Restart your shell or run:")
			ui.Info("  source %s", p)
		},
	},
	"zsh": {
		script: zshCompletionScript,
		installPath: func(home string) string {
			return filepath.Join(home, ".zsh", "completions", "_skillshare")
		},
		postInstall: func(_ string) {
			ui.Info("Add the following to your .zshrc (if not already present):")
			ui.Info("  fpath=(~/.zsh/completions $fpath)")
			ui.Info("  autoload -Uz compinit && compinit")
			ui.Info("Then restart your shell or run: exec zsh")
		},
	},
	"fish": {
		script: fishCompletionScript,
		installPath: func(home string) string {
			return filepath.Join(home, ".config", "fish", "completions", "skillshare.fish")
		},
		postInstall: func(_ string) {
			ui.Info("Completions will be available in new fish sessions automatically.")
		},
	},
	"powershell": {
		script: powershellCompletionScript,
		installPath: func(home string) string {
			return filepath.Join(home, ".config", "powershell", "completions", "skillshare.ps1")
		},
		postInstall: func(p string) {
			ui.Info("Add the following to your PowerShell profile:")
			ui.Info("  . %s", p)
			ui.Info("To find your profile path, run: echo $PROFILE")
		},
	},
	"nushell": {
		script: nushellCompletionScript,
		installPath: func(home string) string {
			return filepath.Join(home, ".config", "nushell", "completions", "skillshare.nu")
		},
		postInstall: func(p string) {
			ui.Info("Add the following to your Nushell config:")
			ui.Info("  source %s", p)
			ui.Info("Or add it to $nu.config-path")
		},
	},
}

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

	def, ok := shells[shell]
	if !ok {
		return fmt.Errorf("unsupported shell: %s (supported: bash, zsh, fish, powershell, nushell)", shell)
	}

	if !install {
		fmt.Print(def.script)
		return nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	destPath := def.installPath(home)
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("cannot create directory %s: %w", dir, err)
	}
	if err := os.WriteFile(destPath, []byte(def.script), 0o644); err != nil {
		return fmt.Errorf("cannot write completion script: %w", err)
	}

	ui.Success("Completion script installed to %s", destPath)
	def.postInstall(destPath)

	return nil
}

func printCompletionUsage() {
	printHelp("skillshare completion <shell> [--install]", "Generate shell completion scripts.",
		helpGroup{title: "Commands", rows: []helpRow{
			{"completion <shell>", "Output completion script to stdout"},
			{"completion <shell> --install", "Install completion script"},
		}},
		helpNotes("Shells", "bash, zsh, fish, powershell, nushell"),
		helpExamples(
			helpRow{"skillshare completion bash --install", "Install bash completions"},
			helpRow{"skillshare completion zsh --install", "Install zsh completions"},
			helpRow{"skillshare completion fish --install", "Install fish completions"},
			helpRow{"skillshare completion powershell --install", "Install PowerShell completions"},
			helpRow{"skillshare completion nushell --install", "Install Nushell completions"},
			helpRow{"skillshare completion bash", "Print script to stdout"},
		),
	)
}
