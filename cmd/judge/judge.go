package judge

import (
	"context"
	"fmt"

	"github.com/jasondellaluce/synchro/pkg/judge"
	"github.com/jasondellaluce/synchro/pkg/utils"
	"github.com/spf13/cobra"
)

var JudgeCmd = &cobra.Command{
	Use:   "judge",
	Short: "Verifies that a commit does not contain harmful patches for the sync process",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 || len(args[0]) == 0 {
			return fmt.Errorf("must define a commit to judge")
		}
		ctx := context.Background()
		git := utils.NewGitHelper()
		return judge.Judge(ctx, git, args[0])
	},
}
