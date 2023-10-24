package main

import (
	"context"
	"fmt"
	"os"

	"github.com/google/go-github/v56/github"
	"github.com/jasondellaluce/synchross/pkg/scan"
	"github.com/jasondellaluce/synchross/pkg/sync"
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
	client := github.NewClient(nil)
	token := os.Getenv("GITHUB_TOKEN")
	if len(token) > 0 {
		client = client.WithAuthToken(token)
	} else {
		logrus.Warn("the GITHUB_TOKEN env variable is not set, you may encounter authentication or rate limiting issues")
	}

	scanRequest := scan.ScanRequest{
		BaseOrg:     "falcosecurity",
		BaseRepo:    "libs",
		ForkOrg:     "draios",
		ForkRepo:    "agent-libs",
		ForkHeadRef: "dev",
	}

	scan, err := scan.Scan(context.Background(), client, &scanRequest)
	exitOnErr(err)

	err = sync.Sync(context.Background(), &sync.SyncRequest{
		Scan:        scanRequest,
		ScanRes:     scan,
		BaseHeadRef: "master",
		SyncBranch:  "oss-sync-master-test",
	})
	exitOnErr(err)
}
