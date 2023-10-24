package sync

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/jasondellaluce/synchross/pkg/scan"
	"github.com/jasondellaluce/synchross/pkg/utils"
	"github.com/ldez/go-git-cmd-wrapper/v2/git"
	"github.com/ldez/go-git-cmd-wrapper/v2/revparse"
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
		logrus.Infof("picking commit %s", c.SHA())
		logrus.Debugf("git cherry-pick %s", c.SHA())
		out, err := git.Raw("cherry-pick", revparse.Args(c.SHA()))
		if err != nil {
			return multierror.Append(err, errors.New(out))
		}
	}
	return nil
}
