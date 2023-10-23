package scan

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v56/github"
	"github.com/hashicorp/go-multierror"
	"github.com/jasondellaluce/synchross/pkg/synchross/utils"
)

type ScanRequest struct {
	BaseOrg     string
	BaseRepo    string
	ForkOrg     string
	ForkRepo    string
	ForkHeadRef string
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

type CommitInfo struct {
	Commit       *github.RepositoryCommit
	PullRequests []*github.PullRequest
	comments     []*github.RepositoryComment
	commentsRepo string
}

func (c *CommitInfo) Message() string {
	return c.Commit.GetCommit().GetMessage()
}

func (c *CommitInfo) SHA() string {
	return c.Commit.GetSHA()
}

func (c *CommitInfo) AuthorLogin() string {
	return c.Commit.GetAuthor().GetLogin()
}

func (c *CommitInfo) Title() string {
	return strings.Split(c.Message(), "\n")[0]
}

func (c *CommitInfo) PullRequestsOfRepo(org, repo string) []*github.PullRequest {
	var res []*github.PullRequest
	fullName := fmt.Sprintf("%s/%s", org, repo)
	for _, pr := range c.PullRequests {
		if pr.GetBase().GetRepo().GetFullName() == fullName {
			res = append(res, pr)
		}
	}
	return res
}

func (c *CommitInfo) GetComments(ctx context.Context, client *github.Client, org, repo string) ([]*github.RepositoryComment, error) {
	repoName := fmt.Sprintf("%s/%s", org, repo)
	if c.commentsRepo != repoName {
		comments, err := utils.CollectSeq(utils.NewGithubSeqIterator(
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
