package cmd

import (
	"os"

	"github.com/jasondellaluce/synchro/pkg/utils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	rootVerbose bool
)

func init() {
	rootCmd.PersistentFlags().BoolVar(&rootVerbose, "verbose", false, "if true, turns the logger into more verbose")
}

var rootCmd = &cobra.Command{
	Use:          utils.ProjectName,
	Short:        utils.ProjectDescription,
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
