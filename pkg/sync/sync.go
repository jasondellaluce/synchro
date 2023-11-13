package sync

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/go-github/v56/github"
	"github.com/hashicorp/go-multierror"
	"github.com/jasondellaluce/synchro/pkg/utils"
	"github.com/sirupsen/logrus"
)

func Sync(ctx context.Context, git utils.GitHelper, client *github.Client, req *Request) error {
	if err := requireNoLocalChanges(git); err != nil {
		return err
	}

	// run a repo scan and collect all the private fork patches
	scanRes, err := scan(ctx, client, req)
	if err != nil {
		return err
	}

	// if we're in dry-run mode, just preview the changes and quit
	if req.DryRun {
		logrus.Info("skipping performing sync due to dry run request")
		for _, c := range scanRes {
			fmt.Fprintf(os.Stdout, "git cherry-pick %s # %s\n", c.SHA(), c.Title())
		}
		return nil
	}

	// check that the current repo is the actual fork and the tool
	// is not erroneously run from the wrong repo
	logrus.Infof("checking that the current repo is the fork one")
	remotes, err := git.GetRemotes()
	if err != nil {
		return err
	}
	if len(remotes) == 0 {
		return fmt.Errorf("can't find any remotes in current repo")
	}
	if originRemote, ok := remotes["origin"]; !ok {
		return fmt.Errorf("can't find `origin` remote in current repo")
	} else if !strings.HasSuffix(originRemote, fmt.Sprintf("%s/%s.git", req.ForkOrg, req.ForkRepo)) {
		return fmt.Errorf("current repo `origin` remote does not match the fork's one: %s", originRemote)
	}

	// apply all the patches one by one
	remoteName := fmt.Sprintf("temp-%s-sync-upstream", utils.ProjectName)
	remoteURL := fmt.Sprintf("https://github.com/%s/%s", req.UpstreamOrg, req.UpstreamRepo)
	logrus.Infof("initiating fork sync for repository %s/%s with upstream %s/%s", req.ForkOrg, req.ForkRepo, req.UpstreamOrg, req.UpstreamRepo)
	return withTempGitRemote(git, remoteName, remoteURL, func() error {
		return withTempLocalBranch(git, req.OutBranch, remoteName, req.UpstreamHeadRef, func() error {
			// we're now at the HEAD of the branch in the upstream repository, in
			// our local copy. Let's proceed cherry-picking all the patches.
			return applyAllPatches(ctx, git, req, scanRes)
		})
	})
}

func applyAllPatches(ctx context.Context, git utils.GitHelper, req *Request, scanRes []*commitInfo) error {
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
