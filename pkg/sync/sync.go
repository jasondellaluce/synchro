package sync

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/jasondellaluce/synchro/pkg/scan"
	"github.com/jasondellaluce/synchro/pkg/utils"
	"github.com/sirupsen/logrus"
)

type SyncRequest struct {
	Scan        scan.ScanRequest
	ScanRes     []*scan.CommitInfo
	BaseHeadRef string
	SyncBranch  string
}

func Sync(ctx context.Context, git utils.GitHelper, req *SyncRequest) error {
	if err := requireNoLocalChanges(git); err != nil {
		return err
	}

	// todo: check that origin remote is the actual repo of fork in req request

	remoteName := fmt.Sprintf("temp-%s-sync-base-remote", utils.ProjectName)
	remoteURL := fmt.Sprintf("https://github.com/%s/%s", req.Scan.BaseOrg, req.Scan.BaseRepo)
	logrus.Infof("initiating fork sync for repository %s/%s with base %s/%s", req.Scan.ForkOrg, req.Scan.ForkRepo, req.Scan.BaseOrg, req.Scan.BaseRepo)
	return withTempGitRemote(git, remoteName, remoteURL, func() error {
		return withTempLocalBranch(git, req.SyncBranch, remoteName, req.BaseHeadRef, func() error {
			// we're now at the HEAD of the branch in the base repository, in
			// our local copy. Let's proceed cherry-picking all the patches.
			return syncAllPatches(ctx, git, req)
		})
	})
}

func syncAllPatches(ctx context.Context, git utils.GitHelper, req *SyncRequest) error {
	// todo: track progress in tmp state file and eventually resume from there
	for _, c := range req.ScanRes {
		logrus.Infof("applying (%s) %s", c.ShortSHA(), c.Title())
		err := git.Do("cherry-pick", c.SHA())
		if err != nil {
			err = fmt.Errorf("merge conflict on commit: %s", c.SHA())
			recoveryErr := attemptCherryPickRecovery(git)
			if recoveryErr != nil {
				logrus.Error("unrecoverable merge conflict occurred, reverting patch")
				return multierror.Append(err, recoveryErr, git.Do("reset", "--hard"))
			}
		}
	}
	return nil
}

func attemptCherryPickRecovery(git utils.GitHelper) error {
	// `git rerere` may potentially have resolved it on our behalf,
	// so we check if there are actual conflicts remaining
	hasConflicts, err := git.HasMergeConflicts()
	if err != nil {
		return err
	}
	if hasConflicts {
		return multierror.Append(err, fmt.Errorf("unresolved merge conflicts detected"))
	}

	// seems like conflicts have been automatically solved, probably
	// to things like `git rerere`. Let's thank the black magic and proceed.
	// we first need to make sure all files are checked in and that
	// we have no unmerged changes
	logrus.Warn("merge conflict detected but automatically resolved, proceeding")
	unmerged, err := git.ListUnmergedFiles()
	if err != nil {
		return err
	}
	for _, f := range unmerged {
		err := git.Do("add", f)
		if err != nil {
			return err
		}
	}

	// todo: mark the commit someway and print a list of fixup at the end
	// of the sync process for traceability of automatic resolutions

	// conflict resolution may lead to an empty patch, check if there
	// are actual changes to be committed
	hasChanges, err := git.HasLocalChanges()
	if err != nil {
		return err
	}
	if !hasChanges {
		// there is no chea
		err := git.Do("reset", "--hard")
		if err != nil {
			return err
		}
	}

	return git.Do("cherry-pick", "--continue")
}
