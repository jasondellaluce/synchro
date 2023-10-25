package branchdb

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jasondellaluce/synchro/pkg/utils"
	"github.com/otiai10/copy"
	"github.com/sirupsen/logrus"
)

func Pull(git utils.GitHelper, remote, branch, filePath string, cleanBranch bool) error {
	if err := requireNoLocalChanges(git); err != nil {
		return err
	}

	localBranch := fmt.Sprintf("temp-local-%s-%s", utils.ProjectName, branch)
	return withTempLocalBranch(git, localBranch, remote, branch, func(exists bool) (bool, error) {
		if !exists {
			logrus.Warn("cache branch not existing on remote, nothing to pull")
			return cleanBranch, nil
		}

		logrus.Info("pulling latest changes")
		err := git.Do("pull", remote, branch)
		if err != nil {
			return cleanBranch, err
		}

		logrus.Info("copying file(s) from working directory into destination")
		localRepoFile := filepath.Base(filePath)
		return cleanBranch, copy.Copy(localRepoFile, filePath, copy.Options{
			OnDirExists: func(src, dest string) copy.DirExistsAction {
				// always replace with most up to date file
				return copy.Replace
			},
		})
	})
}

func Push(git utils.GitHelper, remote, branch, filePath string, cleanBranch bool) error {
	if err := requireNoLocalChanges(git); err != nil {
		return err
	}

	localBranch := fmt.Sprintf("temp-local-%s-%s", utils.ProjectName, branch)
	return withTempLocalBranch(git, localBranch, remote, branch, func(exists bool) (bool, error) {
		if _, err := os.Stat(filePath); err != nil {
			logrus.Warnf("file '%s' not found locally, skipping: %s", filePath, err.Error())
			return cleanBranch, nil
		}

		logrus.Info("copying file(s) into work directory")
		localRepoFile := filepath.Base(filePath)
		err := copy.Copy(filePath, localRepoFile, copy.Options{
			OnDirExists: func(src, dest string) copy.DirExistsAction {
				// always replace with most up to date file
				return copy.Replace
			},
		})
		if err != nil {
			return cleanBranch, err
		}

		// check if there are actual updates to push
		hasChanges, err := git.HasLocalChanges(func(s string) bool {
			return strings.Contains(s, localRepoFile)
		})
		if err != nil {
			return cleanBranch, err
		}
		if !hasChanges {
			logrus.Warn("nothing to push due to no changes detected, skipping")
			return cleanBranch, nil
		}

		// cleanup working directory on exit
		defer git.Do("reset", "--hard")

		// stage file changes
		logrus.Info("staging latest changes")
		err = git.Do("add", localRepoFile)
		if err != nil {
			return cleanBranch, nil
		}

		logrus.Info("committing latest changes")
		err = git.Do("commit", "-m", "update: new file changes")
		if err != nil {
			return cleanBranch, nil
		}

		logrus.Info("pushing latest changes")
		return cleanBranch, git.Do("push", remote, localBranch+":"+branch)
	})
}
