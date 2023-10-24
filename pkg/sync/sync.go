package sync

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/jasondellaluce/synchro/pkg/scan"
	"github.com/jasondellaluce/synchro/pkg/utils"
	"github.com/ldez/go-git-cmd-wrapper/v2/git"
	"github.com/ldez/go-git-cmd-wrapper/v2/reset"
	"github.com/sirupsen/logrus"
)

type SyncRequest struct {
	Scan        scan.ScanRequest
	ScanRes     []*scan.CommitInfo
	BaseHeadRef string
	SyncBranch  string
}

func Sync(ctx context.Context, req *SyncRequest) error {
	// todo: check that origin remote is the actual repo of fork in req request
	// todo: make "origin" settable
	logrus.Infof("initiating fork sync for repository %s/%s with base %s/%s", req.Scan.ForkOrg, req.Scan.ForkRepo, req.Scan.BaseOrg, req.Scan.BaseRepo)
	defer logrus.Infof("finished fork sync for repository %s/%s with base %s/%s", req.Scan.ForkOrg, req.Scan.ForkRepo, req.Scan.BaseOrg, req.Scan.BaseRepo)

	remoteName := fmt.Sprintf("tmp-%s-sync-base-remote", utils.ProjectName)
	remoteURL := fmt.Sprintf("https://github.com/%s/%s", req.Scan.BaseOrg, req.Scan.BaseRepo)
	logrus.Info("setting up temporary remote for sync")
	return withTempGitRemote(remoteName, remoteURL, func() error {
		localBranchName := fmt.Sprintf("tmp-%s-sync-base-remote", utils.ProjectName)
		remoteBaseBranchName := fmt.Sprintf("%s/%s", remoteName, req.BaseHeadRef)
		return withTempLocalBranch(localBranchName, remoteBaseBranchName, func() error {
			// we're now at the HEAD of the branch in the base repository, in
			// our local copy. Let's proceed cherry-picking all the patches.
			return syncAllPatches(ctx, req)
		})
	})
}

func syncAllPatches(ctx context.Context, req *SyncRequest) error {
	// todo: state file and recover
	for _, c := range req.ScanRes {
		logrus.Infof("picking %s %s", c.ShortSHA(), c.Title())
		logrus.Debugf("git cherry-pick %s", c.SHA())
		out, err := git.Raw("cherry-pick", simpleArg(c.SHA()))
		if err != nil {
			// append output to the error for more context
			err = multierror.Append(err, errors.New(strings.TrimSpace(out)))

			// we got an error, BUT git rerere may potentially have resolved it on our behalf
			hasConflicts, errConflicts := hasMergeConflicts()
			if errConflicts != nil || hasConflicts {
				return multierror.Append(err, errConflicts, abortCherryPick())
			}

			// seems like conflicts have been automatically solved, probably
			// to things like `git rerere`. Let's thank the black magic and proceed.
			// we first need to make sure all files are checked in and that
			// we have no unmerged changes
			logrus.Warn("merge conflict occurred but automatically resolved, proceeding")
			logrus.Infof("adding unmerged files and continuing")
			unmerged, errUnmerged := listUnmergedFiles()
			if errUnmerged != nil {
				return multierror.Append(err, errUnmerged, abortCherryPick())
			}
			for _, f := range unmerged {
				addErr := doGit(logrus.DebugLevel, func() (string, error) {
					return git.Add(simpleArg(f))
				})
				if addErr != nil {
					return multierror.Append(err, addErr, abortCherryPick())
				}
			}

			// todo: mark the commit someway and print a list of fixup
			// at the end of the sync process
			continueError := doGit(logrus.DebugLevel, func() (string, error) {
				// note: we allow empty commits because we want to annotate
				// them in case of weird fixups
				return git.Raw("commit", simpleArg("--allow-empty"))
			})
			if continueError != nil {
				return multierror.Append(err, continueError, abortCherryPick())
			}
		}
	}
	return nil
}

func listUnmergedFiles() ([]string, error) {
	logrus.Debugf("git diff --name-only --diff-filter=U --relative")
	out, err := git.Raw("diff", simpleArg("--name-only"), simpleArg("--diff-filter=U"), simpleArg("--relative"))
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	return lines, nil
}

func hasMergeConflicts() (bool, error) {
	out, err := git.Raw("diff", simpleArg("--check"))
	if err != nil {
		return false, err
	}
	return len(strings.TrimSpace(out)) != 0, nil
}

func abortCherryPick() error {
	// revert cherry-picking to restore git status
	logrus.Error("unrecoverable merge conflict occurred, stopping")
	logrus.Info("reverting cherry-pick")
	logrus.Debug("git reset --hard")
	_, err := git.Reset(reset.Hard)
	return err
}
