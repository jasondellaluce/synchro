package sync

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/go-github/v56/github"
	"github.com/jasondellaluce/synchro/pkg/utils"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// Scan analyzes both the upstream and the fork repositories specified in the given
// scan request, and returns a list of commit info representing the restricted
// set of commits that are present in the fork exclusively in the form of
// private patches. Returns a non-nil error in case of failure.
func scan(ctx context.Context, client *github.Client, req *Request) ([]*commitInfo, error) {
	logrus.Infof("initiating fork scan for repository %s/%s with upstream %s/%s", req.ForkOrg, req.ForkRepo, req.UpstreamOrg, req.UpstreamRepo)
	defer logrus.Infof("finished fork scan for repository %s/%s with upstream %s/%s", req.ForkOrg, req.ForkRepo, req.UpstreamOrg, req.UpstreamRepo)

	// iterate through the commits of the fork
	var result []*commitInfo
	err := utils.ConsumeSequence(iterateCommitsByHead(ctx, client, req.ForkOrg, req.ForkRepo, req.ForkHeadRef),
		func(c *github.RepositoryCommit) error {
			info, err := scanRepoCommit(ctx, client, req, c)
			if err == nil {
				if info != nil {
					upstreamPRs := info.pullRequestsOfRepo(req.UpstreamOrg, req.UpstreamRepo)
					if len(info.PullRequests) == 1 && len(upstreamPRs) == 1 && upstreamPRs[0].MergedAt != nil {
						logrus.Debugf("commit is only part of a upstream repo PR, stopping")
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

// performs the scan process for the given commit
func scanRepoCommit(ctx context.Context, client *github.Client, req *Request, c *github.RepositoryCommit) (*commitInfo, error) {
	res := &commitInfo{Commit: c}
	logrus.Infof("scanning commit %s %s", res.SHA(), res.Title())

	var forkPRs []*github.PullRequest
	var upstreamPRs []*github.PullRequest
	var comments []*github.RepositoryComment
	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		logrus.Debugf("listing pull requests in fork repository %s/%s", req.ForkOrg, req.ForkRepo)
		var err error
		forkPRs, err = utils.CollectSequence(iteratePullRequestsByCommitSHA(gCtx, client, req.ForkOrg, req.ForkRepo, res.SHA()))
		return err
	})

	g.Go(func() error {
		logrus.Debugf("listing pull requests in upstream repository %s/%s", req.UpstreamOrg, req.UpstreamRepo)
		var err error
		upstreamPRs, err = utils.CollectSequence(iteratePullRequestsByCommitSHA(gCtx, client, req.UpstreamOrg, req.UpstreamRepo, res.SHA()))
		if err != nil {
			logrus.Debugf("commit probably not found in upstream repo, purposely ignoring error: %s", err.Error())
			return nil
		}
		return nil
	})

	g.Go(func() error {
		var err error
		comments, err = res.getComments(gCtx, client, req.ForkOrg, req.ForkRepo)
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	res.PullRequests = append(forkPRs, upstreamPRs...)

	ref, err := searchForkCommitRef(ctx, client, req, res, comments)
	if err != nil {
		return nil, err
	}
	if ref != 0 {
		logrus.Debugf("checking ref pull request %s/%s#%d", req.UpstreamOrg, req.UpstreamRepo, ref)
		pr, _, err := client.PullRequests.Get(ctx, req.UpstreamOrg, req.UpstreamRepo, ref)
		if err != nil {
			return nil, err
		}

		if pr.MergedAt != nil {
			mergedInUpstreamHead, err := isPullRequestMergedInUpstreamHead(ctx, client, req, pr)
			if err != nil {
				return nil, err
			}
			if mergedInUpstreamHead {
				logrus.Infof("refed pull request is MERGED in upstream head %s, skipping commit", req.UpstreamHeadRef)
				return nil, nil
			}
			logrus.Infof("refed pull request is MERGED, but not in upstream head %s, picking commit", req.UpstreamHeadRef)
		} else if strings.ToLower(pr.GetState()) == "closed" {
			logrus.Infof("refed pull request is CLOSED, picking commit")
		} else {
			logrus.Infof("refed pull request probably still OPEN or DRAFT, picking commit")
		}
	} else {
		logrus.Info("no ref to upstream repository found for commit")
	}

	logrus.Debugf("commit is being picked, checking if we should ignore it")
	err = searchCommitMarkers(ctx, client, req, res, comments)
	if err != nil {
		return nil, err
	}
	if res.HasMarker(CommitMarkerIgnore) {
		logrus.Infof("deteted ignore marker %s, skipping commit", CommitMarkerIgnore)
		return nil, nil
	}

	if ref == 0 && len(res.PullRequests) == 0 {
		logrus.Warn("no metadata found for picked commit")
	}

	return res, nil
}

// returns a sequence containing all pull requests containing a given commit
// SHA for a specific repository.
func iteratePullRequestsByCommitSHA(ctx context.Context, client *github.Client, org, repo, sha string) utils.Sequence[github.PullRequest] {
	it := utils.NewGithubSequence(
		func(o *github.ListOptions) ([]*github.PullRequest, *github.Response, error) {
			return client.PullRequests.ListPullRequestsWithCommit(ctx, org, repo, sha, o)
		})
	return utils.NewFilteredSequence(it, func(pr *github.PullRequest) bool {
		return pr.MergedAt != nil
	})
}

// returns a sequence containing all commits for a specific repository, starting
// from the given head ref and proceeding from the most to the least recent.
func iterateCommitsByHead(ctx context.Context, client *github.Client, org, repo, headRef string) utils.Sequence[github.RepositoryCommit] {
	return utils.NewGithubSequence(
		func(o *github.ListOptions) ([]*github.RepositoryCommit, *github.Response, error) {
			return client.Repositories.ListCommits(ctx, org, repo, &github.CommitsListOptions{
				SHA:         headRef,
				ListOptions: *o,
			})
		})
}

// returns true if the list of references found for a given commit is ambiguous
// with regards to the scanning process.
func commitRefsAreAmbiguos(refs []int) bool {
	if len(refs) > 1 {
		for i := 1; i < len(refs); i++ {
			if refs[i] != refs[0] {
				return true
			}
		}
	}
	return false
}

// searches inside a text for pull request references of the given org and repo.
// Returns a list of non-zero numbers representing the pull request numbers
// found in the references. Returns a non-nil error in case of failure.
func searchPullRequestRefs(org, repo, text string) ([]int, error) {
	var res []int

	var pullRequestRefInTextStyles = []*regexp.Regexp{
		regexp.MustCompile(fmt.Sprintf(`%s/%s#(\d+)`, org, repo)),
		regexp.MustCompile(fmt.Sprintf(`github.com/%s/%s/pull/(\d+)`, org, repo)),
		regexp.MustCompile(fmt.Sprintf(`\[%s#(\d+)\]`, org)),
	}

	for _, s := range pullRequestRefInTextStyles {
		matches := s.FindAllStringSubmatch(text, -1)
		for _, m := range matches {
			if len(m) == 2 {
				num, err := strconv.Atoi(m[1])
				if err != nil {
					return nil, err
				}
				res = append(res, num)
			}
		}
	}

	return res, nil
}

// returns the pull request number relative to the upstream repo
func searchForkCommitRef(ctx context.Context, client *github.Client, req *Request, c *commitInfo, comments []*github.RepositoryComment) (int, error) {
	// search in pull request body
	for _, pr := range c.pullRequestsOfRepo(req.ForkOrg, req.ForkRepo) {
		refs, err := searchPullRequestRefs(req.UpstreamOrg, req.UpstreamRepo, pr.GetBody())
		if err != nil {
			return 0, err
		}
		if len(refs) > 0 {
			if commitRefsAreAmbiguos(refs) {
				url := fmt.Sprintf("https://github.com/%s/%s/pull/%d", req.ForkOrg, req.ForkRepo, pr.GetNumber())
				logrus.Warnf("pull requests body contains multiple upstream repo refs and may be ambiguous: %s", url)
			}
			logrus.Infof("found ref in pull request body #%d", pr.GetNumber())
			return refs[0], nil
		}
	}

	// search in commit message
	// note(mrgian): we don't need to search in commit messages for now
	/*refs, err := searchPullRequestRefs(req.UpstreamOrg, req.UpstreamRepo, c.Message())
	if err != nil {
		return 0, err
	}
	if len(refs) > 0 {
		if commitRefsAreAmbiguos(refs) {
			url := fmt.Sprintf("https://github.com/%s/%s/commit/%s", req.ForkOrg, req.ForkRepo, c.SHA())
			logrus.Warnf("commit message contains multiple upstream repo refs and may be ambiguous: %s", url)
		}
		logrus.Infof("found ref in commit message of %s", c.SHA())
		return refs[0], nil
	}*/

	// search in commit comments
	for _, comment := range comments {
		refs, err := searchPullRequestRefs(req.UpstreamOrg, req.UpstreamRepo, comment.GetBody())
		if err != nil {
			return 0, err
		}
		if len(refs) > 0 {
			if commitRefsAreAmbiguos(refs) {
				url := fmt.Sprintf("https://github.com/%s/%s/commit/%s", req.ForkOrg, req.ForkRepo, c.SHA())
				logrus.Warnf("commit comment contains multiple upstream repo refs and may be ambiguous: %s", url)
			}
			logrus.Infof("found ref in one comment body of %s", c.SHA())
			return refs[0], nil
		}
	}

	return 0, nil
}

// returns true if the commit should be ignored for the given scan request
func searchCommitMarkers(ctx context.Context, client *github.Client, req *Request, c *commitInfo, comments []*github.RepositoryComment) error {
	c.Markers = make(map[string]bool)

	// search in commit's message
	for _, m := range AllCommitMarkers {
		if strings.Contains(c.Message(), m.String()) {
			c.Markers[m.String()] = true
		}
	}

	// search in commit's comments
	for _, comment := range comments {
		for _, m := range AllCommitMarkers {
			if strings.Contains(comment.GetBody(), m.String()) {
				c.Markers[m.String()] = true
			}
		}
	}
	return nil
}

// returns true if the merge commit of a PR is included in the configured
// upstream head ref.
func isPullRequestMergedInUpstreamHead(ctx context.Context, client *github.Client, req *Request, pr *github.PullRequest) (bool, error) {
	mergeCommitSHA := pr.GetMergeCommitSHA()
	if mergeCommitSHA == "" {
		logrus.Warnf("missing merge commit sha for merged pull request #%d", pr.GetNumber())
		return false, nil
	}

	cmp, _, err := client.Repositories.CompareCommits(ctx, req.UpstreamOrg, req.UpstreamRepo, mergeCommitSHA, req.UpstreamHeadRef, nil)
	if err != nil {
		return false, err
	}

	// In GitHub compare API semantics:
	// - "ahead": head includes base plus additional commits
	// - "identical": head equals base
	// Both cases mean the merge commit is included in upstream head.
	status := cmp.GetStatus()
	return status == "ahead" || status == "identical", nil
}
