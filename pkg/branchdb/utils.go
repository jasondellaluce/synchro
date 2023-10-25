package branchdb

import (
	"fmt"
	"io/fs"
	"os"

	"github.com/hashicorp/go-multierror"
	"github.com/jasondellaluce/synchro/pkg/utils"
	"github.com/sirupsen/logrus"
)

var readmeContent = fmt.Sprintf(`
# DO NOT EDIT

Generated and automatically managed by [%s](%s)
`, utils.ProjectName, utils.ProjectRepo)

func withTempLocalBranch(git utils.GitHelper, localBranch, remote, remoteBranch string, f func(bool) (bool, error)) error {
	logrus.Infof("moving into local branch '%s'", localBranch)

	// get current branch
	curBranch, err := git.GetCurrentBranch()
	if err != nil {
		return err
	}
	logrus.Debugf("current branch is '%s'", curBranch)

	// check if branch exists in remote already
	exists, err := git.BranchExistsInRemote(remote, remoteBranch)
	if err != nil {
		return err
	}
	if exists {
		logrus.Debugf("branch '%s' already existent in remote '%s'", remoteBranch, remote)
	}

	// we should not already be in the local branch, but if we are then
	// move to the default branch
	if curBranch == localBranch {
		logrus.Debugf("already on the local branch, moving to the default one")
		remoteDefaultBranch, err := git.GetRemoteDefaultBranch(remote)
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

	// delete on exit if necessary
	deleteOnExit := false
	defer func() {
		if deleteOnExit {
			git.Do("branch", "-D", localBranch)
		}
	}()

	// checkout branch from remote if it exists, or create a new orphan one otherwise
	if exists {
		err = git.Do("checkout", "-b", localBranch, fmt.Sprintf("%s/%s", remote, remoteBranch))
	} else {
		err = checkoutLocalOrphanBranch(git, localBranch)
	}
	if err != nil {
		return err
	}

	// get back to original branch on exit
	defer func() { git.Do("checkout", curBranch) }()

	// run callback
	deleteOnExit, err = f(exists)
	return err
}

func checkoutLocalOrphanBranch(git utils.GitHelper, branch string) (err error) {
	// get current branch, just in case
	var curBranch string
	curBranch, err = git.GetCurrentBranch()
	if err != nil {
		return
	}

	// checkout and create orphan branch
	err = git.Do("checkout", "--orphan", branch)
	if err != nil {
		return
	}

	// from this point, in case of failures get back to where we started
	defer func() {
		if err != nil {
			err = multierror.Append(err, git.Do("reset", "--hard"))
			err = multierror.Append(err, git.Do("clean", "-d", "-x", "-f"))
			err = multierror.Append(err, git.Do("checkout", curBranch))
		}
	}()

	// files may be staged by default, unstage them all
	err = git.Do("reset", "--hard")
	if err != nil {
		return
	}

	// remove all files from working directory
	err = git.Do("clean", "-d", "-x", "-f")
	if err != nil {
		return
	}

	// create readme
	err = os.WriteFile("./README.md", []byte(readmeContent), fs.ModePerm)
	if err != nil {
		return
	}

	// add files and commit
	err = git.Do("add", "-A")
	if err != nil {
		return
	}
	return git.Do("commit", "-m", "new: initial commit")
}

func requireNoLocalChanges(git utils.GitHelper) error {
	if localChanges, err := git.HasLocalChanges(); err != nil || localChanges {
		if localChanges {
			err = multierror.Append(err, fmt.Errorf("local changes must be stashed, committed, or removed"))
		}
		return err
	}
	return nil
}
