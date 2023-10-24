package sync

import (
	"fmt"
	"strings"

	"github.com/ldez/go-git-cmd-wrapper/v2/branch"
	"github.com/ldez/go-git-cmd-wrapper/v2/checkout"
	"github.com/ldez/go-git-cmd-wrapper/v2/fetch"
	"github.com/ldez/go-git-cmd-wrapper/v2/git"
	"github.com/ldez/go-git-cmd-wrapper/v2/remote"
	"github.com/ldez/go-git-cmd-wrapper/v2/revparse"
	"github.com/ldez/go-git-cmd-wrapper/v2/types"
	"github.com/sirupsen/logrus"
)

// todo: factorize all git commands in an interface so that we can have a dry run

func doGit(level logrus.Level, f func() (string, error)) error {
	out, err := f()
	if len(out) > 0 {
		logrus.StandardLogger().Log(level, strings.TrimSpace(out))
	}
	return err
}

func withTempGitRemote(name, url string, f func() error) error {
	// remove remote if it exists already
	remove := func() {
		logrus.Debugf("git remote remove %s", name)
		err := doGit(logrus.DebugLevel, func() (string, error) {
			return git.Remote(remote.Remove(name))
		})
		if err != nil {
			logrus.Debugf("purposely ignoring error: %s", err)
		}
	}
	remove()

	// add remote
	logrus.Debugf("git remote add %s %s", name, url)
	err := doGit(logrus.DebugLevel, func() (string, error) {
		return git.Remote(remote.Add(name, url))
	})
	if err != nil {
		return err
	}

	// remove on exit
	defer remove()

	// fetch from remote
	err = doGit(logrus.DebugLevel, func() (string, error) {
		logrus.Debugf("git fetch --tags %s", name)
		return git.Fetch(fetch.Tags, fetch.Remote(name))
	})
	if err != nil {
		return err
	}

	// prune on exit
	defer doGit(logrus.DebugLevel, func() (string, error) {
		logrus.Debugf("git fetch --prune %s", name)
		return git.Fetch(fetch.Prune, fetch.Remote(name))
	})

	return f()
}

func withTempLocalBranch(localBranch, remoteBranch string, f func() error) error {
	logrus.Info("saving current branch")
	logrus.Debug("git rev-parse --abbrev-ref HEAD")
	curBranch, err := git.RevParse(revparse.AbbrevRef(""), revparse.Args("HEAD"))
	if err != nil {
		return err
	}
	curBranch = strings.TrimSpace(curBranch)
	if len(curBranch) == 0 {
		return fmt.Errorf("can't retrieved current branch in local repository")
	}
	logrus.Debugf("current branch is: %s", curBranch)

	// delete local branch if it exists already
	delete := func() {
		logrus.Debugf("git branch -D %s", localBranch)
		err := doGit(logrus.DebugLevel, func() (string, error) {
			return git.Branch(branch.DeleteForce, branch.BranchName(localBranch))
		})
		if err != nil {
			logrus.Debugf("purposely ignoring error: %s", err)
		}
	}
	delete()

	// checkout remote branch into local one
	err = doGit(logrus.InfoLevel, func() (string, error) {
		return git.Checkout(checkout.NewBranch(localBranch), checkout.Branch(remoteBranch))
	})
	if err != nil {
		return err
	}

	// delete on exit
	//defer delete()

	// get back to original branch on exit
	defer func() {
		logrus.Debugf("git checkout %s", curBranch)
		doGit(logrus.DebugLevel, func() (string, error) {
			return git.Checkout(checkout.Branch(curBranch))
		})
	}()

	return f()
}

func simpleArg(args ...string) func(*types.Cmd) {
	return func(g *types.Cmd) {
		for _, arg := range args {
			g.AddOptions(arg)
		}
	}
}
