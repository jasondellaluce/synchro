package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testCmdExecutor struct {
	cmd  string
	args []string
	out  string
	err  error
}

func (t *testCmdExecutor) exec(cmd string, args ...string) (string, error) {
	t.cmd = cmd
	t.args = args
	return t.out, t.err
}

func TestGetRemotes(t *testing.T) {
	e := &testCmdExecutor{}
	git := &gitHelper{e: e}

	e.out = `upstream	  git@github.com:upstreamorg/upstreamrepo.git (fetch)
		upstream   git@github.com:upstreamorg/upstreamrepo.git (push)
		origin  git@github.com:forkorg/forkrepo.git (fetch)
		origin  git@github.com:forkorg/forkrepo.git (push)`
	e.err = nil
	remotes, err := git.GetRemotes()
	assert.NoError(t, err)
	assert.Len(t, remotes, 2)
	require.Contains(t, remotes, "upstream")
	require.Contains(t, remotes, "origin")
	require.Equal(t, "git@github.com:upstreamorg/upstreamrepo.git", remotes["upstream"])
	require.Equal(t, "git@github.com:forkorg/forkrepo.git", remotes["origin"])

}
