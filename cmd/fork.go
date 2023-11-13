package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/jasondellaluce/synchro/pkg/sync"
	"github.com/jasondellaluce/synchro/pkg/utils"
	"github.com/spf13/cobra"
)

var (
	forkSyncDryRun   bool
	forkSyncBranch   string
	forkHead         string
	forkRepo         string
	forkRepoUpstream string
	forkHeadUpstream string
)

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.Flags().BoolVar(&forkSyncDryRun, "dryrun", false, "preview the sync changes")
	syncCmd.Flags().StringVarP(&forkSyncBranch, "branch", "b", "", "the fork's branch name used for the sync")
	syncCmd.Flags().StringVarP(&forkHead, "head", "c", "", "the head ref of the fork from which commits are scanned")
	syncCmd.Flags().StringVarP(&forkRepo, "repo", "r", "", "the GitHub repository of the fork in the form <org>/<repo>")
	syncCmd.Flags().StringVarP(&forkHeadUpstream, "upstream-head", "C", "", "the head ref of the forked repositoy on which appending the fork's scanned commits")
	syncCmd.Flags().StringVarP(&forkRepoUpstream, "upstream-repo", "R", "", "the forked GitHub repository in the form <org>/<repo>")
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Syncs the fork to a ref from the forked repository by appending all the commits resulting from a fork scan",
	RunE: func(cmd *cobra.Command, args []string) error {
		var err error
		if len(forkRepoUpstream) == 0 {
			err = multierror.Append(fmt.Errorf("must define upstream repository in scan request"), err)
		}
		if len(forkRepo) == 0 {
			err = multierror.Append(fmt.Errorf("must define fork's repository in scan request"), err)
		}
		if len(forkHeadUpstream) == 0 {
			err = multierror.Append(fmt.Errorf("must define upstream head ref in scan request"), err)
		}
		if len(forkHead) == 0 {
			err = multierror.Append(fmt.Errorf("must define fork's head ref in scan request"), err)
		}
		if len(forkSyncBranch) == 0 {
			err = multierror.Append(fmt.Errorf("must define name of the sync branch in fork"), err)
		}
		if err != nil {
			return err
		}

		forkOrg, forkRepoName, err := getOrgRepo(forkRepo)
		if err != nil {
			return err
		}
		upstreamOrg, upstreamRepoName, err := getOrgRepo(forkRepoUpstream)
		if err != nil {
			return err
		}

		ctx := context.Background()
		client := getGithubClient()
		return sync.Sync(
			ctx,
			utils.NewGitHelper(),
			client,
			&sync.Request{
				DryRun:          forkSyncDryRun,
				OutBranch:       forkSyncBranch,
				UpstreamOrg:     upstreamOrg,
				UpstreamRepo:    upstreamRepoName,
				ForkOrg:         forkOrg,
				ForkRepo:        forkRepoName,
				ForkHeadRef:     forkHead,
				UpstreamHeadRef: forkHeadUpstream,
			},
		)
	},
}

func getOrgRepo(s string) (string, string, error) {
	tokens := strings.Split(s, "/")
	if len(tokens) != 2 {
		return "", "", fmt.Errorf("repository must be in the form <org>/<repo>: %s", s)
	}
	return tokens[0], tokens[1], nil
}
