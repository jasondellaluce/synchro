package utils

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
	"go.uber.org/multierr"
)

type GitHelper interface {
	// Essentials
	// Pull(remote, branch string)
	// Push(remote, branch string)
	// Reset()
	// ResetHard()
	// Add(file string)
	// AddAll()
	// Commit(msg string)
	// Checkout(branch string)
	// CheckoutRemote(branch string, remoteBranch string)
	// AddRemote(remote string)
	// RemoveRemote(remote string)
	// FetchRemote(remote string)
	// FetchPrune(remote string)
	// CherryPick(commit string)
	// CherryPickContinue()
	// CherryPickAbort()
	// DeleteBranch() string
	Do(commands ...string) error
	DoOutput(commands ...string) (string, error)
	HasLocalChanges(filters ...func(string) bool) (bool, error)
	ListUnmergedFiles() ([]string, error)
	GetCurrentBranch() (string, error)
	GetRemoteDefaultBranch(remote string) (string, error)
	BranchExistsInRemote(remote, branch string) (bool, error)
	GetRepoRootDir() (string, error)
}

func NewGitHelper() GitHelper {
	return &gitHelper{}
}

type gitHelper struct{}

func (g *gitHelper) Do(commands ...string) error {
	_, err := g.DoOutput(commands...)
	return err
}

func (g *gitHelper) DoOutput(commands ...string) (string, error) {
	if len(commands) < 1 {
		return "", fmt.Errorf("attempted executing empty git command")
	}
	logrus.Debug("git " + strings.Join(commands, " "))
	outBytes, err := exec.Command("git", commands...).CombinedOutput()
	out := strings.TrimSpace(string(outBytes))
	logrus.Debug(out)
	return strings.TrimSpace(string(out)), err
}

func (g *gitHelper) HasLocalChanges(filters ...func(string) bool) (bool, error) {
	out, err := g.DoOutput("status", "--porcelain")
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(out, "\n") {
		if len(out) == 0 {
			continue
		}
		filtered := false
		for _, f := range filters {
			if !f(line) {
				filtered = true
				break
			}
		}
		if !filtered {
			return true, nil
		}
	}
	return false, nil
}

func (g *gitHelper) ListUnmergedFiles() ([]string, error) {
	out, err := g.DoOutput("diff", "--name-only", "--diff-filter=U", "--relative")
	if err != nil {
		if len(out) > 0 {
			err = multierr.Append(err, errors.New(out))
		}
		return nil, err
	}
	var res []string
	for _, f := range strings.Split(out, "\n") {
		if len(f) > 0 {
			res = append(res, f)
		}
	}
	return res, nil
}

func (g *gitHelper) GetCurrentBranch() (string, error) {
	out, err := g.DoOutput("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	if len(out) == 0 {
		return "", fmt.Errorf("can't retrieve current branch")
	}
	return out, nil
}

func (g *gitHelper) GetRemoteDefaultBranch(remote string) (string, error) {
	refs := fmt.Sprintf("refs/remotes/%s/HEAD", remote)
	out, err := g.DoOutput("symbolic-ref", refs, "--short")
	if err != nil {
		return "", err
	}
	if len(out) == 0 {
		return "", fmt.Errorf("can't retrieve default branch for remote '%s'", remote)
	}
	return strings.TrimPrefix(out, remote+"/"), nil
}

func (g *gitHelper) BranchExistsInRemote(remote, branch string) (bool, error) {
	out, err := g.DoOutput("ls-remote", "--heads", remote, fmt.Sprintf("refs/heads/%s", branch))
	if err != nil {
		return false, err
	}
	return len(out) != 0, nil
}

func (g *gitHelper) GetRepoRootDir() (string, error) {
	out, err := g.DoOutput("rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return out, nil
}
