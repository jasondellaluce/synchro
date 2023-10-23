package scan

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v56/github"
	"github.com/jasondellaluce/synchross/pkg/synchross/utils"
	"github.com/sirupsen/logrus"
)

const IgnoreCommitMarker = "SYNC_IGNORE"

func iteratePullRequestsByCommitSHA(ctx context.Context, client *github.Client, org, repo, sha string) utils.SeqIterator[github.PullRequest] {
	it := utils.NewGithubSeqIterator(
		func(o *github.ListOptions) ([]*github.PullRequest, *github.Response, error) {
			return client.PullRequests.ListPullRequestsWithCommit(ctx, org, repo, sha, o)
		})
	return utils.NewFilteredSeqIterator(it, func(pr *github.PullRequest) bool {
		return pr.MergedAt != nil
	})
}

func iterateCommitsByHead(ctx context.Context, client *github.Client, org, repo, headRef string) utils.SeqIterator[github.RepositoryCommit] {
	return utils.NewGithubSeqIterator(
		func(o *github.ListOptions) ([]*github.RepositoryCommit, *github.Response, error) {
			return client.Repositories.ListCommits(ctx, org, repo, &github.CommitsListOptions{
				SHA:         headRef,
				ListOptions: *o,
			})
		})
}

func commitLinksAreAmbiguos(links []*utils.PullRequestLink) bool {
	if len(links) > 1 {
		for i := 1; i < len(links); i++ {
			if links[i].Num != links[0].Num {
				return true
			}
		}
	}
	return false
}

// returns the pull request number relative to the base repo
func searchForkCommitLink(ctx context.Context, client *github.Client, req *ScanRequest, c *CommitInfo) (int, error) {
	// search in pull request body
	for _, pr := range c.PullRequestsOfRepo(req.ForkOrg, req.ForkRepo) {
		links, err := utils.SearchPullRequestLinks(req.BaseOrg, req.BaseRepo, pr.GetBody())
		if err != nil {
			return 0, err
		}
		if commitLinksAreAmbiguos(links) {
			url := fmt.Sprintf("https://github.com/%s/%s/pull/%d", req.ForkOrg, req.ForkRepo, pr.GetNumber())
			return 0, fmt.Errorf("pull requests body contains multiple base repo links and may be ambiguous: %s", url)
		}
		if len(links) > 0 {
			logrus.Infof("found link in pull request body #%d", pr.GetNumber())
			return links[0].Num, nil
		}
	}

	// search in commit message
	links, err := utils.SearchPullRequestLinks(req.BaseOrg, req.BaseRepo, c.Message())
	if err != nil {
		return 0, err
	}
	if commitLinksAreAmbiguos(links) {
		url := fmt.Sprintf("https://github.com/%s/%s/commit/%s)", req.ForkOrg, req.ForkRepo, c.SHA())
		return 0, fmt.Errorf("commit message contains multiple base repo links and may be ambiguous: %s", url)
	}
	if len(links) > 0 {
		logrus.Infof("found link in commit message of %s", c.SHA())
		return links[0].Num, nil
	}

	// search in commit comments
	comments, err := c.GetComments(ctx, client, req.ForkOrg, req.ForkRepo)
	if err != nil {
		return 0, err
	}
	for _, comment := range comments {
		links, err := utils.SearchPullRequestLinks(req.BaseOrg, req.BaseRepo, comment.GetBody())
		if err != nil {
			return 0, err
		}
		if commitLinksAreAmbiguos(links) {
			url := fmt.Sprintf("https://github.com/%s/%s/commit/%s)", req.ForkOrg, req.ForkRepo, c.SHA())
			return 0, fmt.Errorf("commit comment contains multiple base repo links and may be ambiguous: %s", url)
		}
		if len(links) > 0 {
			logrus.Infof("found link in one comment body of %s", c.SHA())
			return links[0].Num, nil
		}
	}

	return 0, nil
}

func checkCommitShouldBeIgnored(ctx context.Context, client *github.Client, req *ScanRequest, c *CommitInfo) (bool, error) {
	comments, err := c.GetComments(ctx, client, req.ForkOrg, req.ForkRepo)
	if err != nil {
		return false, err
	}
	for _, comment := range comments {
		if strings.Contains(comment.GetBody(), IgnoreCommitMarker) {
			return true, nil
		}
	}
	return false, nil
}

func scanRepoCommit(ctx context.Context, client *github.Client, req *ScanRequest, c *github.RepositoryCommit) (*CommitInfo, error) {
	res := &CommitInfo{Commit: c}
	logrus.Infof("analyzing commit %s %s", res.SHA(), res.Title())

	logrus.Debugf("listing pull requests in fork repository %s/%s", req.ForkOrg, req.ForkRepo)
	pulls, err := utils.CollectSeq(iteratePullRequestsByCommitSHA(ctx, client, req.ForkOrg, req.ForkRepo, res.SHA()))
	if err != nil {
		return nil, err
	}
	res.PullRequests = pulls

	logrus.Debugf("listing pull requests in base repository %s/%s", req.BaseOrg, req.BaseRepo)
	pulls, err = utils.CollectSeq(iteratePullRequestsByCommitSHA(ctx, client, req.BaseOrg, req.BaseRepo, res.SHA()))
	if err != nil {
		logrus.Debugf("commit probably not found in base repo, purposely ignoring error: %s", err.Error())
	} else {
		res.PullRequests = append(res.PullRequests, pulls...)
	}

	link, err := searchForkCommitLink(ctx, client, req, res)
	if err != nil {
		return nil, err
	}
	if link != 0 {
		logrus.Debugf("checking linked pull request %s/%s#%d", req.BaseOrg, req.BaseRepo, link)
		pr, _, err := client.PullRequests.Get(ctx, req.BaseOrg, req.BaseRepo, link)
		if err != nil {
			return nil, err
		}

		if pr.MergedAt != nil {
			logrus.Infof("linked pull request is MERGED, skipping commit")
			return nil, nil
		} else if strings.ToLower(pr.GetState()) == "closed" {
			logrus.Infof("linked pull request is CLOSED, picking commit")
		} else {
			logrus.Infof("linked pull request probably still OPEN or DRAFT, picking commit")
		}
	} else {
		logrus.Info("no link to base repository found for commit")
	}

	logrus.Debugf("commit is being picked, checking if we should ignore it")
	ignore, err := checkCommitShouldBeIgnored(ctx, client, req, res)
	if err != nil {
		return nil, err
	}
	if ignore {
		logrus.Infof("deteted ignore marker %s, skipping commit", IgnoreCommitMarker)
		return nil, nil
	}

	if link == 0 && len(res.PullRequests) == 0 {
		logrus.Warn("no metadata found for picked commit")
	}

	return res, nil
}

func Scan(ctx context.Context, client *github.Client, req *ScanRequest) ([]*CommitInfo, error) {
	logrus.Infof("initiating fork scan for repository %s/%s with base %s/%s", req.ForkOrg, req.ForkRepo, req.BaseOrg, req.BaseRepo)
	err := req.Error()
	if err != nil {
		return nil, err
	}

	// iterate through the commits of the fork
	var result []*CommitInfo
	err = utils.ConsumeSeq(iterateCommitsByHead(ctx, client, req.ForkOrg, req.ForkRepo, req.ForkHeadRef),
		func(c *github.RepositoryCommit) error {
			info, err := scanRepoCommit(ctx, client, req, c)
			if err == nil {
				if info != nil {
					basePRs := info.PullRequestsOfRepo(req.BaseOrg, req.BaseRepo)
					if len(info.PullRequests) == 1 && len(basePRs) == 1 && basePRs[0].MergedAt != nil {
						logrus.Infof("commit is only part of a base repo PR, stopping")
						return utils.ErrSeqBreakout
					}
					result = append(result, info)
				}
			}
			return err
		})
	if err != nil && err != utils.ErrSeqBreakout {
		return nil, err
	}
	utils.ReverseSlice(result)
	return result, nil
}
