package sync

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/jasondellaluce/synchro/pkg/utils"
	"github.com/sirupsen/logrus"
)

func requireNoLocalChanges(git utils.GitHelper) error {
	if localChanges, err := git.HasLocalChanges(); err != nil || localChanges {
		if localChanges {
			err = multierror.Append(err, fmt.Errorf("local changes must be stashed, committed, or removed"))
		}
		return err
	}
	return nil
}

func withTempGitRemote(git utils.GitHelper, remote, url string, f func() error) error {
	logrus.Infof("adding temporary git remote for '%s'", url)

	// remove remote if it exists already
	git.Do("remote", "remove", remote)

	// add remote
	err := git.Do("remote", "add", remote, url)
	if err != nil {
		return err
	}

	// remove on exit
	defer git.Do("remote", "remove", remote)

	// prune on exit
	defer git.Do("fetch", "--prune", remote)

	// fetch all from remote, tags included
	err = git.Do("fetch", "--tags", remote)
	if err != nil {
		return err
	}

	// invoke callback
	return f()
}

func withTempLocalBranch(git utils.GitHelper, localBranch, remote, remoteBranch string, f func() error) error {
	// note: the remote ref may be a tag, so we need to check it first
	isTag, err := git.TagExists(remoteBranch)
	if err != nil {
		return err
	}
	remoteRef := remoteBranch
	if !isTag {
		remoteRef = fmt.Sprintf("%s/%s", remote, remoteBranch)
	}

	logrus.Infof("moving into local branch '%s' tracking '%s'", localBranch, remoteRef)

	// get current branch
	curBranch, err := git.GetCurrentBranch()
	if err != nil {
		return err
	}
	logrus.Debugf("current branch is '%s'", curBranch)

	// we should not already be in the temporary branch, but if we are then
	// move to the default branch
	if curBranch == localBranch {
		logrus.Debugf("already on the local branch, moving to the default one")
		remoteDefaultBranch, err := git.GetRemoteDefaultBranch("origin")
		if err != nil {
			return err
		}
		err = git.Do("checkout", remoteDefaultBranch)
		if err != nil {
			return err
		}
		curBranch = remoteDefaultBranch
	}

	// remove local branch if it exists
	logrus.Debugf("deleting local branch '%s' in case it exists", localBranch)
	git.Do("branch", "-D", localBranch)

	// checkout remote branch into local one
	err = git.Do("checkout", "-b", localBranch, remoteRef)
	if err != nil {
		return err
	}

	// get back to original branch on exit
	defer func() { git.Do("checkout", curBranch) }()

	// run callback
	return f()
}
