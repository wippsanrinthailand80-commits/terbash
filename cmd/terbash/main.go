package main

import (
	"fmt"
	"os"

	"github.com/terbash/terbash/cmd/terbash/cmd"
)

// Set via Makefile LDFLAGS (-X main.version=... etc.).
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	cmd.Version = version
	cmd.Commit = commit
	cmd.Date = date
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
