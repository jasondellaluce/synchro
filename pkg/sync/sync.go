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

// SyncCommitBodyHeader is a keyword that can be used to prefix a line in the
// body message of a commit for specifying metadata about the sync process
// relative to that commit
var SyncCommitBodyHeader = strings.ToUpper(utils.ProjectName)

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
	} else if !strings.Contains(originRemote, fmt.Sprintf("%s/%s", req.ForkOrg, req.ForkRepo)) {
		return fmt.Errorf("current repo `origin` remote does not match the fork's one: %s", originRemote)
	}

	// apply all the patches one by one
	remoteName := fmt.Sprintf("temp-%s-sync-upstream", utils.ProjectName)
	remoteURL := fmt.Sprintf("https://github.com/%s/%s", req.UpstreamOrg, req.UpstreamRepo)
	logrus.Infof("initiating fork sync for repository %s/%s with upstream %s/%s", req.ForkOrg, req.ForkRepo, req.UpstreamOrg, req.UpstreamRepo)
	return utils.WithTempGitRemote(git, remoteName, remoteURL, func() error {
		return utils.WithTempLocalBranch(git, req.OutBranch, remoteName, req.UpstreamHeadRef, func() (bool, error) {
			// we're now at the HEAD of the branch in the upstream repository, in
			// our local copy. Let's proceed cherry-picking all the patches.
			return false, applyAllPatches(ctx, git, req, scanRes)
		})
	})
}

func applyAllPatches(ctx context.Context, git utils.GitHelper, req *Request, scanRes []*commitInfo) error {
	// todo: track progress in tmp state file and eventually resume from there
	for _, c := range scanRes {
		logrus.Infof("applying (%s) %s", c.ShortSHA(), c.Title())

		recovered := false
		out, err := git.DoOutput("cherry-pick", "--allow-empty", c.SHA())
		if err != nil {
			err = fmt.Errorf("merge conflict on commit: %s", c.SHA())
			recoveryErr := attemptMergeConflictRecovery(git, out, req, c)
			if recoveryErr != nil {
				logrus.Error("unrecoverable merge conflict occurred, reverting patch")
				return multierror.Append(err, recoveryErr, git.Do("reset", "--hard"))
			}
			recovered = true
			if hasChanges, changesErr := git.HasLocalChanges(); changesErr != nil {
				logrus.Error("failed checking for remaining changes, reverting patch")
				return multierror.Append(err, changesErr, git.Do("reset", "--hard"))
			} else if !hasChanges {
				logrus.Warn("cherry-pick is now empty possibly due to conflict resolution, skipping commit")
				continue
			}
			continueErr := git.Do("cherry-pick", "--continue")
			if continueErr != nil {
				logrus.Error("failed continuing cherry-pick, reverting patch")
				return multierror.Append(err, continueErr, git.Do("reset", "--hard"))
			}
		}

		// mark the commit with metadata about the automated sync
		var commitMsg strings.Builder
		prevMsg, err := git.DoOutput("log", "--format=%B", "-n1")
		if err != nil {
			logrus.Error("failed obtaining latest commit message")
			return err
		}
		commitURL := fmt.Sprintf("https://github.com/%s/%s/commit/%s", req.ForkOrg, req.ForkRepo, c.SHA())
		commitMsg.WriteString(commitMessageWithNoSyncMarkers(prevMsg) + "\n\n")
		commitMsg.WriteString(fmt.Sprintf("%s: porting of %s (%s)\n", SyncCommitBodyHeader, c.ShortSHA(), commitURL))
		if recovered {
			commitMsg.WriteString(fmt.Sprintf("%s: solved merge conflicts automatically\n", SyncCommitBodyHeader))
		}
		err = git.Do("commit", "--amend", "-m", commitMsg.String())
		if err != nil {
			logrus.Error("failed appending metadata to commit message")
			return err
		}
	}
	return nil
}

func commitMessageWithNoSyncMarkers(s string) string {
	var res strings.Builder
	for _, l := range strings.Split(s, "\n") {
		if !strings.HasPrefix(l, SyncCommitBodyHeader) {
			res.WriteString(l + "\n")
		}
	}
	return res.String()
}
