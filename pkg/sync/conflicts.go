package sync

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/jasondellaluce/synchro/pkg/utils"
	"github.com/sirupsen/logrus"
	"go.uber.org/multierr"
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
	ForkRenamed     string
}

// deleteModifyConflictInfo represents a conflict in which a file has
// been renamed both in upstream in the fork, but with different names
type renameRenameConflictInfo struct {
	UpstreamOriginal string
	UpstreamRenamed  string
	ForkRenamed      string
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

// this is invoked when a `git cherry-pick` fails with a non-zero status code,
// and the goal is to identify all the merge conflicts and attempt resolving
// them manually. A non-nil error is returned in case the recover attempt fails.
func attemptMergeConflictRecovery(git utils.GitHelper, out string) error {
	// TODO: make sure this is run at the root of thegit repo

	numConflicts := countMergeConflicts(out)

	// content conflicts will be handled through git rerere. If not, we'll
	// take this count in account later for defining the right action items
	numContentConflicts := countMergeContentConflicts(out)

	// in case a file has been modified upstream, but deleted downstream,
	// our policy is to delete the file.
	md, err := getModifyDeleteConflictInfos(out)
	if err != nil {
		return fmt.Errorf("could not check for modify/delete conflicts: %s", err.Error())
	}
	for _, c := range md {
		logrus.Warnf("merge conflict auto-recovery: modify/delete detected for file %s, deleting it", c.UpstreamModified)
		err := git.Do("rm", "-f", c.UpstreamModified)
		if err != nil {
			return fmt.Errorf("could not recover from modify/delete conflict: %s", err.Error())
		}
	}

	// in case a file has been renamed both upstream and downstream,
	// both with different names, our policy is to keep the downstream renaming.
	rr, err := getRenameRenameConflictInfos(out)
	if err != nil {
		return fmt.Errorf("could not check for rename/rename conflicts: %s", err.Error())
	}
	for _, c := range rr {
		logrus.Warnf("merge conflict auto-recovery: rename/rename detected for file %s, keeping downstream name %s", c.UpstreamOriginal, c.ForkRenamed)
		err = multierr.Append(git.Do("rm", "-f", c.ForkRenamed), git.Do("mv", c.UpstreamRenamed, c.ForkRenamed))
		if err != nil {
			return fmt.Errorf("could not recover from rename/rename conflict: %s", err.Error())
		}
	}

	// in case a file has been renamed upstream, but deleted downstream,
	// our policy is to delete the file.
	rd, err := getRenameDeleteConflictInfos(out)
	if err != nil {
		return fmt.Errorf("could not check for rename/delete conflicts: %s", err.Error())
	}
	for _, c := range rd {
		logrus.Warnf("merge conflict auto-recovery: rename/delete detected for file %s, deleting it", c.UpstreamOriginal)
		err = multierr.Append(git.Do("rm", "-f", c.UpstreamOriginal), git.Do("rm", "-f", c.UpstreamRenamed))
		if err != nil {
			return fmt.Errorf("could not recover from rename/delete conflict: %s", err.Error())
		}
	}

	// in case a file has been deleted upstream, but modified downstream,
	// our policy is to delete the file.
	// note: this is one of the most dangerous recovery method as it could lead
	// to build or test failures, which should be dealt with manually.
	dm, err := getDeleteModifyConflictInfos(out)
	if err != nil {
		return fmt.Errorf("could not check for delete/modify conflicts: %s", err.Error())
	}
	for _, c := range dm {
		// TODO: print out action items in case of problems
		logrus.Warnf("merge conflict auto-recovery: delete/modify detected for file %s, deleting it", c.UpstreamDeleted)
		err = git.Do("rm", "-f", c.UpstreamDeleted)
		if err != nil {
			return fmt.Errorf("could not recover from delete/modify conflict: %s", err.Error())
		}
	}

	// in case a file has been deleted upstream, but renamed downstream,
	// our policy is to delete the file.
	// note: this is one of the most dangerous recovery method as it could lead
	// to build or test failures, which should be dealt with manually.
	dr, err := getDeleteRenameConflictInfos(out)
	if err != nil {
		return fmt.Errorf("could not check for delete/rename conflicts: %s", err.Error())
	}
	for _, c := range dr {
		// TODO: print out action items in case of problems
		logrus.Warnf("merge conflict auto-recovery: delete/rename detected for file %s, deleting it", c.UpstreamDeleted)
		err = multierr.Append(git.Do("rm", "-f", c.UpstreamDeleted), git.Do("rm", "-f", c.ForkRenamed))
		if err != nil {
			return fmt.Errorf("could not recover from delete/rename conflict: %s", err.Error())
		}
	}

	numNonContentConfilicts := len(md) + len(rr) + len(rd) + len(dm) + len(dr)
	if numConflicts > numNonContentConfilicts {
		// check if the remaining merge conflicts are all content ones
		// or if there are some unknown from which we can't possibly recover
		unknownConflicts := numConflicts - (numNonContentConfilicts + numContentConflicts)
		if unknownConflicts > 0 {
			return fmt.Errorf("%d unknown conflicts encountered, can't recover: %s", unknownConflicts, out)
		}

		// at this point we have only content conflicts remaining, check if
		// they have all been solved already through `git rerere`, otherwise
		// return an error and provide guidance on how to solve the conflic
		// through manual intervention
		unmerged, err := git.ListUnmergedFiles()
		if err != nil {
			return err
		}
		if len(unmerged) > 0 {
			// TODO: print out action items
			return fmt.Errorf("could not recover from content/content conflict, must solve manually with git rerere")
		}

		logrus.Warn("merge content conflict detected but automatically resolved, proceeding")
		err = git.Do("add", "-A")
		if err != nil {
			return fmt.Errorf("could not recover from content conflict: %s", err.Error())
		}
	}

	return nil
}

func countMergeConflicts(s string) int {
	return strings.Count(s, "CONFLICT (")
}

func countMergeContentConflicts(s string) int {
	return strings.Count(s, "CONFLICT (content)")
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
			ForkRenamed:     m[2],
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
			ForkRenamed:      m[3],
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
