package downstream

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/jasondellaluce/synchro/pkg/downstream"
	"github.com/jasondellaluce/synchro/pkg/utils"
	"github.com/spf13/cobra"
)

var (
	prNumUpstream        uint
	branch               string
	repo                 string
	head                 string
	repoUpstream         string
	headUpstream         string
	searchAfter          string
	preserveTempBranches bool
	noPush               bool
)

func init() {
	DownstreamCmd.Flags().UintVarP(&prNumUpstream, "pr-num", "n", 0, "the upstream GitHub Pull Request number to be downstreamed")
	DownstreamCmd.Flags().StringVarP(&branch, "branch", "b", "", "the fork's output branch used to port the downstreamed commits")
	DownstreamCmd.PersistentFlags().StringVarP(&head, "head", "c", "", "the head ref of the fork from which commits are scanned")
	DownstreamCmd.Flags().StringVarP(&repo, "repo", "r", "", "the fork GitHub repository in the form <org>/<repo>")
	DownstreamCmd.PersistentFlags().StringVarP(&headUpstream, "upstream-head", "C", "", "the head ref of the upstream repositoy on which appending the fork's scanned commits")
	DownstreamCmd.PersistentFlags().StringVarP(&repoUpstream, "upstream-repo", "R", "", "the upstream GitHub repository in the form <org>/<repo>")
	DownstreamCmd.Flags().BoolVar(&preserveTempBranches, "keep-branches", false, "if true, any temporary local branches will not be removed after the execution of a command")
	DownstreamCmd.Flags().BoolVar(&noPush, "no-push", false, "if true, the downstreamed branch will not be pushed and opening a pull request will not be attempted")
	DownstreamCmd.AddCommand(DownstreamSuggestCmd)

	DownstreamSuggestCmd.Flags().StringVar(&searchAfter, "search-after", time.Now().AddDate(0, 0, -7).Format(time.RFC3339), "timestamp after which searching merged pull requests (RFC3339 format)")
}

var DownstreamCmd = &cobra.Command{
	Use:   "downstream",
	Short: "Ports a GitHub Pull Request from an upstream OSS repository to a downstream fork",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := checkPersistenFlags()
		if len(branch) == 0 {
			branch = fmt.Sprintf("%s-downstream-%s-pr-%d", utils.ProjectName, head, prNumUpstream)
		}
		if len(repo) == 0 {
			err = multierror.Append(fmt.Errorf("must define fork repository"), err)
		}
		if prNumUpstream == 0 {
			err = multierror.Append(fmt.Errorf("must define a pull request number to be downstreamed"), err)
		}
		if len(head) == 0 {
			err = multierror.Append(fmt.Errorf("must define fork's head ref"), err)
		}
		if err != nil {
			return err
		}

		upstreamOrg, upstreamRepoName, err := getOrgRepo(repoUpstream)
		if err != nil {
			return err
		}

		forkOrg, forkRepoName, err := getOrgRepo(repo)
		if err != nil {
			return err
		}

		ctx := context.Background()
		git := utils.NewGitHelper()
		client := utils.GetGithubClient()
		return downstream.Downstream(ctx, git, client, &downstream.DownstreamRequest{
			Branch:                 branch,
			UpstreamOrg:            upstreamOrg,
			UpstreamRepo:           upstreamRepoName,
			UpstreamHeadRef:        headUpstream,
			UpstreamPullRequestNum: int(prNumUpstream),
			ForkOrg:                forkOrg,
			ForkRepo:               forkRepoName,
			ForkHeadRef:            head,
			PreserveTempBranches:   preserveTempBranches,
			PushAndOpenPullRequest: !noPush,
		})
	},
}

var DownstreamSuggestCmd = &cobra.Command{
	Use:   "suggest",
	Short: "Suggests a list of GitHub Pull Requests to be downstreamed",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := checkPersistenFlags()
		if err != nil {
			return err
		}

		searchAfterTs, err := time.Parse(time.RFC3339, searchAfter)
		if err != nil {
			return err
		}

		upstreamOrg, upstreamRepoName, err := getOrgRepo(repoUpstream)
		if err != nil {
			return err
		}

		ctx := context.Background()
		git := utils.NewGitHelper()
		client := utils.GetGithubClient()
		return downstream.Suggest(ctx, git, client, &downstream.SuggestRequest{
			UpstreamOrg:     upstreamOrg,
			UpstreamRepo:    upstreamRepoName,
			UpstreamHeadRef: headUpstream,
			ForkHeadRef:     head,
			SearchAfter:     searchAfterTs,
		})
	},
}

func checkPersistenFlags() error {
	var err error
	if len(repoUpstream) == 0 {
		err = multierror.Append(fmt.Errorf("must define upstream repository"), err)
	}
	if len(headUpstream) == 0 {
		err = multierror.Append(fmt.Errorf("must define upstream head ref"), err)
	}
	if len(head) == 0 {
		err = multierror.Append(fmt.Errorf("must define fork's head ref"), err)
	}
	return err
}

func getOrgRepo(s string) (string, string, error) {
	tokens := strings.Split(s, "/")
	if len(tokens) != 2 {
		return "", "", fmt.Errorf("repository must be in the form <org>/<repo>: %s", s)
	}
	return tokens[0], tokens[1], nil
}
