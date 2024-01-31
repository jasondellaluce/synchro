package sync

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
	syncDryRun       bool
	syncBranch       string
	syncHead         string
	syncRepo         string
	syncRepoUpstream string
	syncHeadUpstream string
)

func init() {
	SyncCmd.Flags().BoolVar(&syncDryRun, "dryrun", false, "preview the sync changes")
	SyncCmd.Flags().StringVarP(&syncBranch, "branch", "b", "", "the fork's synched output branch")
	SyncCmd.Flags().StringVarP(&syncHead, "head", "c", "", "the head ref of the fork from which commits are scanned")
	SyncCmd.Flags().StringVarP(&syncRepo, "repo", "r", "", "the GitHub repository of the fork in the form <org>/<repo>")
	SyncCmd.Flags().StringVarP(&syncHeadUpstream, "upstream-head", "C", "", "the head ref of the upstream repositoy on which appending the fork's scanned commits")
	SyncCmd.Flags().StringVarP(&syncRepoUpstream, "upstream-repo", "R", "", "the upstream GitHub repository in the form <org>/<repo>")
}

var SyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Syncs the fork to an upstream ref by appending all the custom commits",
	RunE: func(cmd *cobra.Command, args []string) error {
		var err error
		if len(syncRepoUpstream) == 0 {
			err = multierror.Append(fmt.Errorf("must define upstream repository in scan request"), err)
		}
		if len(syncRepo) == 0 {
			err = multierror.Append(fmt.Errorf("must define fork's repository in scan request"), err)
		}
		if len(syncHeadUpstream) == 0 {
			err = multierror.Append(fmt.Errorf("must define upstream head ref in scan request"), err)
		}
		if len(syncHead) == 0 {
			err = multierror.Append(fmt.Errorf("must define fork's head ref in scan request"), err)
		}
		if len(syncBranch) == 0 {
			err = multierror.Append(fmt.Errorf("must define name of the sync branch in fork"), err)
		}
		if err != nil {
			return err
		}

		forkOrg, syncRepoName, err := getOrgRepo(syncRepo)
		if err != nil {
			return err
		}
		upstreamOrg, upstreamRepoName, err := getOrgRepo(syncRepoUpstream)
		if err != nil {
			return err
		}

		ctx := context.Background()
		client := utils.GetGithubClient()
		return sync.Sync(
			ctx,
			utils.NewGitHelper(),
			client,
			&sync.Request{
				DryRun:          syncDryRun,
				OutBranch:       syncBranch,
				UpstreamOrg:     upstreamOrg,
				UpstreamRepo:    upstreamRepoName,
				ForkOrg:         forkOrg,
				ForkRepo:        syncRepoName,
				ForkHeadRef:     syncHead,
				UpstreamHeadRef: syncHeadUpstream,
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
