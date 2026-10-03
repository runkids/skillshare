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
			ui.Next("source "+shortenPath(p), "load it in this shell, or restart it")
		},
	},
	"zsh": {
		script: zshCompletionScript,
		installPath: func(home string) string {
			return filepath.Join(home, ".zsh", "completions", "_skillshare")
		},
		postInstall: func(_ string) {
			printShellSetup("Add to ~/.zshrc", "fpath=(~/.zsh/completions $fpath)", "autoload -Uz compinit && compinit")
			ui.Next("exec zsh", "reload your shell")
		},
	},
	"fish": {
		script: fishCompletionScript,
		installPath: func(home string) string {
			return filepath.Join(home, ".config", "fish", "completions", "skillshare.fish")
		},
		postInstall: func(_ string) {
			ui.Note("New fish sessions load it automatically")
		},
	},
	"powershell": {
		script: powershellCompletionScript,
		installPath: func(home string) string {
			return filepath.Join(home, ".config", "powershell", "completions", "skillshare.ps1")
		},
		postInstall: func(p string) {
			printShellSetup("Add to your PowerShell profile"+ui.DimText(" · echo $PROFILE shows where"), ". "+shortenPath(p))
		},
	},
	"nushell": {
		script: nushellCompletionScript,
		installPath: func(home string) string {
			return filepath.Join(home, ".config", "nushell", "completions", "skillshare.nu")
		},
		postInstall: func(p string) {
			printShellSetup("Add to your Nushell config"+ui.DimText(" · $nu.config-path shows where"), "source "+shortenPath(p))
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

	ui.Done(ui.MarkOK, "Completion installed to "+shortenPath(destPath), 0)
	def.postInstall(destPath)

	return nil
}

// printShellSetup prints a bold heading and the lines to paste into a shell
// config file, left plain so they copy cleanly.
func printShellSetup(heading string, lines ...string) {
	fmt.Println()
	fmt.Println(ui.Bold + heading + ui.Reset)
	for _, line := range lines {
		fmt.Println("  " + line)
	}
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
