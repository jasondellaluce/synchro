package sync

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/jasondellaluce/synchro/pkg/utils"
)

func requireNoLocalChanges(git utils.GitHelper) error {
	if localChanges, err := git.HasLocalChanges(); err != nil || localChanges {
		if localChanges {
			err = multierror.Append(err, fmt.Errorf("local changes must be stashed, committed, or removed"))
		}
		return err
	}
	return nil
}
