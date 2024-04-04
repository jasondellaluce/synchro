package downstream

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
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
			out, err := git.DoOutput("log", "--fixed-strings", "--pretty=format:%h", "--grep", msgLines[0])
			if err != nil {
				return err
			}
			found := strings.Split(out, "\n")
			hasCommit, err := hasCommit(git, req, found, c.GetCommit())
			if err != nil {
				return err
			}
			if hasCommit {
				numFoundCommits++
			}
			numCommits++
		}

		// if less than the 50% of the PR's commit are present in the downstream fork
		// history (checked from the provided head), then we can conclude
		// that the PR is a good candidate to be downstreamed.
		const k float64 = 0.5
		threshold := (int)(math.Ceil(float64(numCommits) * k))
		if numFoundCommits < threshold {
			fmt.Fprintf(os.Stdout, "%d, %s, %s\n", v.GetNumber(), v.GetHTMLURL(), v.GetTitle())
		} else {
			logrus.Warningf("skipping already ported PR %d (%d/%d commits): %s", v.GetNumber(), numFoundCommits, numCommits, v.GetHTMLURL())
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

func hasCommit(git utils.GitHelper, req *SuggestRequest, found []string, c *github.Commit) (bool, error) {
	has_commit := false
	for _, commit := range found {
		has, err := compareDiff(git, req, commit, c.GetURL())
		if err != nil {
			return false, err
		}
		has_commit = has
	}

	return has_commit, nil
}

func compareDiff(git utils.GitHelper, req *SuggestRequest, c string, u string) (bool, error) {
	if len(c) == 0 {
		return false, nil
	}
	remoteDiff, err := pullRemoteDiff(req, u)
	if err != nil {
		return false, err
	}
	localDiff, err := git.DoOutput("show", "--pretty=format:%n", c)
	if err != nil {
		return false, err
	}
	remoteDiffLines := strings.Split(strings.TrimSuffix(remoteDiff, "\n"), "\n")
	localDiffLines := strings.Split(strings.TrimSuffix(localDiff, "\n"), "\n")

	//iterate from back and remove empty lines
	for i := len(remoteDiffLines) - 1; i >= 0; i-- {
		if remoteDiffLines[i] == " " {
			remoteDiffLines = append(remoteDiffLines[:i], remoteDiffLines[i+1:]...)
		}
	}

	for i := len(localDiffLines) - 1; i >= 0; i-- {
		if localDiffLines[i] == " " {
			localDiffLines = append(localDiffLines[:i], localDiffLines[i+1:]...)
		}
	}

	//remote diff sometimes contains an extra blank space as last line
	if remoteDiffLines[len(remoteDiffLines)-1] == " " {
		remoteDiffLines = remoteDiffLines[:len(remoteDiffLines)-1]
	}

	if len(remoteDiffLines) != len(localDiffLines) {
		return false, nil
	}

	for i, line := range remoteDiffLines {
		if line != localDiffLines[i] && !strings.Contains(line, "index") && !strings.Contains(line, "@@") {
			return false, nil
		}
	}

	return true, nil
}

func pullRemoteDiff(req *SuggestRequest, u string) (string, error) {
	hash := strings.Split(u, "/commits/")[1]
	url := "https://github.com/" + req.UpstreamOrg + "/" + req.UpstreamRepo + "/commit/" + hash + ".diff"
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}
