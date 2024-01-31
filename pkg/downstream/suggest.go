package downstream

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v56/github"
	"github.com/jasondellaluce/synchro/pkg/utils"
	"github.com/sirupsen/logrus"
)

type SuggestRequest struct {
	UpstreamOrg     string
	UpstreamRepo    string
	UpstreamHeadRef string
	ForkHeadRef     string
	SearchAfter     time.Time
}

func Suggest(ctx context.Context, git utils.GitHelper, client *github.Client, req *SuggestRequest) error {
	// get current branch
	curBranch, err := git.GetCurrentBranch()
	if err != nil {
		return err
	}
	logrus.Debugf("current branch is '%s'", curBranch)

	// moving to head if necessary, and get back once we're done
	if curBranch != req.ForkHeadRef {
		err = git.Do("checkout", req.ForkHeadRef)
		if err != nil {
			return err
		}
		defer func() { git.Do("checkout", curBranch) }()
	}

	errStop := errors.New("stop")
	pulls := iterateMergedPullRequests(ctx, client, req.UpstreamOrg, req.UpstreamRepo, req.UpstreamHeadRef)
	err = utils.ConsumeSequence(pulls, func(v *github.PullRequest) error {
		logrus.Debugf("checking pull request %d merged at %s: %s", v.GetNumber(), v.GetMergedAt().String(), v.GetHTMLURL())

		// make sure we respect the time bounds
		lastUpdateTime := v.MergedAt
		if lastUpdateTime != nil && lastUpdateTime.GetTime().Before(req.SearchAfter) {
			logrus.Infof("found pull request updated before search limit, stopping search: updated=%s, limit=%s", lastUpdateTime.String(), req.SearchAfter.String())
			return errStop
		}

		// retrieve PR's commits
		commits, err := utils.CollectSequence(iteratePullRequestCommits(ctx, client, req.UpstreamOrg, req.UpstreamRepo, v.GetNumber()))
		if err != nil {
			return err
		}

		// search in local history for the PR commits
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
			fmt.Fprintf(os.Stdout, "%d, %s, %s\n", v.GetNumber(), v.GetHTMLURL(), v.GetTitle())
		}

		return nil
	})

	if err != nil && err != errStop {
		return err
	}

	return nil
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
