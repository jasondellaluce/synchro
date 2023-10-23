package utils

import (
	"fmt"
	"regexp"
	"strconv"
)

type PullRequestLink struct {
	Num  int
	Link string
}

// SearchPullRequestLinks searches in the text a reference to a pull request
// for the given organziation and repositury. Return 0 in case no reference
// is found, and the number of the pull request otherwise.
func SearchPullRequestLinks(org, repo, text string) ([]*PullRequestLink, error) {
	var res []*PullRequestLink

	var PullRequestLinkInTextStyles = []*regexp.Regexp{
		regexp.MustCompile(fmt.Sprintf(`%s/%s#(\d+)`, org, repo)),
		regexp.MustCompile(fmt.Sprintf(`github.com/%s/%s/pull/(\d+)`, org, repo)),
		regexp.MustCompile(fmt.Sprintf(`\[%s#(\d+)\]`, org)),
	}

	for _, s := range PullRequestLinkInTextStyles {
		matches := s.FindAllStringSubmatch(text, -1)
		for _, m := range matches {
			if len(m) == 2 {
				num, err := strconv.Atoi(m[1])
				if err != nil {
					return nil, err
				}
				res = append(res, &PullRequestLink{Num: num, Link: m[0]})
			}
		}
	}

	return res, nil
}

func ReverseSlice[S ~[]E, E any](s S) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
