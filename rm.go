package main

import (
	"bufio"
	"fmt"
	"strings"
)

// cmdRm implements `wt rm [<name>] [--force]`.
func cmdRm(args []string) int {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Fprintln(stdout, "usage: wt rm [<name>] [--force]")
			return 0
		}
	}

	positional, flags, unknown := splitFlags(args, "force")
	if len(unknown) > 0 {
		fmt.Fprintf(stderr, "wt rm: unknown flag %q. usage: wt rm [<name>] [--force]\n", unknown[0])
		return 2
	}
	if len(positional) > 1 {
		fmt.Fprintf(stderr, "wt rm: too many arguments: %s. usage: wt rm [<name>] [--force]\n", strings.Join(positional[1:], " "))
		return 2
	}
	var name string
	if len(positional) == 1 {
		name = positional[0]
	}
	force := flags["force"]

	worktrees, cwd, code := loadWorktrees("wt rm")
	if code != 0 {
		return code
	}
	mainCheckout := worktrees[0].Path

	// Trunk must be resolvable: wt never drops the trunk, and without knowing
	// which branch that is we can't promise the target isn't it.
	trunk, err := resolveTrunk(mainCheckout)
	if err != nil {
		fmt.Fprintf(stderr, "wt rm: %v\nRefusing to remove anything while the trunk is unknown; use `git worktree remove` directly if you must.\n", err)
		return 2
	}
	// The trunk worktree itself may legitimately not exist (trunk not checked
	// out anywhere); it's only used to pick the branch-delete directory.
	trunkWorktree, _ := findTrunkWorktree(worktrees, trunk)

	target, err := resolveRmTarget(worktrees, cwd, name, trunk)
	if err != nil {
		fmt.Fprintf(stderr, "wt rm: %v\n", err)
		return 2
	}

	dirty, dirt, err := isDirty(target.Path)
	if err != nil {
		fmt.Fprintf(stderr, "wt rm: attempted `git status --porcelain` in %s, git refused: %v\n", target.Path, err)
		return 2
	}
	if dirty && !force {
		fmt.Fprintf(stderr, "wt rm: %s has uncommitted changes, refusing to remove:\n%s\nCommit or stash the changes, or re-run `wt rm %s --force` to discard them.\n", target.Path, dirt, target.Branch)
		return 1
	}

	if !confirm(fmt.Sprintf("Remove worktree %s (branch %s)? [y/N] ", target.Path, target.Branch)) {
		fmt.Fprintln(stderr, "wt rm: aborted.")
		return 1
	}

	runDestroyHook(target.Path)

	if err := removeWorktree(mainCheckout, *target, trunk, force); err != nil {
		fmt.Fprintf(stderr, "wt rm: attempted `git worktree remove %s`, refused: %v. Check `git worktree list`, resolve manually, or re-run with --force.\n", target.Path, err)
		return 2
	}

	if target.Branch != "" {
		// git branch -d judges merged-ness against the HEAD of the directory
		// it runs in, so run it in the trunk worktree (falling back to the
		// main checkout when trunk isn't checked out anywhere) rather than
		// mainCheckout, which may be parked on some other branch entirely.
		branchDeleteDir := mainCheckout
		if trunkWorktree != nil {
			branchDeleteDir = trunkWorktree.Path
		}
		if err := deleteBranch(branchDeleteDir, target.Branch, trunk); err != nil {
			fmt.Fprintf(stderr, "wt rm: worktree removed, but `git branch -d %s` refused (likely unmerged commits): %v. Delete manually with `git branch -D %s` if you're sure.\n", target.Branch, err, target.Branch)
		}
	}

	if inside := currentWorktree(worktrees, cwd); inside != nil && inside.Path == target.Path {
		enterWorktree(worktrees[0])
	}

	return 0
}

// resolveRmTarget picks the worktree to remove: explicit name, else the
// worktree cwd is inside (if not trunk), else an fzf/prompt pick among
// non-trunk worktrees. The trunk worktree — the main checkout, or whichever
// linked worktree has trunk checked out — is never a valid target.
func resolveRmTarget(worktrees []Worktree, cwd, name, trunk string) (*Worktree, error) {
	if name != "" {
		w := findWorktreeByName(worktrees, name)
		if w == nil {
			return nil, fmt.Errorf("no worktree named %q; run `git worktree list` to see existing worktrees", name)
		}
		if isTrunkWorktree(*w, trunk) {
			return nil, fmt.Errorf("%q is the trunk worktree and can't be removed with `wt rm`", name)
		}
		return w, nil
	}

	if cur := currentWorktree(worktrees, cwd); cur != nil && !isTrunkWorktree(*cur, trunk) {
		return cur, nil
	}

	var pickable []Worktree
	for _, w := range worktrees {
		if !isTrunkWorktree(w, trunk) {
			pickable = append(pickable, w)
		}
	}
	if len(pickable) == 0 {
		return nil, fmt.Errorf("no non-trunk worktrees to remove")
	}
	return pickWorktree(pickable)
}

// confirm prompts on stderr and reads a y/n answer from stdin. Anything but
// y/yes (case-insensitive) is treated as "no". It wraps stdin in a fresh
// bufio.Reader, so it's only correct for a single confirm() per command
// invocation; callers that need to ask more than one question in a row must
// share one reader across the questions (see confirmWith), otherwise
// whatever the first bufio.Reader buffered past the first line is lost when
// it's discarded.
func confirm(prompt string) bool {
	return confirmWith(bufio.NewReader(stdin), prompt)
}

// confirmWith is like confirm but reads from a caller-supplied reader, so
// multiple prompts in one command can share it instead of each discarding
// whatever the previous bufio.Reader had already buffered.
func confirmWith(reader *bufio.Reader, prompt string) bool {
	fmt.Fprint(stderr, prompt)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return false
	}
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes"
}
