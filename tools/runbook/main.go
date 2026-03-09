package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: runbook <file.md|directory>")
		os.Exit(1)
	}
	fmt.Printf("runbook: %s\n", os.Args[1])
}
