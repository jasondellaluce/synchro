package sync

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConflictInfoRetrieval(t *testing.T) {
	const conflictSample1 = `
CONFLICT (rename/delete): a.txt renamed to b.txt in HEAD, but deleted in d533f0e98 (test).
CONFLICT (modify/delete): a.txt deleted in d533f0e98 (test) and modified in HEAD.  Version HEAD of a.txt left in tree.
`

	const conflictSample2 = `
CONFLICT (rename/rename): a.txt renamed to a2.txt in HEAD and to a3.txt in 4b258cd86 (test).
CONFLICT (modify/delete): b.txt deleted in HEAD and modified in 4b258cd86 (test).  Version 4b258cd86 (test) of CMakeListsGtestInclude.cmake left in tree.
CONFLICT (rename/delete): c.txt renamed to c2.txt in 4b258cd86 (test), but deleted in HEAD.
`

	t.Run("delete-modify", func(t *testing.T) {
		expected := []*deleteModifyConflictInfo{
			{
				UpstreamDeleted: "b.txt",
			},
		}
		conflicts, err := getDeleteModifyConflictInfos(conflictSample2)
		assert.NoError(t, err)
		assert.Equal(t, expected, conflicts)
	})

	t.Run("delete-rename", func(t *testing.T) {
		expected := []*deleteRenameConflictInfo{
			{
				UpstreamDeleted: "c.txt",
				ForkRenamed:     "c2.txt",
			},
		}
		conflicts, err := getDeleteRenameConflictInfos(conflictSample2)
		assert.NoError(t, err)
		assert.Equal(t, expected, conflicts)
	})

	t.Run("rename-rename", func(t *testing.T) {
		expected := []*renameRenameConflictInfo{
			{
				UpstreamOriginal: "a.txt",
				UpstreamRenamed:  "a2.txt",
				ForkRenamed:      "a3.txt",
			},
		}
		conflicts, err := getRenameRenameConflictInfos(conflictSample2)
		assert.NoError(t, err)
		assert.Equal(t, expected, conflicts)
	})

	t.Run("rename-delete", func(t *testing.T) {
		expected := []*renameDeleteConflictInfo{
			{
				UpstreamOriginal: "a.txt",
				UpstreamRenamed:  "b.txt",
			},
		}
		conflicts, err := getRenameDeleteConflictInfos(conflictSample1)
		assert.NoError(t, err)
		assert.Equal(t, expected, conflicts)
	})

	t.Run("modify-delete", func(t *testing.T) {
		expected := []*modifyDeleteConflictInfo{
			{
				UpstreamModified: "a.txt",
			},
		}
		conflicts, err := getModifyDeleteConflictInfos(conflictSample1)
		assert.NoError(t, err)
		assert.Equal(t, expected, conflicts)
	})
}
