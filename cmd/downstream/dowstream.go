package downstream

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/jasondellaluce/synchro/pkg/downstream"
	"github.com/jasondellaluce/synchro/pkg/utils"
	"github.com/spf13/cobra"
)

var (
	branch        string
	head          string
	repoUpstream  string
	headUpstream  string
	prNumUpstream uint
	searchAfter   string
)

func init() {
	DownstreamCmd.Flags().UintVarP(&prNumUpstream, "pr-num", "n", 0, "the upstream GitHub Pull Request number to be downstreamed")
	DownstreamCmd.Flags().StringVarP(&branch, "branch", "b", "", "the fork's output branch used to port the downstreamed commits")
	DownstreamCmd.PersistentFlags().StringVarP(&head, "head", "c", "", "the head ref of the fork from which commits are scanned")
	DownstreamCmd.PersistentFlags().StringVarP(&headUpstream, "upstream-head", "C", "", "the head ref of the upstream repositoy on which appending the fork's scanned commits")
	DownstreamCmd.PersistentFlags().StringVarP(&repoUpstream, "upstream-repo", "R", "", "the upstream GitHub repository in the form <org>/<repo>")
	DownstreamCmd.AddCommand(DownstreamSuggestCmd)

	DownstreamSuggestCmd.Flags().StringVar(&searchAfter, "search-after", time.Now().AddDate(0, 0, -7).Format(time.RFC3339), "timestamp after which searching merged pull requests (RFC3339 format)")
}

var DownstreamCmd = &cobra.Command{
	Use:   "downstream",
	Short: "Ports a GitHub Pull Request from an upstream OSS repository to a downstream fork",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := checkPersistenFlags()
		if len(branch) == 0 {
			err = multierror.Append(fmt.Errorf("must define name of the sync branch in fork"), err)
		}
		if err != nil {
			return err
		}

		// upstreamOrg, upstreamRepoName, err := getOrgRepo(repoUpstream)
		// if err != nil {
		// 	return err
		// }

		// todo: implement this
		return errors.New("downstream feature not available yet")
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
		err = multierror.Append(fmt.Errorf("must define upstream repository in scan request"), err)
	}
	if len(headUpstream) == 0 {
		err = multierror.Append(fmt.Errorf("must define upstream head ref in scan request"), err)
	}
	if len(head) == 0 {
		err = multierror.Append(fmt.Errorf("must define fork's head ref in scan request"), err)
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
