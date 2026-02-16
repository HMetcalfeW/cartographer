package version

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Build-time variables set via -ldflags.
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

// VersionCmd prints version information.
var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of Cartographer",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("cartographer %s (commit: %s, built: %s)\n", Version, Commit, Date)
	},
}
