package sync

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v56/github"
	"github.com/jasondellaluce/synchro/pkg/utils"
)

// Request contains all the info required for performing a fork scan
type Request struct {
	UpstreamOrg     string
	UpstreamRepo    string
	UpstreamHeadRef string
	ForkOrg         string
	ForkRepo        string
	ForkHeadRef     string
	OutBranch       string
	DryRun          bool
}

// commitInfo contains information about a single commit resulting from a fork
// scan and provides receiver accessors for information about it
type commitInfo struct {
	Commit       *github.RepositoryCommit
	PullRequests []*github.PullRequest
	// internal use
	comments     []*github.RepositoryComment
	commentsRepo string
}

func (c *commitInfo) Message() string {
	return c.Commit.GetCommit().GetMessage()
}

func (c *commitInfo) SHA() string {
	return c.Commit.GetSHA()
}

func (c *commitInfo) ShortSHA() string {
	return c.SHA()[:8]
}

func (c *commitInfo) AuthorLogin() string {
	return c.Commit.GetAuthor().GetLogin()
}

func (c *commitInfo) Title() string {
	return strings.Split(c.Message(), "\n")[0]
}

func (c *commitInfo) pullRequestsOfRepo(org, repo string) []*github.PullRequest {
	var res []*github.PullRequest
	fullName := fmt.Sprintf("%s/%s", org, repo)
	for _, pr := range c.PullRequests {
		if pr.GetBase().GetRepo().GetFullName() == fullName {
			res = append(res, pr)
		}
	}
	return res
}

func (c *commitInfo) getComments(ctx context.Context, client *github.Client, org, repo string) ([]*github.RepositoryComment, error) {
	repoName := fmt.Sprintf("%s/%s", org, repo)
	if c.commentsRepo != repoName {
		comments, err := utils.CollectSequence(utils.NewGithubSequence(
			func(o *github.ListOptions) ([]*github.RepositoryComment, *github.Response, error) {
				return client.Repositories.ListCommitComments(ctx, org, repo, c.SHA(), o)
			}))
		if err != nil {
			return nil, err
		}
		c.comments = comments
		c.commentsRepo = repoName
	}
	return c.comments, nil
}
