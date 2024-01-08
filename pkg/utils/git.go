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
	GetRemotes() (map[string]string, error)
	TagExists(tag string) (bool, error)
	BranchExists(branch string) (bool, error)
}

type cmdExecutor interface {
	exec(cmd string, args ...string) (string, error)
}

type execCmdExecutor struct{}

func (g *execCmdExecutor) exec(cmd string, args ...string) (string, error) {
	outBytes, err := exec.Command(cmd, args...).CombinedOutput()
	return strings.TrimSpace(string(outBytes)), err
}

func NewGitHelper() GitHelper {
	return &gitHelper{e: &execCmdExecutor{}}
}

type gitHelper struct {
	e cmdExecutor
}

func (g *gitHelper) DoOutput(commands ...string) (string, error) {
	if len(commands) < 1 {
		return "", fmt.Errorf("attempted executing empty git command")
	}
	logrus.Debug("git " + strings.Join(commands, " "))
	out, err := g.e.exec("git", commands...)
	logrus.Debug(out)
	return out, err
}

func (g *gitHelper) Do(commands ...string) error {
	_, err := g.DoOutput(commands...)
	return err
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

func (g *gitHelper) GetRemotes() (map[string]string, error) {
	out, err := g.DoOutput("remote", "-v")
	if err != nil {
		return nil, err
	}
	res := make(map[string]string)
	for _, l := range strings.Split(out, "\n") {
		if len(l) == 0 {
			continue
		}
		tokens := strings.Fields(l)
		if len(tokens) < 2 {
			return nil, fmt.Errorf("can't parse result of `git remote -v` in line: %s", l)
		}
		res[tokens[0]] = tokens[1]
	}
	return res, nil
}

func (g *gitHelper) TagExists(tag string) (bool, error) {
	out, err := g.DoOutput("tag", "-l", tag)
	if err != nil {
		return false, err
	}
	return len(out) > 0, nil
}

func (g *gitHelper) BranchExists(branch string) (bool, error) {
	err := g.Do("show-ref", "--verify", "refs/heads/"+branch)
	return err == nil, nil
}
