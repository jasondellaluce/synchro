package scan

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v56/github"
	"github.com/hashicorp/go-multierror"
	"github.com/jasondellaluce/synchross/pkg/synchross/utils"
	"github.com/sirupsen/logrus"
)

const IgnoreCommitMarker = "SYNC_IGNORE"

type ScanRequest struct {
	BaseOrg     string
	BaseRepo    string
	ForkOrg     string
	ForkRepo    string
	ForkHeadRef string
}

type ScanResult struct {
	Title string
	Body  string
	SHA   string
}

func (s *ScanRequest) Error() error {
	var err error
	if len(s.BaseOrg) == 0 {
		err = multierror.Append(fmt.Errorf("must define base organization in scan request"), err)
	}
	if len(s.BaseRepo) == 0 {
		err = multierror.Append(fmt.Errorf("must define base repository in scan request"), err)
	}
	if len(s.ForkOrg) == 0 {
		err = multierror.Append(fmt.Errorf("must define fork's organization in scan request"), err)
	}
	if len(s.ForkRepo) == 0 {
		err = multierror.Append(fmt.Errorf("must define fork's repository in scan request"), err)
	}
	if len(s.ForkHeadRef) == 0 {
		err = multierror.Append(fmt.Errorf("must define fork's head ref in scan request"), err)
	}
	return err
}

func (s *ScanRequest) String() string {
	return fmt.Sprintf("base=%s/%s fork=%s/%s", s.BaseOrg, s.BaseRepo, s.ForkOrg, s.ForkRepo)
}

type tagList []string

func listRepoTags(ctx context.Context, client *github.Client, org, repo string) (map[string]tagList, error) {
	res := make(map[string]tagList)
	tags, err := utils.IterateGithubPages(
		func(o *github.ListOptions) ([]*github.RepositoryTag, *github.Response, error) {
			return client.Repositories.ListTags(ctx, org, repo, o)
		})
	if err != nil {
		return nil, err
	}
	for _, t := range tags {
		sha := t.GetCommit().GetSHA()
		if len(sha) > 0 && len(t.GetName()) > 0 {
			res[sha] = append(res[sha], t.GetName())
		} else {
			logrus.Warnf("found empty tag name or sha for repository %s/%s", org, repo)
		}
	}
	return res, nil
}

func listRepoPullRequestsByCommitSHA(ctx context.Context, client *github.Client, org, repo, sha string) ([]*github.PullRequest, error) {
	prs, err := utils.IterateGithubPages(
		func(o *github.ListOptions) ([]*github.PullRequest, *github.Response, error) {
			return client.PullRequests.ListPullRequestsWithCommit(ctx, org, repo, sha, o)
		})
	if err != nil {
		return nil, err
	}
	var res []*github.PullRequest
	for _, pr := range prs {
		if pr.GetMerged() {
			res = append(res, pr)
		}
	}
	return res, nil
}

func formatCommit(c *github.RepositoryCommit, prs []*github.PullRequest, tags map[string]tagList) string {
	var prRefs []string
	for _, pr := range prs {
		prRefs = append(prRefs, fmt.Sprintf("%s#%d", pr.GetBase().GetRepo().GetFullName(), pr.GetNumber()))
	}

	var tagRefs []string
	for _, tag := range tags[c.GetSHA()] {
		tagRefs = append(tagRefs, tag)
	}
	return fmt.Sprintf("%s, %s, pulls=(%s), tags=(%s)", c.GetSHA(), c.GetAuthor().GetLogin(), strings.Join(prRefs, ", "), strings.Join(tagRefs, ", "))
}

func findCommitLinksInFork(ctx context.Context, client *github.Client, req *ScanRequest, c *github.RepositoryCommit, prs []*github.PullRequest) (*utils.PullRequestLink, error) {
	forkFullName := fmt.Sprintf("%s/%s", req.ForkOrg, req.ForkRepo)
	for _, pr := range prs {
		if pr.GetBase().GetRepo().GetFullName() == forkFullName {
			links, err := utils.SearchPullRequestLinks(req.BaseOrg, req.BaseRepo, pr.GetBody())
			if err != nil {
				return nil, err
			}
			if len(links) > 0 {
				// todo: support multiple refs for each PR body
				return links[0], nil
			}

			// todo: also support searching inPR comments?
		}
	}

	links, err := utils.SearchPullRequestLinks(req.BaseOrg, req.BaseRepo, c.GetCommit().GetMessage())
	if err != nil {
		return nil, err
	}
	if len(links) > 0 {
		// todo: support multiple refs for each PR body
		return links[0], nil
	}

	commitComments, err := utils.IterateGithubPages(
		func(o *github.ListOptions) ([]*github.RepositoryComment, *github.Response, error) {
			return client.Repositories.ListCommitComments(ctx, req.ForkOrg, req.ForkRepo, c.GetSHA(), o)
		})
	if err != nil {
		return nil, err
	}
	for _, comment := range commitComments {
		links, err := utils.SearchPullRequestLinks(req.BaseOrg, req.BaseRepo, comment.GetBody())
		if err != nil {
			return nil, err
		}
		if len(links) > 0 {
			// todo: support multiple refs for each PR body
			return links[0], nil
		}
	}

	return nil, nil
}

func checkCommitShouldBeIgnored(ctx context.Context, client *github.Client, req *ScanRequest, c *github.RepositoryCommit) (bool, error) {
	commitComments, err := utils.IterateGithubPages(
		func(o *github.ListOptions) ([]*github.RepositoryComment, *github.Response, error) {
			return client.Repositories.ListCommitComments(ctx, req.ForkOrg, req.ForkRepo, c.GetSHA(), o)
		})
	if err != nil {
		return false, err
	}
	for _, comment := range commitComments {
		// todo: check PR comment to maybe?
		if strings.Contains(comment.GetBody(), IgnoreCommitMarker) {
			return true, nil
		}
	}
	return false, nil
}

func Scan(ctx context.Context, client *github.Client, req *ScanRequest) ([]*ScanResult, error) {
	logrus.Info("starting new scan")
	err := req.Error()
	if err != nil {
		return nil, err
	}
	logrus.Info(req.String())

	// get all tags of base repo
	logrus.Debugf("listing tags for repository %s/%s", req.BaseOrg, req.BaseRepo)
	baseTags, err := listRepoTags(ctx, client, req.BaseOrg, req.BaseRepo)
	if err != nil {
		return nil, err
	}

	// get fork commits
	logrus.Debugf("listing commits for repository %s/%s", req.ForkOrg, req.ForkRepo)
	forkCommits, err := utils.IterateGithubPages(
		func(o *github.ListOptions) ([]*github.RepositoryCommit, *github.Response, error) {
			return client.Repositories.ListCommits(ctx, req.ForkOrg, req.ForkRepo, &github.CommitsListOptions{
				SHA:         req.ForkHeadRef,
				ListOptions: *o,
			})
		})
	if err != nil {
		return nil, err
	}

	// iterate through fork's commits
	var cherryPicks []*ScanResult
	for _, c := range forkCommits {
		// this logic is based on the assumption that everything that goes into falcosecurity/libs gets
		// in through a PR (incl. on every release branch) - this is currently a requirement and documented
		// in the process
		// I couldn't figure out a way to understand if a commit is only present on a fork and not in the
		// original repo, i.e. the API equivalent of the yellow box that appears for example here:
		// https://github.com/falcosecurity/libs/commit/d6af955bce6533fc470082a3b863de7793c90440
		logrus.Infof("analyzing commit %s", c.GetSHA())

		// get pull requests with the commit
		logrus.Debugf("listing pull requests for repository %s/%s", req.ForkOrg, req.ForkRepo)
		pullRequests, err := listRepoPullRequestsByCommitSHA(ctx, client, req.ForkOrg, req.ForkRepo, c.GetSHA())
		if err != nil {
			return nil, err
		}
		logrus.Debugf("listing pull requests for repository %s/%s", req.BaseOrg, req.BaseRepo)
		basePrs, err := listRepoPullRequestsByCommitSHA(ctx, client, req.BaseOrg, req.BaseRepo, c.GetSHA())
		if err != nil {
			logrus.Info("purposely ignoring error")
		}
		pullRequests = append(pullRequests, basePrs...)

		hasPRs := len(pullRequests) > 0
		hasBaseTags := len(baseTags[c.GetSHA()]) > 0

		// sumarrize commit info
		logrus.Info(formatCommit(c, pullRequests, baseTags))

		// search links in body of PRs
		logrus.Debugf("searching links to commit in repository %s/%s", req.ForkOrg, req.ForkRepo)
		link, err := findCommitLinksInFork(ctx, client, req, c, pullRequests)
		if err != nil {
			return nil, err
		}

		// we have at least one link to an OSS pull request for this commit
		pickCommit := true
		hasLink := link != nil
		if hasLink {
			logrus.Debugf("checking linked pull request %s/%s#%d", req.BaseOrg, req.BaseRepo, link.Num)
			pr, _, err := client.PullRequests.Get(ctx, req.BaseOrg, req.BaseRepo, link.Num)
			if err != nil {
				return nil, err
			}

			if pr.GetMerged() {
				logrus.Infof("linked pull request is MERGED, skipping commit")
				pickCommit = false
			} else if strings.ToLower(pr.GetState()) == "closed" {
				logrus.Infof("linked pull request is CLOSED, picking commit")
			} else if strings.ToLower(pr.GetState()) == "closed" {
				logrus.Infof("linked pull request probably still OPEN or DRAFT, picking commit")
			}
		} else {
			logrus.Warnf("no link found")
		}

		// last few checks before picking the commit
		if pickCommit {
			logrus.Debugf("commit is being picked, checking if we should ignore it")
			ignore, err := checkCommitShouldBeIgnored(ctx, client, req, c)
			if err != nil {
				return nil, err
			}
			if ignore {
				pickCommit = false
				logrus.Infof("deteted ignore marker %s, skipping commit", IgnoreCommitMarker)
			}

			if pickCommit && !hasLink && !hasBaseTags && !hasPRs {
				logrus.Warn("no metadata found for picked commit")
				// todo: implement in-depth analysis with GH search
			}
		}

		baseRepoName := fmt.Sprintf("%s/%s", req.BaseOrg, req.BaseRepo)
		if len(pullRequests) == 1 && pullRequests[0].GetBase().GetRepo().GetFullName() == baseRepoName && pullRequests[0].GetMerged() {
			logrus.Debugf("commit is only part of a base repo PR, stopping")
			break
		}

		if pickCommit {
			cherryPicks = append(cherryPicks, &ScanResult{
				SHA:   c.GetSHA(),
				Title: strings.Split(c.GetCommit().GetMessage(), "\n")[0],
				Body:  c.GetCommit().GetMessage(),
			})
		}
	}

	utils.ReverseSlice(cherryPicks)
	return cherryPicks, nil
}
