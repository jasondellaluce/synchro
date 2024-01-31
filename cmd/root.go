package cmd

import (
	"os"

	"github.com/jasondellaluce/synchro/cmd/conflict"
	"github.com/jasondellaluce/synchro/cmd/explain"
	"github.com/jasondellaluce/synchro/cmd/readme"
	"github.com/jasondellaluce/synchro/cmd/sync"
	"github.com/jasondellaluce/synchro/pkg/utils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	rootVerbose bool
)

func init() {
	rootCmd.PersistentFlags().BoolVar(&rootVerbose, "verbose", false, "if true, turns the logger into more verbose")
	rootCmd.AddCommand(sync.SyncCmd)
	rootCmd.AddCommand(readme.ReadmeCmd)
	rootCmd.AddCommand(explain.ExplainCmd)
	rootCmd.AddCommand(conflict.ConflictCmd)
}

var rootCmd = &cobra.Command{
	Use:          utils.ProjectName,
	Short:        utils.ProjectDescription,
	Version:      utils.ProjectVersion,
	SilenceUsage: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		logrus.SetOutput(os.Stderr)
		if rootVerbose {
			logrus.SetLevel(logrus.DebugLevel)
		} else {
			logrus.SetLevel(logrus.InfoLevel)
		}
	},
}

// Execute will execute the root command
func Execute() error {
	return rootCmd.Execute()
}
