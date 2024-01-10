package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/jasondellaluce/synchro/pkg/sync"
	"github.com/jasondellaluce/synchro/pkg/utils"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(explainCmd)
	explainCmd.AddCommand(explainMarkersCmd)
	explainCmd.AddCommand(explainConflictsCmd)
}

var explainCmd = &cobra.Command{
	Use:   "explain",
	Short: "Provide explanation about the tool's behavior",
}

var explainMarkersCmd = &cobra.Command{
	Use:   "markers",
	Short: "Lists and describes the supported commit markers",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(os.Stdout, "# Commit Markers\n\n")
		fmt.Fprintf(os.Stdout, "Commit markers are keywords that can be annotated either in the body message of a commit, "+
			"or in one or more GitHub comments relative to a commit. "+
			"They can be used to influence the behavior of the `%s` tool when scanning a given commit during a fork sync.\n\n",
			utils.ProjectName,
		)
		data := [][]string{{"Marker", "Description"}}
		for _, m := range sync.AllCommitMarkers {
			data = append(data, []string{"`" + m.String() + "`", m.Description()})
		}
		explainAsTable(data, os.Stdout)
	},
}

var explainConflictsCmd = &cobra.Command{
	Use:   "conflicts",
	Short: "Lists and describes the supported merge conflict automatic resolution scenarios",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(os.Stdout, "# Merge Conflict Recovery\n\n")
		fmt.Fprintf(os.Stdout, "The `%s` tools supports automatic recovery from many "+
			"scenarios of git merge conflict that could arise when picking a commit during a fork sync. "+
			"The default recovery strategy of the tool can be influenced by the markers annotated on each commit.\n\n",
			utils.ProjectName,
		)
		data := [][]string{{"Conflict", "Description", "Recovery"}}
		for _, c := range sync.AllConflictInfos {
			data = append(data, []string{"`" + c.String() + "`", c.Description(), c.RecoverDescription()})
		}
		explainAsTable(data, os.Stdout)
	},
}

func explainAsTable(data [][]string, w io.Writer) {
	table := tablewriter.NewWriter(w)
	table.SetHeader(data[0])
	table.SetAutoWrapText(false)
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	table.AppendBulk(data[1:])
	table.Render()
}
