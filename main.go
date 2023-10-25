package main

import (
	"fmt"
	"os"

	"github.com/jasondellaluce/synchro/pkg/branchdb"
	"github.com/jasondellaluce/synchro/pkg/utils"
	"github.com/sirupsen/logrus"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
}

func exitOnErr(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func main() {
	// client := github.NewClient(nil)
	// token := os.Getenv("GITHUB_TOKEN")
	// if len(token) > 0 {
	// 	client = client.WithAuthToken(token)
	// } else {
	// 	logrus.Warn("the GITHUB_TOKEN env variable is not set, you may encounter authentication or rate limiting issues")
	// }

	// scanRequest := scan.ScanRequest{
	// 	BaseOrg:     "falcosecurity",
	// 	BaseRepo:    "libs",
	// 	ForkOrg:     "draios",
	// 	ForkRepo:    "agent-libs",
	// 	ForkHeadRef: "87ee9be09f61acf61f0e6f38e1458419c969b916",
	// }

	// scan, err := scan.Scan(context.Background(), client, &scanRequest)
	// exitOnErr(err)

	// err = sync.Sync(context.Background(), &sync.SyncRequest{
	// 	Scan:        scanRequest,
	// 	ScanRes:     scan,
	// 	BaseHeadRef: "master",
	// 	SyncBranch:  "oss-sync-master-test",
	// })
	// exitOnErr(err)

	git := utils.NewGitHelper()
	err := branchdb.Pull(
		git,
		"origin",
		fmt.Sprintf("%s-rerere-cache", utils.ProjectName),
		"/home/ubuntu/dev/jasondellaluce/test.txt",
		false,
	)
	exitOnErr(err)

}
