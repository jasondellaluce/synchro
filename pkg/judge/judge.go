package judge

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jasondellaluce/synchro/pkg/utils"
	"github.com/sirupsen/logrus"
)

type ChangeType int

const (
	Modify ChangeType = iota
	Add
	Delete
	Rename
)

type CommitChange struct {
	FileName string
	Type     ChangeType
}

func Judge(ctx context.Context, git utils.GitHelper, commit string) error {
	out, err := git.DoOutput("show", "--raw", commit)
	if err != nil {
		return err
	}

	modified := 0
	added := 0
	deleted := 0
	renamed := 0

	lines := strings.Split(out, "\n")
	var changes []CommitChange

	for _, line := range lines {
		if len(line) > 0 && line[0] == ':' {
			change, err := parseMetadataInfo(line)
			if err != nil {
				return err
			}
			changes = append(changes, change)
		}
	}

	for _, change := range changes {
		switch change.Type {
		case Modify:
			modified = 1
		case Add:
			added = 1
		case Delete:
			deleted = 1
		case Rename:
			renamed = 1
		}
	}

	if (modified + added + deleted + renamed) > 1 {
		logrus.Errorf("commits can either have modified, added, deleted or renamed files and not a combination of those")

		for _, change := range changes {
			if modified != 0 && change.Type == Modify {
				fmt.Fprintf(os.Stdout, "Modified file: %s\n", change.FileName)
			}
			if added != 0 && change.Type == Add {
				fmt.Fprintf(os.Stdout, "Added file: %s\n", change.FileName)
			}
			if deleted != 0 && change.Type == Delete {
				fmt.Fprintf(os.Stdout, "Deleted file: %s\n", change.FileName)
			}
			if renamed != 0 && change.Type == Rename {
				fmt.Fprintf(os.Stdout, "Renamed file: %s\n", change.FileName)
			}
		}
	}

	return err
}

func parseMetadataInfo(m string) (CommitChange, error) {
	var change CommitChange

	//split and trim spaces
	out := strings.Split(m, " ")
	for i := range out {
		out[i] = strings.TrimSpace(out[i])
	}

	if len(out) < 5 {
		return change, fmt.Errorf("cannot parse commit metadata informations")
	}

	if len(out[4]) > 0 {
		//change type and filename are separated by a tab
		out := strings.Split(out[4], "\t")
		if len(out) < 2 {
			return change, fmt.Errorf("cannot parse commit metadata informations")
		}
		switch out[0][0] {
		case 'M':
			change.Type = Modify
		case 'A':
			change.Type = Add
		case 'D':
			change.Type = Delete
		case 'R':
			change.Type = Rename
		default:
			return change, fmt.Errorf("cannot find change type in commit metadata")
		}

		if len(out[1]) > 0 {
			change.FileName = out[1]
		} else {
			return change, fmt.Errorf("cannot find file name in commit metadata")
		}
	} else {
		return change, fmt.Errorf("cannot parse commit metadata informations")
	}

	return change, nil
}
