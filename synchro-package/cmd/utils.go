package cmd

import (
	"os"

	"github.com/google/go-github/v56/github"
	"github.com/sirupsen/logrus"
)

func getGithubClient() *github.Client {
	client := github.NewClient(nil)
	token := os.Getenv("GITHUB_TOKEN")
	if len(token) > 0 {
		client = client.WithAuthToken(token)
	} else {
		logrus.Warn("the GITHUB_TOKEN env variable is not set, you may encounter authentication or rate limiting issues")
	}
	return client
}
