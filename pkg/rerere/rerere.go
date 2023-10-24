package rerere

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/jasondellaluce/synchro/pkg/utils"
	"github.com/ldez/go-git-cmd-wrapper/v2/branch"
	"github.com/ldez/go-git-cmd-wrapper/v2/checkout"
	"github.com/ldez/go-git-cmd-wrapper/v2/commit"
	"github.com/ldez/go-git-cmd-wrapper/v2/git"
	"github.com/ldez/go-git-cmd-wrapper/v2/revparse"
	"github.com/ldez/go-git-cmd-wrapper/v2/types"
	"github.com/otiai10/copy"
	"github.com/sirupsen/logrus"
)

const branchCacheDir = "./rr-cache"

const gitCacheDir = "./.git/rr-cache"

// Pulls in the local git repo the `git rerere` cache from a remote branch.
func Pull(remote, branch string) error {
	logrus.Info("checking out rerere cache branch")
	localBranch := fmt.Sprintf("tmp-%s-rerere-cache", utils.ProjectName)
	return withTempLocalBranch(localBranch, remote, branch, func(exists bool) error {
		if !exists {
			logrus.Info("cache branch not existing on remote, nothing to pull")
			return nil
		}

		logrus.Info("pulling latest changes")
		err := doGit(logrus.DebugLevel, func() (string, error) {
			return git.Raw("pull", simpleArg(remote), simpleArg(branch))
		})
		if err != nil {
			return err
		}

		logrus.Info("copying pulled rerere cache into .git directory")
		opt := copy.Options{
			OnDirExists: func(src, dest string) copy.DirExistsAction {
				return copy.Replace
			},
		}
		return copy.Copy(branchCacheDir, gitCacheDir, opt)
	})
}

// Pushes in the local git repo the `git rerere` cache from a remote branch.
func Push(remote, branch string) error {
	// todo: prevent this from happening with unclean working tree
	logrus.Info("checking out rerere cache branch")
	localBranch := fmt.Sprintf("tmp-%s-rerere-cache", utils.ProjectName)
	return withTempLocalBranch(localBranch, remote, branch, func(exists bool) error {
		logrus.Info("copying pulled rerere cache from .git directory")
		opt := copy.Options{
			OnDirExists: func(src, dest string) copy.DirExistsAction {
				return copy.Replace
			},
		}
		if _, err := os.Stat(gitCacheDir); err != nil {
			logrus.Warnf("no %s directory found locally, skipping push", gitCacheDir)
			return nil
		}

		err := copy.Copy(gitCacheDir, branchCacheDir, opt)
		if err != nil {
			return err
		}

		hasChanges, err := hasLocalChanges()
		if err != nil {
			return err
		}
		if hasChanges {
			defer func() {
				// TODO: this does not work properly yet
				logrus.Debug("cleaning up working tree")
				git.Raw("reset", simpleArg("--hard"))
			}()

			logrus.Debug("staging new changes")
			out, err := git.Raw("add", simpleArg(branchCacheDir))
			if err != nil {
				return multierror.Append(err, fmt.Errorf(out))
			}
			logrus.Info("committing new changes")
			out, err = git.Commit(commit.Message("update: add new rerere cache entries"))
			if err != nil {
				return multierror.Append(err, fmt.Errorf(out))
			}
			logrus.Info("pushing latest changes")
			return doGit(logrus.DebugLevel, func() (string, error) {
				return git.Raw("push", simpleArg(remote), simpleArg(localBranch+":"+branch))
			})
		} else {
			logrus.Warn("no changes detected, skipping push")
			return nil
		}
	})
}

func withTempLocalBranch(localBranch, remote, remoteBranch string, f func(bool) error) error {
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

	// if branch exists on remote, check it out locally. If not, create
	// a local branch and we'll push it later
	exists := false
	err = doGit(logrus.InfoLevel, func() (string, error) {
		exists, err = branchExistsInRemote(remote, remoteBranch)
		if err != nil {
			return "", err
		}
		if exists {
			// todo: checkout only the directory
			ref := fmt.Sprintf("%s/%s", remote, remoteBranch)
			return git.Checkout(checkout.NewBranch(localBranch), checkout.Branch(ref))
		} else {
			return "", checkoutLocalOrphanBranch(localBranch)
		}
	})
	if err != nil {
		return err
	}

	// delete on exit
	// todo: make this optional with --cleanup-branch maybe?
	// defer delete()

	// get back to original branch on exit
	defer func() {
		logrus.Debugf("git checkout %s", curBranch)
		doGit(logrus.DebugLevel, func() (string, error) {
			return git.Checkout(checkout.Branch(curBranch))
		})
	}()

	return f(exists)
}

func doGit(level logrus.Level, f func() (string, error)) error {
	out, err := f()
	if len(out) > 0 {
		logrus.StandardLogger().Log(level, strings.TrimSpace(out))
	}
	return err
}

func hasLocalChanges() (bool, error) {
	logrus.Debugf("git status --porcelain")
	out, err := git.Raw("status", simpleArg("--porcelain"))
	if err != nil {
		return false, err
	}
	return len(strings.TrimSpace(out)) != 0, nil
}

// note: how do I prevent files from being added?
func checkoutLocalOrphanBranch(branch string) error {
	_, err := git.Checkout(checkout.Orphan(branch))
	if err != nil {
		return err
	}
	_, err = git.Raw("reset")
	if err != nil {
		return err
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	err = removeAll(wd)
	if err != nil {
		return err
	}
	_, err = git.Raw("add", simpleArg("-A"))
	if err != nil {
		return err
	}
	_, err = git.Commit(commit.AllowEmpty, commit.Message("initial commit"))
	return err
}

func branchExistsInRemote(remote, branch string) (bool, error) {
	ref := fmt.Sprintf("refs/heads/%s", branch)
	out, err := git.Raw("ls-remote", simpleArg("--heads"), simpleArg(remote), simpleArg(ref))
	if err != nil {
		return false, err
	}
	return len(strings.TrimSpace(out)) != 0, nil
}

func simpleArg(args ...string) func(*types.Cmd) {
	return func(g *types.Cmd) {
		for _, arg := range args {
			g.AddOptions(arg)
		}
	}
}

func removeAll(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && path != dir && !strings.Contains(path, "/.git") && !strings.Contains(path, "/build") {
			if !info.IsDir() {
				return os.RemoveAll(path)
			}
		}
		return err
	})
}
