package downstream

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/go-github/v56/github"
	"github.com/hashicorp/go-multierror"
	"github.com/jasondellaluce/synchro/pkg/utils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	branch        string
	head          string
	repoUpstream  string
	headUpstream  string
	prNumUpstream uint
)

func init() {
	DownstreamCmd.Flags().UintVarP(&prNumUpstream, "pr-num", "n", 0, "the upstream GitHub Pull Request number to be downstreamed")
	DownstreamCmd.Flags().StringVarP(&branch, "branch", "b", "", "the fork's output branch used to port the downstreamed commits")
	DownstreamCmd.PersistentFlags().StringVarP(&head, "head", "c", "", "the head ref of the fork from which commits are scanned")
	DownstreamCmd.PersistentFlags().StringVarP(&headUpstream, "upstream-head", "C", "", "the head ref of the upstream repositoy on which appending the fork's scanned commits")
	DownstreamCmd.PersistentFlags().StringVarP(&repoUpstream, "upstream-repo", "R", "", "the upstream GitHub repository in the form <org>/<repo>")
	DownstreamCmd.AddCommand(DownstreamSuggestCmd)
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

		ctx := context.Background()
		git := utils.NewGitHelper()
		client := utils.GetGithubClient()

		upstreamOrg, upstreamRepoName, err := getOrgRepo(repoUpstream)
		if err != nil {
			return err
		}

		// get current branch
		curBranch, err := git.GetCurrentBranch()
		if err != nil {
			return err
		}
		logrus.Debugf("current branch is '%s'", curBranch)
		if curBranch != head {
			// moving to head, and get back once we're done
			err = git.Do("checkout", head)
			if err != nil {
				return err
			}
			defer func() { git.Do("checkout", curBranch) }()
		}

		pulls := iterateMergedPullRequests(ctx, client, upstreamOrg, upstreamRepoName, headUpstream)
		return utils.ConsumeSequence(pulls, func(v *github.PullRequest) error {
			//logrus.Infof("checking pull request %d merged at %s: %s", v.GetNumber(), v.GetMergedAt().String(), v.GetHTMLURL())
			commits, err := utils.CollectSequence(iteratePullRequestCommits(ctx, client, upstreamOrg, upstreamRepoName, v.GetNumber()))
			if err != nil {
				return err
			}

			numCommits := 0
			numFoundCommits := 0
			for _, c := range commits {
				msgLines := strings.Split(c.GetCommit().GetMessage(), "\n")
				if len(msgLines) == 0 {
					return fmt.Errorf("found commit with empty body: %s", c.GetSHA())
				}
				out, err := git.DoOutput("log", "--fixed-strings", "--grep", msgLines[0])
				if err != nil {
					return err
				}
				if len(out) > 0 {
					numFoundCommits++
				}
				numCommits++
			}

			// it may happen that a PR has been partially downstreamed in
			// the fork and some of its commits have been discarded. In such
			// a case, we'll not suggest it as a dowstream candidate but we'll
			// log it for sake of transparency
			if numFoundCommits > 0 && numFoundCommits < numCommits {
				logrus.Warningf("pull request %d has been partially ported (%d/%d commits): %s", v.GetNumber(), numFoundCommits, numCommits, v.GetHTMLURL())
			}

			// if none of the PR's commit are present in the downstream fork
			// history (checked from the provided head), then we can conclude
			// that the PR is a good candidate to be downstreamed.
			if numFoundCommits == 0 {
				fmt.Printf("%d, %s, %s\n", v.GetNumber(), v.GetHTMLURL(), v.GetTitle())
			}
			return nil
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

func iterateMergedPullRequests(ctx context.Context, client *github.Client, org, repo, base string) utils.Sequence[github.PullRequest] {
	it := utils.NewGithubSequence(
		func(o *github.ListOptions) ([]*github.PullRequest, *github.Response, error) {
			return client.PullRequests.List(ctx, org, repo, &github.PullRequestListOptions{
				ListOptions: *o,
				Base:        base,
				State:       "closed",
				Sort:        "updated",
				Direction:   "desc",
			})
		})
	return utils.NewFilteredSequence(it, func(pr *github.PullRequest) bool {
		return (pr.Merged != nil && pr.GetMerged()) || pr.MergedAt != nil
	})
}

func iteratePullRequestCommits(ctx context.Context, client *github.Client, org, repo string, prNum int) utils.Sequence[github.RepositoryCommit] {
	return utils.NewGithubSequence(
		func(o *github.ListOptions) ([]*github.RepositoryCommit, *github.Response, error) {
			return client.PullRequests.ListCommits(ctx, org, repo, prNum, o)
		})
}
