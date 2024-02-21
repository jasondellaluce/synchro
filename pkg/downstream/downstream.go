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
)

type DownstreamRequest struct {
	Branch                 string
	UpstreamOrg            string
	UpstreamRepo           string
	UpstreamHeadRef        string
	UpstreamPullRequestNum int
	ForkOrg                string
	ForkRepo               string
	ForkHeadRef            string
	PreserveTempBranches   bool
	PushAndOpenPullRequest bool
}

func Downstream(ctx context.Context, git utils.GitHelper, client *github.Client, req *DownstreamRequest) error {
	// check that the current repo is the actual fork and the tool
	// is not erroneously run from the wrong repo
	logrus.Infof("checking that the current repo is the fork one")
	remotes, err := git.GetRemotes()
	if err != nil {
		return err
	}
	if len(remotes) == 0 {
		return fmt.Errorf("can't find any remotes in current repo")
	}
	if originRemote, ok := remotes["origin"]; !ok {
		return fmt.Errorf("can't find `origin` remote in current repo")
	} else if !strings.Contains(originRemote, fmt.Sprintf("%s/%s", req.ForkOrg, req.ForkRepo)) {
		return fmt.Errorf("current repo `origin` remote does not match the fork's one: %s", originRemote)
	}

	logrus.Infof("retrieving pull request #%d from %s/%s\n", req.UpstreamPullRequestNum, req.UpstreamOrg, req.UpstreamRepo)
	pr, _, err := client.PullRequests.Get(ctx, req.UpstreamOrg, req.UpstreamRepo, req.UpstreamPullRequestNum)
	if err != nil {
		return err
	}

	if (pr.Merged == nil || !pr.GetMerged()) || (pr.MergedAt == nil) {
		// todo: support downstreaming unmerged PRs
		logrus.Warnf("unmerged pull requests are currently not supported for downstreaming, skipping")
		return nil
	}

	var commitTitles []string
	commits, err := utils.CollectSequence(iteratePullRequestCommits(ctx, client, req.UpstreamOrg, req.UpstreamRepo, req.UpstreamPullRequestNum))
	if err != nil {
		return err
	}
	for _, c := range commits {
		msgLines := strings.Split(c.GetCommit().GetMessage(), "\n")
		if len(msgLines) == 0 {
			return fmt.Errorf("found commit with empty body: %s", c.GetSHA())
		}
		logrus.Infof("found commit: %s", msgLines[0])
		commitTitles = append(commitTitles, msgLines[0])
	}

	logrus.Infof("adding temporary remote for upstream %s/%s", req.UpstreamOrg, req.UpstreamRepo)
	remoteName := fmt.Sprintf("temp-%s-upstream-%s-%s", utils.ProjectName, req.UpstreamOrg, req.UpstreamRepo)
	remoteURL := fmt.Sprintf("https://github.com/%s/%s", req.UpstreamOrg, req.UpstreamRepo)
	return utils.WithTempGitRemote(git, remoteName, remoteURL, func() error {
		// search for hashes of all PR's commit
		// note: in case a PR is merged, the commit hashes will always differ
		// from the ones of the PR, which could report the commits from a given
		// branch and event from another fork
		logrus.Infof("searching for all pull request commits")
		var commitHashes []string
		upstreamTmpDefaultBranch := fmt.Sprintf("temp-%s-upstream-default-%s-%s", utils.ProjectName, req.UpstreamOrg, req.UpstreamRepo)
		err := utils.WithTempLocalBranch(git, upstreamTmpDefaultBranch, remoteName, req.UpstreamHeadRef, func() (bool, error) {
			for _, title := range commitTitles {
				out, err := git.DoOutput("log", "--oneline", "--abbrev=64", "--fixed-strings", "--grep", title)
				if err != nil {
					return !req.PreserveTempBranches, err
				}
				if len(out) == 0 {
					err = fmt.Errorf("could not find upstream commit with title: %s", title)
					logrus.Error(err.Error())
					return !req.PreserveTempBranches, err
				}
				// commit hash is the first space-separated token
				tokens := strings.Split(out, " ")
				if len(tokens) == 0 {
					err = fmt.Errorf("found corrupted upstream commit hash: title=%s, hash=%s", title, out)
					logrus.Error(err.Error())
					return !req.PreserveTempBranches, err
				}
				logrus.Infof("found hash %s for commit: %s", tokens[0], title)
				commitHashes = append(commitHashes, tokens[0])
			}
			return !req.PreserveTempBranches, nil
		})
		if err != nil {
			return err
		}

		// now it's time to create a temporary branch starting from the fork's
		// head ref and start cherry-picking all the commits found
		logrus.Infof("picking for all pull request commits in temporary branch")
		downstreamOutputBranch := req.Branch
		return utils.WithTempLocalBranch(git, downstreamOutputBranch, "origin", req.ForkHeadRef, func() (bool, error) {
			for _, hash := range commitHashes {
				logrus.Infof("picking commit %s", hash)
				out, err := git.DoOutput("cherry-pick", "--allow-empty", hash)
				if err != nil {
					logrus.Error("unrecoverable merge conflict occurred, reverting patch")
					errOut := errors.New(out)
					return !req.PreserveTempBranches, multierror.Append(err, errOut, git.Do("reset", "--hard"))
				}
			}
			if req.PushAndOpenPullRequest {
				return !req.PreserveTempBranches, pushAndOpenPullRequest(ctx, git, client, req, downstreamOutputBranch, pr.GetTitle())
			}
			return !req.PreserveTempBranches, nil
		})
	})

}

func pushAndOpenPullRequest(ctx context.Context, git utils.GitHelper, client *github.Client, req *DownstreamRequest, branch, prTitle string) error {
	// we expect to be in the temp branch containing all the picked commits
	curBranch, err := git.GetCurrentBranch()
	if err != nil {
		return err
	}
	if curBranch != branch {
		return fmt.Errorf("expected to be in '%s' branch, but currently in '%s'", branch, curBranch)
	}

	// checking if there's a diff or if there are no changes
	diff, err := git.DoOutput("diff", fmt.Sprintf("HEAD..origin/%s", req.ForkHeadRef))
	if err != nil {
		return err
	}
	if len(diff) == 0 {
		logrus.Warnf("found an empty diff, nothing to push, skipping")
		return nil
	}

	logrus.Infof("checking if a pull request has already been opened for the same changes")
	skip := false
	titlePrefix := fmt.Sprintf("downstream(#%d): ", req.UpstreamPullRequestNum)
	searchFilter := fmt.Sprintf("type:pr repo:\"%s/%s\" \"%s\"", req.ForkOrg, req.ForkRepo, titlePrefix)
	searchRes, _, err := client.Search.Issues(ctx, searchFilter, &github.SearchOptions{})
	if err != nil {
		return err
	}
	logrus.Infof("search found %d results", searchRes.GetTotal())
	if searchRes.GetTotal() > 0 {
		for _, issue := range searchRes.Issues {
			logrus.Debugf("checking search result %s", issue.GetHTMLURL())
			if issue.IsPullRequest() && strings.HasPrefix(issue.GetTitle(), titlePrefix) {
				logrus.Warnf("found existing pull request downstreaming same changes: %s", issue.GetHTMLURL())
				skip = true
			}
		}
	}
	if skip {
		logrus.Infof("skipping opening pull request")
		return nil
	}

	// push branch on fork
	logrus.Infof("pushing branch '%s' into %s/%s", branch, req.ForkOrg, req.ForkRepo)
	err = git.Do("push", "-f", "origin", branch)
	if err != nil {
		logrus.Errorf("failure in pushing branch into fork: %s", branch)
		return err
	}

	logrus.Infof("opening new pull request in %s/%s", req.ForkOrg, req.ForkRepo)
	pullRequestTitle := fmt.Sprintf("%s%s", titlePrefix, prTitle)
	pullRequestBody := fmt.Sprintf("Ref: https://github.com/%s/%s/pull/%d", req.UpstreamOrg, req.UpstreamRepo, req.UpstreamPullRequestNum)
	pr, _, err := client.PullRequests.Create(ctx, req.ForkOrg, req.ForkRepo, &github.NewPullRequest{
		Title: &pullRequestTitle,
		Head:  &branch,
		Base:  &req.ForkHeadRef,
		Body:  &pullRequestBody,
	})
	if err != nil {
		logrus.Errorf("failure in opening pull request: %s", err.Error())
		return err
	}

	logrus.Infof("pull request opened successfully: %s", pr.GetHTMLURL())
	return nil
}
