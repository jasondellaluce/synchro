package sync

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

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

// this is invoked when a `git cherry-pick` fails with a non-zero status code,
// and the goal is to identify all the merge conflicts and attempt resolving
// them manually. A non-nil error is returned in case the recover attempt fails.
func attemptMergeConflictRecovery(git utils.GitHelper, out string, req *Request, commit *commitInfo) error {
	if err := requireWorkInRepoRootDir(git); err != nil {
		return err
	}

	// collect all non-content conflict info
	var nonContentConfilicts []ConflictInfo

	// count number of conflicts and use it later
	numConflicts := countMergeConflicts(out)

	// content conflicts will be handled through git rerere. If not, we'll
	// take this count in account later for defining the right action items
	numContentConflicts := countMergeContentConflicts(out)

	md, err := getModifyDeleteConflictInfos(out)
	if err != nil {
		return fmt.Errorf("could not check for modify/delete conflicts: %s", err.Error())
	}
	nonContentConfilicts = append(nonContentConfilicts, md...)

	rr, err := getRenameRenameConflictInfos(out)
	if err != nil {
		return fmt.Errorf("could not check for rename/rename conflicts: %s", err.Error())
	}
	nonContentConfilicts = append(nonContentConfilicts, rr...)

	rd, err := getRenameDeleteConflictInfos(out)
	if err != nil {
		return fmt.Errorf("could not check for rename/delete conflicts: %s", err.Error())
	}
	nonContentConfilicts = append(nonContentConfilicts, rd...)

	dm, err := getDeleteModifyConflictInfos(out)
	if err != nil {
		return fmt.Errorf("could not check for delete/modify conflicts: %s", err.Error())
	}
	nonContentConfilicts = append(nonContentConfilicts, dm...)

	dr, err := getDeleteRenameConflictInfos(out)
	if err != nil {
		return fmt.Errorf("could not check for delete/rename conflicts: %s", err.Error())
	}
	nonContentConfilicts = append(nonContentConfilicts, dr...)

	// check if the remaining merge conflicts are all content ones
	// or if there are some unknown from which we can't possibly recover
	unknownConflicts := numConflicts - (len(nonContentConfilicts) + numContentConflicts)
	if len(nonContentConfilicts) > numConflicts || unknownConflicts > 0 {
		return fmt.Errorf("unknown conflicts encountered (%d content, %d non-content, %d total), can't recover: %s", numContentConflicts, len(nonContentConfilicts), numConflicts, out)
	}

	// attempt recovering from all the non-content conflicts, one by one
	for _, conflict := range nonContentConfilicts {
		if err := conflict.Recover(git, req, commit); err != nil {
			return err
		}
	}

	// for content merge conflicts, check if the conflict markers
	// have all been solved already through `git rerere`, otherwise
	// return an error and provide guidance on how to solve the conflict
	// through manual intervention
	if numContentConflicts > 0 {
		out, err := git.DoOutput("diff", "--check")
		if err != nil {
			return fmt.Errorf("could not check for content conflicts: %s", err.Error())
		}

		// the output will not be empty if there are remaining content conflicts.
		// In that case we attempt to extract them and recovery from them
		if len(out) > 0 {
			cc, err := getContentConflictInfos(out)
			if err != nil {
				return fmt.Errorf("could not parse for content conflicts: %s", err.Error())
			}

			for _, conflict := range cc {
				if err := conflict.Recover(git, req, commit); err != nil {
					// in case recovery is impossible, we write to stdout some guidance
					// on how users can proceed manually
					suggestion := formatConflictSuggestion(contentConflictSuggestion, &conflictSuggestionInfo{
						UpstreamOrg:       req.UpstreamOrg,
						UpstreamRepo:      req.UpstreamRepo,
						UpstreamRef:       req.UpstreamHeadRef,
						ForkOrg:           req.ForkOrg,
						ForkRepo:          req.ForkRepo,
						ConflictCommitSHA: commit.SHA(),
						BranchName:        req.OutBranch,
					})
					fmt.Fprintf(os.Stdout, "%s\n", suggestion)
					return err
				}
			}
		}

		logrus.Warn("merge content conflict detected but automatically resolved, proceeding")
	}

	// check that we didn't miss any unmerged file and stage all changes. At this
	// point only content conflicts should be unmerged.
	unmerged, err := git.ListUnmergedFiles()
	if err != nil {
		return err
	}
	if len(unmerged) != numContentConflicts {
		return fmt.Errorf("found %d unmerged files but expected %d: %s", len(unmerged), numContentConflicts, strings.Join(unmerged, ","))
	}
	err = git.Do("add", "-A")
	if err != nil {
		return fmt.Errorf("could not recover from content conflict: %s", err.Error())
	}

	return nil
}

func requireWorkInRepoRootDir(git utils.GitHelper) error {
	// note: merge conflicts will give relative paths of conflicting files,
	// so if automatic recovery is needed we have to make sure that we
	// are in the repo's root diretory
	logrus.Debug("making sure app is executing in repo root directory")
	repoRootDir, err := git.GetRepoRootDir()
	if err != nil {
		return err
	}
	curDir, err := os.Getwd()
	if err != nil {
		return err
	}
	if repoRootDir != curDir {
		logrus.Infof("changing working directory to repo's root: %s", repoRootDir)
		err := os.Chdir(repoRootDir)
		if err != nil {
			return err
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

func getDeleteModifyConflictInfos(s string) ([]ConflictInfo, error) {
	var res []ConflictInfo
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

func getDeleteRenameConflictInfos(s string) ([]ConflictInfo, error) {
	var res []ConflictInfo
	matches := rgxConflictDeleteRename.FindAllStringSubmatch(s, -1)
	for _, m := range matches {
		if len(m) != 3 {
			return nil, fmt.Errorf("unexpected regex match when looking for delete/rename merge conflict error")
		}
		res = append(res, &deleteRenameConflictInfo{
			UpstreamDeleted:   m[1],
			DownstreamRenamed: m[2],
		})
	}
	return res, nil
}

func getRenameRenameConflictInfos(s string) ([]ConflictInfo, error) {
	var res []ConflictInfo
	matches := rgxConflictRenameRename.FindAllStringSubmatch(s, -1)
	for _, m := range matches {
		if len(m) != 4 {
			return nil, fmt.Errorf("unexpected regex match when looking for rename/rename merge conflict error")
		}
		res = append(res, &renameRenameConflictInfo{
			UpstreamOriginal:  m[1],
			UpstreamRenamed:   m[2],
			DownstreamRenamed: m[3],
		})
	}
	return res, nil
}

func getRenameDeleteConflictInfos(s string) ([]ConflictInfo, error) {
	var res []ConflictInfo
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

func getModifyDeleteConflictInfos(s string) ([]ConflictInfo, error) {
	var res []ConflictInfo
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

func getContentConflictInfos(s string) ([]ConflictInfo, error) {
	var res []ConflictInfo

	// Read output line by line, which is in the form of:
	// CMakeLists.txt:1: leftover conflict marker
	// CMakeLists.txt:2: leftover conflict marker
	// CMakeLists.txt:18: leftover conflict marker
	files := make(map[string]*contentConflictInfo)
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		tokens := strings.Split(scanner.Text(), ":")
		if len(tokens) < 2 {
			return nil, fmt.Errorf("can't parse content conflict line: %s", scanner.Text())
		}
		fileName := tokens[0]
		_, ok := files[fileName]
		if !ok {
			info := &contentConflictInfo{Modified: fileName}
			res = append(res, info)
			files[fileName] = info
		}
		// todo(jasondellaluce): also collect conflict markers in the future
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return res, nil
}
