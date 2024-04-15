package downstream

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
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
			hasCommit, err := hasCommit(ctx, git, client, req, found, c.GetCommit())
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

func hasCommit(ctx context.Context, git utils.GitHelper, client *github.Client, req *SuggestRequest, found []string, c *github.Commit) (bool, error) {
	for _, commit := range found {
		has, err := compareDiff(ctx, git, client, req, commit, c.GetURL())
		if err != nil {
			return false, err
		}
		if has {
			return true, nil
		}
	}

	return false, nil
}

func compareDiff(ctx context.Context, git utils.GitHelper, client *github.Client, req *SuggestRequest, c string, u string) (bool, error) {
	if len(c) == 0 {
		return false, nil
	}
	remoteDiff, err := pullRemoteDiff(ctx, client, req, u)
	if err != nil {
		return false, err
	}
	localDiff, err := git.DoOutput("show", "--pretty=format:%n", c)
	if err != nil {
		return false, err
	}
	remoteDiffLines := strings.Split(strings.Trim(remoteDiff, "\n"), "\n")
	localDiffLines := strings.Split(strings.Trim(localDiff, "\n"), "\n")

	remoteDiffLines = sanitizeDiff(remoteDiffLines)
	localDiffLines = sanitizeDiff(localDiffLines)

	if len(remoteDiffLines) != len(localDiffLines) {
		return false, nil
	}

	for i, line := range remoteDiffLines {
		if line != localDiffLines[i] && !strings.HasPrefix(line, "index") && !strings.HasPrefix(line, "@@") {
			return false, nil
		}
	}

	return true, nil
}

func pullRemoteDiff(ctx context.Context, client *github.Client, req *SuggestRequest, u string) (string, error) {
	// do some input checks
	tokens := strings.Split(u, "/commits/")
	if len(tokens) < 2 {
		return "", fmt.Errorf("can't find commit hash in string: %s", u)
	}

	// retrieve commit through GitHub APIs
	hash := tokens[1]
	commit, _, err := client.Repositories.GetCommit(ctx, req.UpstreamOrg, req.UpstreamRepo, hash, nil)
	if err != nil {
		return "", err
	}
	if commit.HTMLURL == nil {
		return "", fmt.Errorf("can't find HTML url for commit: %s", hash)
	}

	// in GitHub, by convention adding ".diff" to the HTML url returns the commit's diff.
	url := commit.GetHTMLURL() + ".diff"

	// perform the get request with the GitHub client to preserve authentication
	resp, err := client.Client().Get(url)
	if err != nil {
		return "", err
	}

	// read commit's diff -- bound read size to 100 MB as we can't trust anyone
	body, err := io.ReadAll(io.LimitReader(resp.Body, 100*1024*1024))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func sanitizeDiff(lines []string) []string {
	for len(lines) > 0 && lines[len(lines)-1] == " " {
		lines = lines[:len(lines)-1]
	}

	return lines
}
