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
	rerereRemote               string
	rerereStorageBranch        string
	rererePreserveTempBranches bool
)

func init() {
	rootCmd.AddCommand(rerereCmd)
	rerereCmd.AddCommand(rererePullCmd)
	rerereCmd.AddCommand(rererePushCmd)

	defaultBranch := fmt.Sprintf("%s-rerere-cache", utils.ProjectName)
	rerereCmd.PersistentFlags().StringVarP(&rerereRemote, "remote", "r", "origin", "the remote name of the storage branch")
	rerereCmd.PersistentFlags().StringVarP(&rerereStorageBranch, "branch", "b", defaultBranch, "the name of the storage to be used as storage for the rerere cache")
	rerereCmd.PersistentFlags().BoolVar(&rererePreserveTempBranches, "keep-branches", false, "if true, any temporary local branches will not be removed after the execution of a command")
}

var rerereCmd = &cobra.Command{
	Use:   "rerere",
	Short: "Manage the local `git rerere` resolutions cache",
}

var rererePullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pulls from a branch starage the latest `git rerere` cache updates",
	RunE: func(cmd *cobra.Command, args []string) error {
		return branchdb.Pull(
			utils.NewGitHelper(),
			rerereRemote,
			rerereStorageBranch,
			rerereCacheFilePath,
			!rererePreserveTempBranches,
		)
	},
}

var rererePushCmd = &cobra.Command{
	Use:   "push",
	Short: "Pushes into a branch starage the local `git rerere` cache",
	RunE: func(cmd *cobra.Command, args []string) error {
		return branchdb.Push(
			utils.NewGitHelper(),
			rerereRemote,
			rerereStorageBranch,
			rerereCacheFilePath,
			!rererePreserveTempBranches,
		)
	},
}
