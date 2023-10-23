package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSearchPullRequestLinks(t *testing.T) {
	const org = "falcosecurity"
	const repo = "libs"

	t.Run("no-ref", func(t *testing.T) {
		const text = "Some text with no ref"
		refs, err := SearchPullRequestLinks(org, repo, text)
		assert.NoError(t, err)
		assert.Empty(t, refs)
	})
	t.Run("one-match", func(t *testing.T) {
		const text = "Some text and a\nref: falcosecurity/libs#1234"
		refs, err := SearchPullRequestLinks(org, repo, text)
		assert.NoError(t, err)
		assert.Len(t, refs, 1)
		assert.Equal(t, 1234, refs[0].Num)
		assert.Equal(t, "falcosecurity/libs#1234", refs[0].Ref)
	})
	t.Run("multi-match", func(t *testing.T) {
		const text = `Some text and 3 refs:
			falcosecurity/libs#123
			github.com/falcosecurity/libs/pull/456
			[falcosecurity#789]`
		refs, err := SearchPullRequestLinks(org, repo, text)
		assert.NoError(t, err)
		assert.Len(t, refs, 3)
		assert.Equal(t, 123, refs[0].Num)
		assert.Equal(t, "falcosecurity/libs#123", refs[0].Ref)
		assert.Equal(t, 456, refs[1].Num)
		assert.Equal(t, "github.com/falcosecurity/libs/pull/456", refs[1].Ref)
		assert.Equal(t, 789, refs[2].Num)
		assert.Equal(t, "[falcosecurity#789]", refs[2].Ref)
	})
	t.Run("wrong-repo", func(t *testing.T) {
		const text = "Some text and a\nref: falcosecurity/falco#1234"
		refs, err := SearchPullRequestLinks(org, repo, text)
		assert.NoError(t, err)
		assert.Empty(t, refs)
	})
}
