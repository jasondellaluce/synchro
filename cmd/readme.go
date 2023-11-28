package cmd

import (
	"fmt"
	"os"

	"github.com/jasondellaluce/synchro/pkg/utils"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(readmeCmd)
}

var readmeCmd = &cobra.Command{
	Use:   "readme",
	Short: "Generates a readme for the project",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(os.Stdout, "# %s\n\n", utils.ProjectName)
		fmt.Fprintf(os.Stdout, "_%s_\n\n", utils.ProjectDescription)
		fmt.Fprintf(os.Stdout, "## Installing\n\n")
		fmt.Fprintf(os.Stdout, "`go install %s@latest`\n\n", utils.PackageName)
		fmt.Fprintf(os.Stdout, "#")
		explainMarkersCmd.Run(cmd, args)
		fmt.Fprintf(os.Stdout, "\n#")
		explainConflictsCmd.Run(cmd, args)
	},
}
