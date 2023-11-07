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
		out, err := git.DoOutput("cherry-pick", "--allow-empty", c.SHA())
		if err != nil {
			err = fmt.Errorf("merge conflict on commit: %s", c.SHA())
			recoveryErr := attemptMergeConflictRecovery(git, out)
			if recoveryErr != nil {
				logrus.Error("unrecoverable merge conflict occurred, reverting patch")
				return multierror.Append(err, recoveryErr, git.Do("reset", "--hard"))
			}

			// TODO: add a note or body comment describing what commit has been
			// ported and how the automatic merge happened

			continueErr := git.Do("cherry-pick", "--allow-empty", "--continue")
			if continueErr != nil {
				logrus.Error("failed continuing cherry-pick, reverting patch")
				return multierror.Append(err, continueErr, git.Do("reset", "--hard"))
			}
		}
	}
	return nil
}
