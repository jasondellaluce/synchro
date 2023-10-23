package utils

import "github.com/google/go-github/v56/github"

func IterateGithubPages[T interface{}](f func(*github.ListOptions) ([]*T, *github.Response, error)) ([]*T, error) {
	var res []*T
	options := github.ListOptions{Page: 1, PerPage: 100}
	for {
		vals, _, err := f(&options)
		if err != nil {
			return nil, err
		}
		res = append(res, vals...)
		if len(vals) < options.PerPage {
			return res, nil
		}
		options.Page++
	}
}
