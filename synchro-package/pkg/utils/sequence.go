package utils

import (
	"errors"
)

// ErrSeqBreakout represents the intentional breakout from a sequence
// iteration operation.
var ErrSeqBreakout = errors.New("sequence iteration breakout")

// Sequence represents an iterator for sequences that can be used for loop
// over all the elements available one by one in a stateful manner, up until
// the sequence is completely consumed.
type Sequence[T interface{}] interface {
	//
	// Next returns the next element in the iterator, or nil in case the
	// sequence has no more elements.
	Next() *T
	//
	// Returns a non-nil error in case of failures when creating or using
	// the iterator, or nil otherwise.
	Error() error
}

// NewFilteredSequence returns a new sequence that filters the elements of
// another sequence with a filtering function. The new sequence contains all
// elements for which the filtering function returns true.
func NewFilteredSequence[T interface{}](it Sequence[T], filter func(*T) bool) Sequence[T] {
	return &filteredSequence[T]{
		it:     it,
		filter: filter,
	}
}

type filteredSequence[T interface{}] struct {
	it     Sequence[T]
	filter func(*T) bool
}

func (f *filteredSequence[T]) Error() error {
	return f.it.Error()
}

func (f *filteredSequence[T]) Next() *T {
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

// ConsumeSequence takes a sequence and iterates over all its elements up until
// all are consumed, and invokes a consumption callbacks on them. If the consumer
// function returns a non-nil error, the iterations stops and th error is returned.
// Returns a non-nil error in case of failure, and nil otherwise.
func ConsumeSequence[T interface{}](it Sequence[T], consume func(*T) error) error {
	for v := it.Next(); v != nil; v = it.Next() {
		err := consume(v)
		if err != nil {
			return err
		}
	}
	return it.Error()
}

// CollectSequence takes a sequence and iterates over all its elements up until
// all are consumed, collecting them in a slice. Returns a non-nil error in
// case of failure, and nil otherwise.
func CollectSequence[T interface{}](it Sequence[T]) ([]*T, error) {
	var res []*T
	err := ConsumeSequence(it, func(t *T) error {
		res = append(res, t)
		return nil
	})
	if err != nil && err != ErrSeqBreakout {
		return nil, err
	}
	return res, nil
}
