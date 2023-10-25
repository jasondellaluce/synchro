package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/jasondellaluce/synchro/pkg/scan"
	"github.com/jasondellaluce/synchro/pkg/sync"
	"github.com/jasondellaluce/synchro/pkg/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/multierr"
)

var (
	forkHead       string
	forkOrg        string
	forkRepo       string
	forkOrgBase    string
	forkRepoBase   string
	forkHeadBase   string
	forkSyncBranch string
)

func init() {
	rootCmd.AddCommand(forkCmd)
	forkCmd.AddCommand(forkScanCmd)
	forkCmd.AddCommand(forkSyncCmd)

	// todo: we may want to split scan and sync commands in the future, in which case
	// these flags must be splitten and not be persistent in the base-level comamnd
	forkCmd.PersistentFlags().StringVarP(&forkHead, "fork-head", "c", "", "the head ref of the fork from commits are scanned")
	forkCmd.PersistentFlags().StringVarP(&forkOrg, "fork-org", "o", "", "the GitHub organization of the fork")
	forkCmd.PersistentFlags().StringVarP(&forkRepo, "fork-repo", "r", "", "the GitHub repository of the fork")
	forkCmd.PersistentFlags().StringVarP(&forkOrgBase, "base-org", "O", "", "the GitHub organization of the forked repository")
	forkCmd.PersistentFlags().StringVarP(&forkRepoBase, "base-repo", "R", "", "the forked GitHub repository")

	forkSyncCmd.Flags().StringVarP(&forkHeadBase, "base-head", "C", "", "the head ref of the forked repositoy on which appending the fork's scanned commits")
	forkSyncCmd.Flags().StringVarP(&forkSyncBranch, "sync-branch", "b", "", "the fork's branch name used for the sync")
}

var forkCmd = &cobra.Command{
	Use:   "fork",
	Short: "Manage a private fork of an Base repository",
}

var forkScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scans a fork's ref and the OSS and finds the restricted set of commits/patches that are present only in the fork",
	RunE: func(cmd *cobra.Command, args []string) error {
		var err error
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			if len(f.Value.String()) == 0 {
				multierr.Append(err, fmt.Errorf("must define arg '%s'", f.Name))
			}
		})
		if err != nil {
			return err
		}

		client := getGithubClient()
		scanRequest := scan.ScanRequest{
			BaseOrg:     forkOrgBase,
			BaseRepo:    forkRepoBase,
			ForkOrg:     forkOrg,
			ForkRepo:    forkRepo,
			ForkHeadRef: forkHead,
		}
		scan, err := scan.Scan(context.Background(), client, &scanRequest)
		if err != nil {
			return err
		}
		for _, c := range scan {
			// todo: store scan results in a state file
			fmt.Fprintf(os.Stdout, "git cherry-pick %s # %s", c.SHA(), c.Title())
		}
		return nil
	},
}

var forkSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Syncs the fork to a ref from the forked repository by appending all the commits resulting from a fork scan",
	RunE: func(cmd *cobra.Command, args []string) error {
		var err error
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			if len(f.Value.String()) == 0 {
				multierr.Append(err, fmt.Errorf("must define arg '%s'", f.Name))
			}
		})
		if err != nil {
			return err
		}

		// todo: make sync not do the implicit scan -- separate the two steps
		// and write a state file in the .git directory
		ctx := context.Background()
		client := getGithubClient()
		scanRequest := scan.ScanRequest{
			BaseOrg:     forkOrgBase,
			BaseRepo:    forkRepoBase,
			ForkOrg:     forkOrg,
			ForkRepo:    forkRepo,
			ForkHeadRef: forkHead,
		}
		scan, err := scan.Scan(ctx, client, &scanRequest)
		if err != nil {
			return err
		}
		return sync.Sync(
			ctx,
			utils.NewGitHelper(),
			&sync.SyncRequest{
				Scan:        scanRequest,
				ScanRes:     scan,
				BaseHeadRef: forkHeadBase,
				SyncBranch:  forkSyncBranch,
			},
		)
	},
}
