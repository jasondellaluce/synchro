package sync

import (
	"context"
	"fmt"
	"os"

	"github.com/google/go-github/v56/github"
	"github.com/hashicorp/go-multierror"
	"github.com/jasondellaluce/synchro/pkg/utils"
	"github.com/sirupsen/logrus"
)

func Sync(ctx context.Context, git utils.GitHelper, client *github.Client, req *Request) error {
	if err := requireNoLocalChanges(git); err != nil {
		return err
	}

	scanRes, err := scan(ctx, client, req)
	if err != nil {
		return err
	}
	if req.DryRun {
		logrus.Info("skipping performing sync due to dry run request")
		for _, c := range scanRes {
			fmt.Fprintf(os.Stdout, "git cherry-pick %s # %s\n", c.SHA(), c.Title())
		}
		return nil
	}

	// todo: check that origin remote is the actual repo of fork in req request

	remoteName := fmt.Sprintf("temp-%s-sync-upstream", utils.ProjectName)
	remoteURL := fmt.Sprintf("https://github.com/%s/%s", req.UpstreamOrg, req.UpstreamRepo)
	logrus.Infof("initiating fork sync for repository %s/%s with upstream %s/%s", req.ForkOrg, req.ForkRepo, req.UpstreamOrg, req.UpstreamRepo)
	return withTempGitRemote(git, remoteName, remoteURL, func() error {
		return withTempLocalBranch(git, req.OutBranch, remoteName, req.UpstreamHeadRef, func() error {
			// we're now at the HEAD of the branch in the upstream repository, in
			// our local copy. Let's proceed cherry-picking all the patches.
			return syncAllPatches(ctx, git, req, scanRes)
		})
	})
}

func syncAllPatches(ctx context.Context, git utils.GitHelper, req *Request, scanRes []*commitInfo) error {
	// todo: track progress in tmp state file and eventually resume from there
	for _, c := range scanRes {
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
	// TODO: it may also happen that we have different kinds of conflicts, we should deal with:
	// - CONFLICT (content) <- done with rerere
	// - CONFLICT (rename/delete)
	// - CONFLICT (modify/delete)
	// how do we deal with that? maybe we should just add everything?
	// `git rerere` may potentially have resolved it on our behalf,
	// so we check if there are actual conflicts remaining.
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
		return git.Do("reset", "--hard")
	}

	return git.Do("cherry-pick", "--continue")
}
