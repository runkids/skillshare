package main

import (
	"fmt"
	"strconv"
	"strings"

	"skillshare/internal/clidiscovery"
)

// cliUsageError codes are assigned where this command validates its own input,
// never inferred from the wording of errors returned by other commands.
type cliUsageError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

func (e *cliUsageError) Error() string { return e.Message }

func cliUsage(message string) error {
	return &cliUsageError{Code: "usage", Message: message, Hint: "Run 'skillshare cli search --help' for usage and examples."}
}

type cliSearchResponse struct {
	Query   string                `json:"query"`
	Results []clidiscovery.Result `json:"results"`
}

func cmdCLI(args []string) error {
	// Scan before validation so a late JSON flag also covers usage failures.
	// After --, even --json is a task word, not an output-mode flag.
	jsonMode := false
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if arg == "--json" || arg == "-j" {
			jsonMode = true
		}
	}
	err := runCLI(args, jsonMode)
	if usage, ok := err.(*cliUsageError); ok && jsonMode {
		return writeJSONResult(struct {
			Error *cliUsageError `json:"error"`
		}{usage}, err)
	}
	return err
}

func runCLI(args []string, jsonMode bool) error {
	if len(args) == 0 || len(args) == 1 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		printCLIHelp()
		return nil
	}
	if args[0] != "search" {
		return cliUsage(fmt.Sprintf("unknown cli command: %s", args[0]))
	}
	limit := 5
	task := []string{}
	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			task = append(task, args[i+1:]...)
			i = len(args)
		case arg == "--help" || arg == "-h":
			printCLIHelp()
			return nil
		case arg == "--json" || arg == "-j":
		case arg == "--limit" || strings.HasPrefix(arg, "--limit="):
			value := strings.TrimPrefix(arg, "--limit=")
			if arg == "--limit" {
				i++
				if i == len(args) {
					return cliUsage("--limit requires a number between 1 and 20")
				}
				value = args[i]
			}
			var err error
			limit, err = strconv.Atoi(value)
			if err != nil || limit < 1 || limit > 20 {
				return cliUsage("--limit must be a number between 1 and 20")
			}
		case strings.HasPrefix(arg, "-"):
			return cliUsage(fmt.Sprintf("unknown option: %s; put task words beginning with '-' after --", arg))
		default:
			task = append(task, arg)
		}
	}
	query := strings.Join(task, " ")
	if strings.TrimSpace(query) == "" {
		return cliUsage("search requires a non-empty task description")
	}
	response := cliSearchResponse{Query: query, Results: clidiscovery.Search(cliCommandCatalog(), query, limit)}
	if jsonMode {
		return writeJSON(response)
	}
	hint := "skillshare"
	for i, result := range response.Results {
		fmt.Printf("%s  %s\n", result.Command, result.Summary)
		if i == 0 {
			hint = result.Command
		}
	}
	fmt.Printf("Run '%s --help' for flags and examples.\n", hint)
	return nil
}

// cliCommandCatalog reuses the root help summaries and verifies each command
// against dispatch. It intentionally indexes top-level commands only: handlers
// own nested help, and invoking them here could load config or mutate state.
func cliCommandCatalog() []clidiscovery.Result {
	entries := []clidiscovery.Result{}
	for _, group := range rootHelpGroups() {
		for _, row := range group.rows {
			if commands[row.name] == nil {
				continue
			}
			switch row.name {
			case "cli", "completion", "ui", "tui":
				continue
			}
			command := "skillshare " + row.name
			entries = append(entries, clidiscovery.Result{Command: command, Summary: row.desc, Usage: command + " --help"})
		}
	}
	return entries
}

func printCLIHelp() {
	// Do not use themed help: resolving the theme can probe the terminal.
	fmt.Println(`Usage: skillshare cli search <task words...> [options]

Find top-level commands from their names and English help summaries.
Offline: no configuration, network access, or command execution.
Use a result's --help to find its subcommands, flags and examples.
To find remote skills instead, use 'skillshare search <query>'.

Options:
  --limit N     Maximum results, 1–20 (default 5)
  --json, -j    Machine-readable query and results
  --help, -h    Show this help
  --            Treat remaining arguments as task words

Examples:
  skillshare cli search "install skills"
  skillshare cli search remove --limit 3 --json`)
}
