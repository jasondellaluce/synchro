package sync

import (
	"fmt"

	"github.com/jasondellaluce/synchro/pkg/utils"
	"github.com/sirupsen/logrus"
	"go.uber.org/multierr"
)

// ConflictInfo represents information about a merge conflict and how to recover from it
type ConflictInfo interface {
	String() string
	Description() string
	RecoverDescription() string
	Recover(git utils.GitHelper, r *Request, c *commitInfo) error
}

// AllConflictInfos is a collection of all merge conflict infos supported
var AllConflictInfos = []ConflictInfo{
	&contentConflictInfo{},
	&deleteModifyConflictInfo{},
	&deleteRenameConflictInfo{},
	&renameRenameConflictInfo{},
	&renameDeleteConflictInfo{},
	&modifyDeleteConflictInfo{},
}

// contentConflictInfo represents a conflict in which a file has been modified
// both in upstream and downstream in similar locations but with different changes
type contentConflictInfo struct {
	Modified string
}

func (info *contentConflictInfo) String() string {
	return "content"
}

func (info *contentConflictInfo) Description() string {
	return "A file has been modified both in upstream and downstream in similar locations but with different changes"
}

func (info *contentConflictInfo) RecoverDescription() string {
	return fmt.Sprintf("Conflict markers are solved by accepting the upstream modifications if the commit is marked with `%s`, and by accepting the downstream ones if marked with `%s`. ", CommitMarkerConflictSkip, CommitMarkerConflictApply) +
		fmt.Sprintf("By default, conflicts are tentatively solved using the cache provided of the `%s conflict` commands (powerded by `git rerere`). ", utils.ProjectName) +
		"If failing, the recovery attempt is aborted and guidance on the required manual intervention is provided"
}

func (info *contentConflictInfo) Recover(git utils.GitHelper, r *Request, c *commitInfo) error {
	// with CommitMarkerConflictSkip, we keep the upstream version of the conflicting files
	if c.HasMarker(CommitMarkerConflictSkip) {
		logrus.Warnf("merge conflict auto-recovery (%s): content conflict in file %s, keeping upstream changes", CommitMarkerConflictSkip, info.Modified)
		return recoverErr("content", git.Do("checkout", "--ours", info.Modified))
	}

	// with CommitMarkerConflictApply, we keep the downstream version of the conflicting files
	if c.HasMarker(CommitMarkerConflictApply) {
		logrus.Warnf("merge conflict auto-recovery (%s): content conflict in file %s, keeping downstream changes", CommitMarkerConflictApply, info.Modified)
		return recoverErr("content", git.Do("checkout", "--theirs", info.Modified))
	}

	return fmt.Errorf("content conflict can't be solved automatically for file %s", info.Modified)
}

// deleteModifyConflictInfo represents a conflict in which a file has both
// been deleted upstream and modified downstream
type deleteModifyConflictInfo struct {
	UpstreamDeleted string
}

func (info *deleteModifyConflictInfo) String() string {
	return "delete-modify"
}

func (info *deleteModifyConflictInfo) Description() string {
	return "A file has both been deleted upstream and modified downstream"
}

func (info *deleteModifyConflictInfo) RecoverDescription() string {
	return fmt.Sprintf("The file is preserved with the new modifications if the commit is marked with `%s`, and deleted otherwise", CommitMarkerConflictApply)
}

// a file has been deleted upstream, but modified downstream
// note: this is one of the most dangerous recovery method as it could lead
// to build or test failures, which should be dealt with manually.
func (info *deleteModifyConflictInfo) Recover(git utils.GitHelper, r *Request, c *commitInfo) error {
	// with CommitMarkerConflictApply, we preserve the file and apply the edits
	if c.HasMarker(CommitMarkerConflictApply) {
		logrus.Warnf("merge conflict auto-recovery (%s): delete/modify detected for file %s, preserving and modifying it", CommitMarkerConflictApply, info.UpstreamDeleted)
		// note: here we assume that git left in tree the modified version
		return recoverErr("delete/modify", git.Do("add", info.UpstreamDeleted))
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
// been deleted upstream and renamed downstream
type deleteRenameConflictInfo struct {
	UpstreamDeleted   string
	DownstreamRenamed string
}

func (info *deleteRenameConflictInfo) String() string {
	return "delete-rename"
}

func (info *deleteRenameConflictInfo) Description() string {
	return "A file has both been deleted upstream and renamed downstream"
}

func (info *deleteRenameConflictInfo) RecoverDescription() string {
	return fmt.Sprintf("The file is preserved with the new name if the commit is marked with `%s`, and deleted otherwise", CommitMarkerConflictApply)
}

// a file has been deleted upstream, but renamed downstream
// note: this is one of the most dangerous recovery method as it could lead
// to build or test failures, which should be dealt with manually.
func (info *deleteRenameConflictInfo) Recover(git utils.GitHelper, r *Request, c *commitInfo) error {
	// with CommitMarkerConflictApply, we preserve the file and rename it
	if c.HasMarker(CommitMarkerConflictApply) {
		logrus.Warnf("merge conflict auto-recovery (%s): delete/rename detected for file %s, preserving and modifying it", CommitMarkerConflictApply, info.UpstreamDeleted)
		// note: here we assume that git left in tree the renamed version
		return recoverErr("delete/rename", git.Do("add", info.DownstreamRenamed))
	}

	// with CommitMarkerConflictSkip (default), we delete the file
	logrus.Warnf("merge conflict auto-recovery (%s): delete/rename detected for file %s, deleting it", CommitMarkerConflictSkip, info.UpstreamDeleted)
	err := multierr.Append(git.Do("rm", "-f", info.UpstreamDeleted), git.Do("rm", "-f", info.DownstreamRenamed))
	if err != nil {
		// note: not return on error because files can potentially not be there
		// and we would catch inconsistencies anyways when staging files later
		logrus.Error(err.Error())
	}
	return nil
}

// renameRenameConflictInfo represents a conflict in which a file has
// been renamed both upstream and downstream, but with different names
type renameRenameConflictInfo struct {
	UpstreamOriginal  string
	UpstreamRenamed   string
	DownstreamRenamed string
}

func (info *renameRenameConflictInfo) String() string {
	return "rename-rename"
}

func (info *renameRenameConflictInfo) Description() string {
	return "A file has been renamed both upstream and downstream, but with different names"
}

func (info *renameRenameConflictInfo) RecoverDescription() string {
	return fmt.Sprintf("The file is renamed with the upstream if the commit is marked with `%s`, and with the downstream name otherwise", CommitMarkerConflictSkip)
}

// a file has been renamed both upstream and downstream
func (info *renameRenameConflictInfo) Recover(git utils.GitHelper, r *Request, c *commitInfo) error {
	// with CommitMarkerConflictSkip, we keep the file with the upstream name
	if c.HasMarker(CommitMarkerConflictSkip) {
		logrus.Warnf("merge conflict auto-recovery (%s): rename/rename detected for file %s, keeping upstream name", CommitMarkerConflictSkip, info.UpstreamOriginal)
		err := git.Do("rm", "-f", info.DownstreamRenamed)
		if err != nil {
			logrus.Error(err.Error())
		}
		return recoverErr("rename/rename", git.Do("add", info.UpstreamRenamed))
	}

	// with CommitMarkerConflictApply (default), we keep the file with the downstream name
	logrus.Warnf("merge conflict auto-recovery (%s): rename/rename detected for file %s, keeping downstream name %s", CommitMarkerConflictApply, info.UpstreamOriginal, info.DownstreamRenamed)
	err := git.Do("rm", "-f", info.DownstreamRenamed)
	if err != nil {
		logrus.Error(err.Error())
	}
	return recoverErr("rename/rename", git.Do("mv", info.UpstreamRenamed, info.DownstreamRenamed))
}

// renameDeleteConflictInfo represents a conflict in which a file has both
// been renamed upstream and deleted downstream
type renameDeleteConflictInfo struct {
	UpstreamOriginal string
	UpstreamRenamed  string
}

func (info *renameDeleteConflictInfo) String() string {
	return "rename-delete"
}

func (info *renameDeleteConflictInfo) Description() string {
	return "A file has both been renamed upstream and deleted downstream"
}

func (info *renameDeleteConflictInfo) RecoverDescription() string {
	return fmt.Sprintf("The file is preserved with the new name if the commit is marked with `%s`, and deleted otherwise", CommitMarkerConflictSkip)
}

// a file has been renamed upstream, but deleted downstream
func (info *renameDeleteConflictInfo) Recover(git utils.GitHelper, r *Request, c *commitInfo) error {
	// with CommitMarkerConflictSkip, we keep the renamed file
	if c.HasMarker(CommitMarkerConflictSkip) {
		logrus.Warnf("merge conflict auto-recovery (%s): rename/delete detected for file %s, keeping with upstream name", CommitMarkerConflictSkip, info.UpstreamOriginal)
		// note: here we assume that git left in tree the renamed version
		return recoverErr("rename/delete", git.Do("add", info.UpstreamRenamed))
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
// been modified upstream and deleted downstream
type modifyDeleteConflictInfo struct {
	UpstreamModified string
}

func (info *modifyDeleteConflictInfo) String() string {
	return "modify-delete"
}

func (info *modifyDeleteConflictInfo) Description() string {
	return "A file has both been modified upstream and deleted downstream"
}

func (info *modifyDeleteConflictInfo) RecoverDescription() string {
	return fmt.Sprintf("The file is preserved with the new modifications if the commit is marked with `%s`, and deleted otherwise", CommitMarkerConflictSkip)
}

// a file has been modified upstream, but deleted downstream
func (info *modifyDeleteConflictInfo) Recover(git utils.GitHelper, r *Request, c *commitInfo) error {
	// with CommitMarkerConflictSkip, we keep the renamed file
	if c.HasMarker(CommitMarkerConflictSkip) {
		logrus.Warnf("merge conflict auto-recovery (%s): modify/delete detected for file %s, keeping with modified", CommitMarkerConflictSkip, info.UpstreamModified)
		// note: here we assume that git left in tree the modified version
		return recoverErr("modify/delete", git.Do("add", info.UpstreamModified))
	}

	// with CommitMarkerConflictApply, we delete the file
	logrus.Warnf("merge conflict auto-recovery (%s): modify/delete detected for file %s, deleting it", CommitMarkerConflictApply, info.UpstreamModified)
	return recoverErr("modify/delete", git.Do("rm", "-f", info.UpstreamModified))
}

func recoverErr(recType string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("could not recover from %s conflict: %s", recType, err.Error())
}
