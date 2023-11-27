package cmd

import (
	"fmt"

	"github.com/jasondellaluce/synchro/pkg/branchdb"
	"github.com/jasondellaluce/synchro/pkg/utils"
	"github.com/spf13/cobra"
)

const (
	rerereCacheFilePath = "./.git/rr-cache"
)

var (
	conflictRemote               string
	conflictStorageBranch        string
	conflictPreserveTempBranches bool
)

func init() {
	rootCmd.AddCommand(conflictCmd)
	conflictCmd.AddCommand(conflictPullCmd)
	conflictCmd.AddCommand(conflictPushCmd)

	defaultBranch := fmt.Sprintf("%s-rerere-cache", utils.ProjectName)
	conflictCmd.PersistentFlags().StringVarP(&conflictRemote, "remote", "r", "origin", "the remote name of the storage branch")
	conflictCmd.PersistentFlags().StringVarP(&conflictStorageBranch, "branch", "b", defaultBranch, "the name of the storage to be used as storage for the conflicts cache")
	conflictCmd.PersistentFlags().BoolVar(&conflictPreserveTempBranches, "keep-branches", false, "if true, any temporary local branches will not be removed after the execution of a command")
}

var conflictCmd = &cobra.Command{
	Use:   "conflict",
	Short: "Manage the local conflict resolutions cache",
}

var conflictPullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pulls from a branch starage the latest conflict resolution cache updates",
	RunE: func(cmd *cobra.Command, args []string) error {
		return branchdb.Pull(
			utils.NewGitHelper(),
			conflictRemote,
			conflictStorageBranch,
			rerereCacheFilePath,
			!conflictPreserveTempBranches,
		)
	},
}

var conflictPushCmd = &cobra.Command{
	Use:   "push",
	Short: "Pushes into a branch starage the local conflict resolution cache",
	RunE: func(cmd *cobra.Command, args []string) error {
		return branchdb.Push(
			utils.NewGitHelper(),
			conflictRemote,
			conflictStorageBranch,
			rerereCacheFilePath,
			!conflictPreserveTempBranches,
		)
	},
}
