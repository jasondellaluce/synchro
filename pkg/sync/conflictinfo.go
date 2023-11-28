package sync

import (
	"fmt"

	"github.com/jasondellaluce/synchro/pkg/utils"
	"github.com/sirupsen/logrus"
	"go.uber.org/multierr"
)

// abstract interface for merge conflicts from which we can attempt recovering from
type ConflictInfo interface {
	Recover(git utils.GitHelper, r *Request, c *commitInfo) error
}

func wrapRecoveryError(recType string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("could not recover from %s conflict: %s", recType, err.Error())
}

// deleteModifyConflictInfo represents a conflict in which a file has both
// been deleted upstream and modified in the fork
type deleteModifyConflictInfo struct {
	UpstreamDeleted string
}

// a file has been deleted upstream, but modified downstream
// note: this is one of the most dangerous recovery method as it could lead
// to build or test failures, which should be dealt with manually.
func (info *deleteModifyConflictInfo) Recover(git utils.GitHelper, r *Request, c *commitInfo) error {
	// with CommitMarkerConflictApply, we preserve the file and apply the edits
	if c.HasMarker(CommitMarkerConflictApply) {
		logrus.Warnf("merge conflict auto-recovery (%s): delete/modify detected for file %s, preserving and modifying it", CommitMarkerConflictApply, info.UpstreamDeleted)
		// note: here we assume that git left in tree the modified version
		return wrapRecoveryError("delete/modify", git.Do("add", info.UpstreamDeleted))
	}

	// with CommitMarkerConflictSkip (default), we delete the file
	logrus.Warnf("merge conflict auto-recovery (%s): delete/modify detected for file %s, deleting it", CommitMarkerConflictSkip, info.UpstreamDeleted)
	err := git.Do("rm", "-f", info.UpstreamDeleted)
	if err != nil {
		// note: not return on error because files can potentially not be there
		// and we would catch inconsistencies anyways when staging files later
		logrus.Error(err.Error())
	}
	return nil
}

// deleteRenameConflictInfo represents a conflict in which a file has both
// been deleted upstream and renamed in the fork
type deleteRenameConflictInfo struct {
	UpstreamDeleted string
	ForkRenamed     string
}

// a file has been deleted upstream, but renamed downstream
// note: this is one of the most dangerous recovery method as it could lead
// to build or test failures, which should be dealt with manually.
func (info *deleteRenameConflictInfo) Recover(git utils.GitHelper, r *Request, c *commitInfo) error {
	// with CommitMarkerConflictApply, we preserve the file and rename it
	if c.HasMarker(CommitMarkerConflictApply) {
		logrus.Warnf("merge conflict auto-recovery (%s): delete/rename detected for file %s, preserving and modifying it", CommitMarkerConflictApply, info.UpstreamDeleted)
		// note: here we assume that git left in tree the renamed version
		return wrapRecoveryError("delete/rename", git.Do("add", info.ForkRenamed))
	}

	// with CommitMarkerConflictSkip (default), we delete the file
	logrus.Warnf("merge conflict auto-recovery (%s): delete/rename detected for file %s, deleting it", CommitMarkerConflictSkip, info.UpstreamDeleted)
	err := multierr.Append(git.Do("rm", "-f", info.UpstreamDeleted), git.Do("rm", "-f", info.ForkRenamed))
	if err != nil {
		// note: not return on error because files can potentially not be there
		// and we would catch inconsistencies anyways when staging files later
		logrus.Error(err.Error())
	}
	return nil
}

// renameRenameConflictInfo represents a conflict in which a file has
// been renamed both in upstream in the fork, but with different names
type renameRenameConflictInfo struct {
	UpstreamOriginal string
	UpstreamRenamed  string
	ForkRenamed      string
}

// a file has been renamed both upstream and downstream
func (info *renameRenameConflictInfo) Recover(git utils.GitHelper, r *Request, c *commitInfo) error {
	// with CommitMarkerConflictSkip, we keep the file with the upstream name
	if c.HasMarker(CommitMarkerConflictSkip) {
		logrus.Warnf("merge conflict auto-recovery (%s): rename/rename detected for file %s, keeping upstream name", CommitMarkerConflictSkip, info.UpstreamOriginal)
		err := git.Do("rm", "-f", info.ForkRenamed)
		if err != nil {
			logrus.Error(err.Error())
		}
		return wrapRecoveryError("rename/rename", git.Do("add", info.UpstreamRenamed))
	}

	// with CommitMarkerConflictApply (default), we keep the file with the downstream name
	logrus.Warnf("merge conflict auto-recovery (%s): rename/rename detected for file %s, keeping downstream name %s", CommitMarkerConflictApply, info.UpstreamOriginal, info.ForkRenamed)
	err := git.Do("rm", "-f", info.ForkRenamed)
	if err != nil {
		logrus.Error(err.Error())
	}
	return wrapRecoveryError("rename/rename", git.Do("mv", info.UpstreamRenamed, info.ForkRenamed))
}

// renameDeleteConflictInfo represents a conflict in which a file has both
// been renamed upstream and deleted in the fork
type renameDeleteConflictInfo struct {
	UpstreamOriginal string
	UpstreamRenamed  string
}

// a file has been renamed upstream, but deleted downstream
func (info *renameDeleteConflictInfo) Recover(git utils.GitHelper, r *Request, c *commitInfo) error {
	// with CommitMarkerConflictSkip, we keep the renamed file
	if c.HasMarker(CommitMarkerConflictSkip) {
		logrus.Warnf("merge conflict auto-recovery (%s): rename/delete detected for file %s, keeping with upstream name", CommitMarkerConflictSkip, info.UpstreamOriginal)
		// note: here we assume that git left in tree the renamed version
		return wrapRecoveryError("rename/delete", git.Do("add", info.UpstreamRenamed))
	}

	// with CommitMarkerConflictApply (default), we delete the file
	logrus.Warnf("merge conflict auto-recovery (%s): rename/delete detected for file %s, deleting it", CommitMarkerConflictApply, info.UpstreamOriginal)
	err := multierr.Append(git.Do("rm", "-f", info.UpstreamOriginal), git.Do("rm", "-f", info.UpstreamRenamed))
	if err != nil {
		// note: not return on error because files can potentially not be there
		// and we would catch inconsistencies anyways when staging files later
		logrus.Error(err.Error())
	}
	return nil
}

// modifyDeleteConflictInfo represents a conflict in which a file has both
// been modified upstream and deleted in the fork
type modifyDeleteConflictInfo struct {
	UpstreamModified string
}

// a file has been modified upstream, but deleted downstream
func (info *modifyDeleteConflictInfo) Recover(git utils.GitHelper, r *Request, c *commitInfo) error {
	// with CommitMarkerConflictSkip, we keep the renamed file
	if c.HasMarker(CommitMarkerConflictSkip) {
		logrus.Warnf("merge conflict auto-recovery (%s): modify/delete detected for file %s, keeping with modified", CommitMarkerConflictSkip, info.UpstreamModified)
		// note: here we assume that git left in tree the modified version
		return wrapRecoveryError("modify/delete", git.Do("add", info.UpstreamModified))
	}

	// with CommitMarkerConflictApply, we delete the file
	logrus.Warnf("merge conflict auto-recovery (%s): modify/delete detected for file %s, deleting it", CommitMarkerConflictApply, info.UpstreamModified)
	return wrapRecoveryError("modify/delete", git.Do("rm", "-f", info.UpstreamModified))
}

// contentConflictInfo represents a conflict in which a file has been modified
// both in upstream and downstream in similar locations but with different changes
type contentConflictInfo struct {
	Modified string
}

func (info *contentConflictInfo) Recover(git utils.GitHelper, r *Request, c *commitInfo) error {
	// with CommitMarkerConflictSkip, we keep the upstream version of the conflicting files
	if c.HasMarker(CommitMarkerConflictSkip) {
		logrus.Warnf("merge conflict auto-recovery (%s): content conflict in file %s, keeping upstream changes", CommitMarkerConflictSkip, info.Modified)
		return wrapRecoveryError("content", git.Do("checkout", "--ours", info.Modified))
	}

	// with CommitMarkerConflictApply, we keep the downstream version of the conflicting files
	if c.HasMarker(CommitMarkerConflictApply) {
		logrus.Warnf("merge conflict auto-recovery (%s): content conflict in file %s, keeping downstream changes", CommitMarkerConflictApply, info.Modified)
		return wrapRecoveryError("content", git.Do("checkout", "--theirs", info.Modified))
	}

	return fmt.Errorf("content conflict can't be solved automatically for file %s", info.Modified)
}
