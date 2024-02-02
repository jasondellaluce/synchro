package utils

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

func WithTempGitRemote(git GitHelper, remote, url string, f func() error) error {
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

func WithTempLocalBranch(git GitHelper, localBranch, remote, remoteBranch string, f func() (bool, error)) error {
	// note: the remote ref is generally a branch, but we need to check it first
	isBranch, err := git.BranchExistsInRemote(remote, remoteBranch)
	if err != nil {
		return err
	}
	remoteRef := remoteBranch
	if isBranch {
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

	// delete on exit if necessary
	deleteOnExit := false
	defer func() {
		if deleteOnExit {
			git.Do("branch", "-D", localBranch)
		}
	}()

	// get back to original branch on exit
	defer func() { git.Do("checkout", curBranch) }()

	// run callback
	deleteOnExit, err = f()
	return err
}
