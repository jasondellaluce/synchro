package utils

import "github.com/google/go-github/v56/github"

// GithubClientListFunc is a generic functional wrapper for "list"-type API
// invocations of a GitHub client for which the list options are provided.
type GithubClientListFunc[T interface{}] func(*github.ListOptions) ([]*T, *github.Response, error)

// NewGithubSequence creates a new sequence starting from a GithubClientListFunc
func NewGithubSequence[T interface{}](f GithubClientListFunc[T]) Sequence[T] {
	return &githubSequence[T]{
		fetch:   f,
		options: github.ListOptions{Page: 1, PerPage: 100},
	}
}

type githubSequence[T interface{}] struct {
	fetch   GithubClientListFunc[T]
	options github.ListOptions
	err     error
	batch   []*T
	stop    bool
}

func (g *githubSequence[T]) Error() error {
	return g.err
}

func (g *githubSequence[T]) Next() *T {
	if g.err != nil {
		return nil
	}
	if len(g.batch) == 0 && !g.stop {
		g.batch, _, g.err = g.fetch(&g.options)
		if g.err != nil {
			return nil
		}
		g.options.Page++
		if len(g.batch) < g.options.PerPage {
			g.stop = true
		}
	}
	if len(g.batch) == 0 {
		return nil
	}
	res := g.batch[0]
	g.batch = g.batch[1:]
	return res
}
