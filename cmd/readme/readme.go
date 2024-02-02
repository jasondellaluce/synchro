package readme

import (
	"fmt"
	"os"

	"github.com/jasondellaluce/synchro/cmd/explain"
	"github.com/jasondellaluce/synchro/pkg/utils"
	"github.com/spf13/cobra"
)

var ReadmeCmd = &cobra.Command{
	Use:   "readme",
	Short: "Generates a readme for the project",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(os.Stdout, "# %s\n\n", utils.ProjectName)
		fmt.Fprintf(os.Stdout, "_%s_\n\n", utils.ProjectDescription)
		fmt.Fprintf(os.Stdout, "## Installing\n\n")
		fmt.Fprintf(os.Stdout, "```\n")
		fmt.Fprintln(os.Stdout, `LATEST=$(curl -sI https://github.com/jasondellaluce/synchro/releases/latest | awk '/location: /{gsub("\r","",$2);split($2,v,"/");print substr(v[8],2)}')`)
		fmt.Fprintln(os.Stdout, `curl --fail -LS "https://github.com/jasondellaluce/synchro/releases/download/v${LATEST}/synchro_${LATEST}_linux_amd64.tar.gz" | tar -xz`)
		fmt.Fprintln(os.Stdout, `sudo install -o root -g root -m 0755 synchro /usr/local/bin/synchro`)
		fmt.Fprintf(os.Stdout, "```\n\n")
		fmt.Fprintf(os.Stdout, "#")
		explain.ExplainMarkersCmd.Run(cmd, args)
		fmt.Fprintf(os.Stdout, "\n#")
		explain.ExplainConflictsCmd.Run(cmd, args)
	},
}
