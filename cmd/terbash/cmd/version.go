package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Set from main (Makefile LDFLAGS fill main.version/commit/date).
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print terbash version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("terbash %s (%s/%s)\n", Version, runtime.GOOS, runtime.GOARCH)
		fmt.Printf("commit: %s  built: %s\n", Commit, Date)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
