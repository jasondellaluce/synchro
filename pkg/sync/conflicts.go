package sync

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/jasondellaluce/synchro/pkg/utils"
	"github.com/sirupsen/logrus"
)

var rgxConflictDeleteModify = regexp.MustCompile(
	`CONFLICT \(modify/delete\): ([a-zA-Z0-9\-_\.\\\/]+) deleted in HEAD and modified in [a-fA-F0-9]+ \(.*\)`)

var rgxConflictDeleteRename = regexp.MustCompile(
	`CONFLICT \(rename/delete\): ([a-zA-Z0-9\-_\.\\\/]+) renamed to ([a-zA-Z0-9\-_\.\\\/]+) in [a-fA-F0-9]+ \(.*\), but deleted in HEAD`)

var rgxConflictRenameRename = regexp.MustCompile(
	`CONFLICT \(rename/rename\): ([a-zA-Z0-9\-_\.\\\/]+) renamed to ([a-zA-Z0-9\-_\.\\\/]+) in HEAD and to ([a-zA-Z0-9\-_\.\\\/]+) in [a-fA-F0-9]+ \(.*\)`)

var rgxConflictRenameDelete = regexp.MustCompile(
	`CONFLICT \(rename/delete\): ([a-zA-Z0-9\-_\.\\\/]+) renamed to ([a-zA-Z0-9\-_\.\\\/]+) in HEAD, but deleted in [a-fA-F0-9]+ \(.*\)`)

var rgxConflictModifyDelete = regexp.MustCompile(
	`CONFLICT \(modify/delete\): ([a-zA-Z0-9\-_\.\\\/]+) deleted in [a-fA-F0-9]+ \(.*\) and modified in HEAD`)

// deleteModifyConflictInfo represents a conflict in which a file has both
// been deleted upstream and modified in the fork
type deleteModifyConflictInfo struct {
	UpstreamDeleted string
}

// deleteModifyConflictInfo represents a conflict in which a file has both
// been deleted upstream and renamed in the fork
type deleteRenameConflictInfo struct {
	UpstreamDeleted string
	ForkRenemed     string
}

// deleteModifyConflictInfo represents a conflict in which a file has
// been renamed both in upstream in the fork, but with different names
type renameRenameConflictInfo struct {
	UpstreamOriginal string
	UpstreamRenamed  string
	ForkRenemed      string
}

// renameDeleteConflictInfo represents a conflict in which a file has both
// been renamed upstream and deleted in the fork
type renameDeleteConflictInfo struct {
	UpstreamOriginal string
	UpstreamRenamed  string
}

// modifyDeleteConflictInfo represents a conflict in which a file has both
// been modified upstream and deleted in the fork
type modifyDeleteConflictInfo struct {
	UpstreamModified string
}

func attemptMergeConflictRecovery(git utils.GitHelper, mergeOut string) error {
	countMergeConflicts(mergeOut)

	hasConflicts, err := git.HasMergeConflicts() // todo: we may not need this
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

	return nil
}

func countMergeConflicts(out string) int {
	return strings.Count(out, "CONFLICT (")
}

func getDeleteModifyConflictInfos(s string) ([]*deleteModifyConflictInfo, error) {
	var res []*deleteModifyConflictInfo
	matches := rgxConflictDeleteModify.FindAllStringSubmatch(s, -1)
	for _, m := range matches {
		if len(m) != 2 {
			return nil, fmt.Errorf("unexpected regex match when looking for delete/modify merge conflict error")
		}
		res = append(res, &deleteModifyConflictInfo{
			UpstreamDeleted: m[1],
		})
	}
	return res, nil
}
func getDeleteRenameConflictInfos(s string) ([]*deleteRenameConflictInfo, error) {
	var res []*deleteRenameConflictInfo
	matches := rgxConflictDeleteRename.FindAllStringSubmatch(s, -1)
	for _, m := range matches {
		if len(m) != 3 {
			return nil, fmt.Errorf("unexpected regex match when looking for delete/rename merge conflict error")
		}
		res = append(res, &deleteRenameConflictInfo{
			UpstreamDeleted: m[1],
			ForkRenemed:     m[2],
		})
	}
	return res, nil
}

func getRenameRenameConflictInfos(s string) ([]*renameRenameConflictInfo, error) {
	var res []*renameRenameConflictInfo
	matches := rgxConflictRenameRename.FindAllStringSubmatch(s, -1)
	for _, m := range matches {
		if len(m) != 4 {
			return nil, fmt.Errorf("unexpected regex match when looking for rename/rename merge conflict error")
		}
		res = append(res, &renameRenameConflictInfo{
			UpstreamOriginal: m[1],
			UpstreamRenamed:  m[2],
			ForkRenemed:      m[3],
		})
	}
	return res, nil
}

func getRenameDeleteConflictInfos(s string) ([]*renameDeleteConflictInfo, error) {
	var res []*renameDeleteConflictInfo
	matches := rgxConflictRenameDelete.FindAllStringSubmatch(s, -1)
	for _, m := range matches {
		if len(m) != 3 {
			return nil, fmt.Errorf("unexpected regex match when looking for rename/delete merge conflict error")
		}
		res = append(res, &renameDeleteConflictInfo{
			UpstreamOriginal: m[1],
			UpstreamRenamed:  m[2],
		})
	}
	return res, nil
}

func getModifyDeleteConflictInfos(s string) ([]*modifyDeleteConflictInfo, error) {
	var res []*modifyDeleteConflictInfo
	matches := rgxConflictModifyDelete.FindAllStringSubmatch(s, -1)
	for _, m := range matches {
		if len(m) != 2 {
			return nil, fmt.Errorf("unexpected regex match when looking for modify/delete merge conflict error")
		}
		res = append(res, &modifyDeleteConflictInfo{
			UpstreamModified: m[1],
		})
	}
	return res, nil
}
