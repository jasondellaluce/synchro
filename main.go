package main

import (
	"context"
	"fmt"
	"os"

	"github.com/google/go-github/v56/github"
	"github.com/jasondellaluce/synchross/pkg/synchross/scan"
	"github.com/sirupsen/logrus"
)

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
	scan, err := scan.Scan(context.Background(), client, &scan.ScanRequest{
		BaseOrg:     "falcosecurity",
		BaseRepo:    "libs",
		ForkOrg:     "draios",
		ForkRepo:    "agent-libs",
		ForkHeadRef: "dev",
	})
	exitOnErr(err)

	for _, c := range scan {
		println(fmt.Sprintf("git cherry-pick %s # %s", c.SHA(), c.Title()))
	}

}
