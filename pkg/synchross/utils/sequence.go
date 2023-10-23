package utils

import (
	"errors"

	"github.com/google/go-github/v56/github"
)

var ErrSeqBreakout = errors.New("sequence iteration breakout")

// SeqIterator represents an iterator for sequences.
type SeqIterator[T interface{}] interface {
	// Next returns the next element in the iterator, or nil in case the
	// iterator has no more elements.
	Next() *T
	//
	// Returns a non-nil error in case of failures during the creation or
	// the iteration of the iterator, or nil otherwise.
	Error() error
}

type githubListFetchFunc[T interface{}] func(*github.ListOptions) ([]*T, *github.Response, error)

type githubSeqIterator[T interface{}] struct {
	fetch   githubListFetchFunc[T]
	options github.ListOptions
	err     error
	batch   []*T
	stop    bool
}

func (g *githubSeqIterator[T]) Error() error {
	return g.err
}

func (g *githubSeqIterator[T]) Next() *T {
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

func NewGithubSeqIterator[T interface{}](fetch githubListFetchFunc[T]) SeqIterator[T] {
	return &githubSeqIterator[T]{
		fetch:   fetch,
		options: github.ListOptions{Page: 1, PerPage: 100},
	}
}

type filteredSeqIterator[T interface{}] struct {
	it     SeqIterator[T]
	filter func(*T) bool
}

func (f *filteredSeqIterator[T]) Error() error {
	return f.it.Error()
}

func (f *filteredSeqIterator[T]) Next() *T {
	for {
		v := f.it.Next()
		if v == nil {
			return nil
		}
		if f.filter(v) {
			return v
		}
	}
}

func NewFilteredSeqIterator[T interface{}](it SeqIterator[T], filter func(*T) bool) SeqIterator[T] {
	return &filteredSeqIterator[T]{
		it:     it,
		filter: filter,
	}
}

func ConsumeSeq[T interface{}](it SeqIterator[T], consume func(*T) error) error {
	for v := it.Next(); v != nil; v = it.Next() {
		err := consume(v)
		if err != nil {
			return err
		}
	}
	return it.Error()
}

func CollectSeq[T interface{}](it SeqIterator[T]) ([]*T, error) {
	var res []*T
	err := ConsumeSeq(it, func(t *T) error {
		res = append(res, t)
		return nil
	})
	if err != nil && err != ErrSeqBreakout {
		return nil, err
	}
	return res, nil
}
