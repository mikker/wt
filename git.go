package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// gitError wraps a failed git invocation with enough context for callers to
// build agent-facing error messages.
type gitError struct {
	args   []string
	dir    string
	stderr string
	err    error
}

func (e *gitError) Error() string {
	msg := strings.TrimSpace(e.stderr)
	if msg == "" {
		msg = e.err.Error()
	}
	return fmt.Sprintf("git %s (in %s): %s", strings.Join(e.args, " "), e.dir, msg)
}

func (e *gitError) Unwrap() error { return e.err }

// runGit runs git in dir and returns trimmed stdout. On failure the error is
// a *gitError carrying stderr for good error messages.
func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", &gitError{args: args, dir: dir, stderr: stderr.String(), err: err}
	}
	return strings.TrimSpace(stdout.String()), nil
}

// Worktree describes one entry from `git worktree list --porcelain`.
type Worktree struct {
	Path     string
	Branch   string // short branch name, e.g. "main"; empty if detached
	Head     string
	IsMain   bool // true for the main checkout (first worktree listed)
	Detached bool
}

// listWorktrees returns all worktrees for the repo containing dir. The
// first entry is always the main checkout, per git's own convention.
func listWorktrees(dir string) ([]Worktree, error) {
	out, err := runGit(dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return parseWorktrees(out), nil
}

func parseWorktrees(porcelain string) []Worktree {
	var worktrees []Worktree
	var cur *Worktree
	for _, line := range strings.Split(porcelain, "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case line == "":
			cur = nil
		case strings.HasPrefix(line, "worktree "):
			worktrees = append(worktrees, Worktree{Path: strings.TrimPrefix(line, "worktree ")})
			cur = &worktrees[len(worktrees)-1]
		case strings.HasPrefix(line, "HEAD "):
			if cur != nil {
				cur.Head = strings.TrimPrefix(line, "HEAD ")
			}
		case strings.HasPrefix(line, "branch "):
			if cur != nil {
				cur.Branch = strings.TrimPrefix(strings.TrimPrefix(line, "branch "), "refs/heads/")
			}
		case line == "detached":
			if cur != nil {
				cur.Detached = true
			}
		}
	}
	if len(worktrees) > 0 {
		worktrees[0].IsMain = true
	}
	return worktrees
}

// loadWorktrees resolves cwd and lists the repo's worktrees — the preamble
// every command shares. Errors are printed to stderr prefixed with cmdName;
// running outside any git repository gets a short friendly message instead
// of raw git stderr. code is 0 on success, 2 on failure.
func loadWorktrees(cmdName string) (worktrees []Worktree, cwd string, code int) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "%s: could not determine current directory: %v\n", cmdName, err)
		return nil, "", 2
	}
	worktrees, err = listWorktrees(cwd)
	if err != nil {
		var ge *gitError
		if errors.As(err, &ge) && strings.Contains(ge.stderr, "not a git repository") {
			fmt.Fprintf(stderr, "%s: not in a git repository. wt manages a repo's worktrees — cd into a project first.\n", cmdName)
		} else {
			fmt.Fprintf(stderr, "%s: attempted to list worktrees from %s, git refused: %v\n", cmdName, cwd, err)
		}
		return nil, "", 2
	}
	return worktrees, cwd, 0
}

// currentWorktree returns the worktree containing cwd, or nil if cwd isn't
// inside any of them.
func currentWorktree(worktrees []Worktree, cwd string) *Worktree {
	cwd = resolveSymlinks(cwd)
	var best *Worktree
	var bestResolved string
	for i := range worktrees {
		p := resolveSymlinks(worktrees[i].Path)
		if cwd == p || strings.HasPrefix(cwd, p+string(os.PathSeparator)) {
			if best == nil || len(p) > len(bestResolved) {
				best = &worktrees[i]
				bestResolved = p
			}
		}
	}
	return best
}

func resolveSymlinks(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

// resolveTrunk determines the trunk branch name: refs/remotes/origin/HEAD
// if set, else local main, else local master.
func resolveTrunk(dir string) (string, error) {
	if out, err := runGit(dir, "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD"); err == nil {
		if name := strings.TrimPrefix(out, "refs/remotes/origin/"); name != out && name != "" {
			return name, nil
		}
	}
	if branchExists(dir, "main") {
		return "main", nil
	}
	if branchExists(dir, "master") {
		return "master", nil
	}
	return "", fmt.Errorf("could not resolve trunk: no refs/remotes/origin/HEAD, no local `main`, no local `master`. Set one up, e.g. `git branch main` or `git remote set-head origin -a`, and retry")
}

// branchExists reports whether a local branch named name exists.
func branchExists(dir, name string) bool {
	_, err := runGit(dir, "show-ref", "--verify", "--quiet", "refs/heads/"+name)
	return err == nil
}

// findTrunkWorktree returns the worktree with trunk checked out.
func findTrunkWorktree(worktrees []Worktree, trunk string) (*Worktree, error) {
	for i := range worktrees {
		if worktrees[i].Branch == trunk {
			return &worktrees[i], nil
		}
	}
	return nil, fmt.Errorf("no worktree has trunk branch %q checked out; run `git worktree list` to inspect", trunk)
}

// isTrunkWorktree reports whether w is the trunk worktree: either the main
// checkout, or (in a linked-trunk setup) the worktree whose checked-out
// branch is trunk. trunk may be "" if resolution failed, in which case only
// the IsMain check applies.
func isTrunkWorktree(w Worktree, trunk string) bool {
	return w.IsMain || (trunk != "" && w.Branch == trunk)
}

// deleteBranch runs `git branch -d name` (never -D) in dir. It refuses
// outright to delete the trunk branch: wt must never drop the trunk, no
// matter how the name was resolved upstream. This is a backstop — callers
// are expected to have excluded trunk already.
func deleteBranch(dir, name, trunk string) error {
	if name == trunk {
		return fmt.Errorf("refusing to delete trunk branch %q", name)
	}
	_, err := runGit(dir, "branch", "-d", name)
	return err
}

// removeWorktree runs `git worktree remove` for w. It refuses outright when
// w is the trunk worktree — same backstop as deleteBranch: wt must never
// drop the trunk.
func removeWorktree(mainCheckout string, w Worktree, trunk string, force bool) error {
	if isTrunkWorktree(w, trunk) {
		return fmt.Errorf("refusing to remove the trunk worktree %s", w.Path)
	}
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, w.Path)
	_, err := runGit(mainCheckout, args...)
	return err
}

// worktreesDir returns the sibling directory that holds this project's
// worktrees: ../<project>-worktrees relative to the main checkout.
func worktreesDir(mainCheckout string) string {
	parent := filepath.Dir(mainCheckout)
	project := filepath.Base(mainCheckout)
	return filepath.Join(parent, project+"-worktrees")
}

// worktreePath returns the conventional path for worktree name.
func worktreePath(mainCheckout, name string) string {
	return filepath.Join(worktreesDir(mainCheckout), name)
}

// isDirty reports whether dir has uncommitted changes, including untracked
// files. dirt is the raw `git status --porcelain` output for display.
func isDirty(dir string) (dirty bool, dirt string, err error) {
	out, err := runGit(dir, "status", "--porcelain")
	if err != nil {
		return false, "", err
	}
	return out != "", out, nil
}

// aheadBehind reports how many commits branch is ahead of and behind base.
func aheadBehind(dir, base, branch string) (ahead, behind int, err error) {
	out, err := runGit(dir, "rev-list", "--left-right", "--count", base+"..."+branch)
	if err != nil {
		return 0, 0, err
	}
	parts := strings.Fields(out)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unexpected `git rev-list --left-right --count` output: %q", out)
	}
	behind, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	ahead, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}
	return ahead, behind, nil
}

// gitConfigValue returns the git config value for key in dir, and whether
// it's set at all (as opposed to defaulting to "").
func gitConfigValue(dir, key string) (value string, ok bool) {
	out, err := runGit(dir, "config", "--get", key)
	if err != nil {
		return "", false
	}
	return out, true
}

// gitConfigBool reports whether git config key is set to "true" in dir.
func gitConfigBool(dir, key string) bool {
	v, ok := gitConfigValue(dir, key)
	return ok && v == "true"
}

// setGitConfig sets a git config key/value in dir.
func setGitConfig(dir, key, value string) error {
	_, err := runGit(dir, "config", key, value)
	return err
}
