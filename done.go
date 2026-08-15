package main

import (
	"fmt"
)

// effectivePersistent resolves whether a worktree should be treated as
// persistent. First match wins:
//  1. --rm / --keep flag
//  2. git config branch.<name>.wt-persist (explicitly true or false)
//  3. `persistent = true` in the worktree's .wt/config
//  4. default: ephemeral
func effectivePersistent(mainCheckout, worktreeDir, branch string, rm, keep bool) bool {
	switch {
	case rm:
		return false
	case keep:
		return true
	}
	if v, ok := gitConfigValue(mainCheckout, "branch."+branch+".wt-persist"); ok {
		return v == "true"
	}
	return projectPersistent(worktreeDir)
}

// parseDoneFlags parses the --rm/--keep flags shared by `wt done` and
// `wt ship`. ok is false when the caller should return immediately with
// code (0 for -h/--help, 2 for a usage error).
func parseDoneFlags(cmdName, usage string, args []string) (rm, keep bool, code int, ok bool) {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Fprintln(stdout, usage)
			return false, false, 0, false
		}
	}
	positional, flags, unknown := splitFlags(args, "rm", "keep")
	if len(unknown) > 0 {
		fmt.Fprintf(stderr, "%s: unknown flag %q. %s\n", cmdName, unknown[0], usage)
		return false, false, 2, false
	}
	if len(positional) > 0 {
		fmt.Fprintf(stderr, "%s: unexpected argument %q. %s\n", cmdName, positional[0], usage)
		return false, false, 2, false
	}
	if flags["rm"] && flags["keep"] {
		fmt.Fprintf(stderr, "%s: --rm and --keep are mutually exclusive\n", cmdName)
		return false, false, 2, false
	}
	return flags["rm"], flags["keep"], 0, true
}

// repoCtx bundles the worktree/trunk state that doneFlow and cmdShip both
// need before doing their real work.
type repoCtx struct {
	worktrees     []Worktree
	mainCheckout  string
	trunk         string
	trunkWorktree *Worktree
	cur           *Worktree // worktree containing cwd, or nil
}

// loadRepo resolves cwd, lists worktrees, and resolves trunk and the trunk
// worktree — the preamble shared by doneFlow and cmdShip. cmdName prefixes
// error messages. code is 0 on success; on failure the caller should return
// code immediately (the error has already been printed).
func loadRepo(cmdName string) (ctx repoCtx, code int) {
	worktrees, cwd, code := loadWorktrees(cmdName)
	if code != 0 {
		return ctx, code
	}
	mainCheckout := worktrees[0].Path

	trunk, err := resolveTrunk(mainCheckout)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", cmdName, err)
		return ctx, 2
	}
	trunkWorktree, err := findTrunkWorktree(worktrees, trunk)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", cmdName, err)
		return ctx, 2
	}

	return repoCtx{
		worktrees:     worktrees,
		mainCheckout:  mainCheckout,
		trunk:         trunk,
		trunkWorktree: trunkWorktree,
		cur:           currentWorktree(worktrees, cwd),
	}, 0
}

// cmdDone implements `wt done`.
func cmdDone(args []string) int {
	rm, keep, code, ok := parseDoneFlags("wt done", "usage: wt done [--rm|--keep]", args)
	if !ok {
		return code
	}
	removal, code := doneFlow("wt done", rm, keep)
	if code == 0 && removal != nil {
		emitRemovalEvent("wt done", "done", *removal)
	}
	return code
}

// cmdShip implements `wt ship`: fetch, fast-forward local trunk to
// origin/<trunk>, run the done flow, then push trunk. The feature branch is
// never pushed.
func cmdShip(args []string) int {
	rm, keep, code, ok := parseDoneFlags("wt ship", "usage: wt ship [--rm|--keep]", args)
	if !ok {
		return code
	}

	ctx, code := loadRepo("wt ship")
	if code != 0 {
		return code
	}
	mainCheckout, trunk, trunkWorktree := ctx.mainCheckout, ctx.trunk, ctx.trunkWorktree

	if _, err := runGit(mainCheckout, "fetch", "origin", "--prune"); err != nil {
		fmt.Fprintf(stderr, "wt ship: attempted `git fetch origin --prune`, git refused: %v\nCheck your network connection and that `origin` is configured (`git remote -v`).\n", err)
		return 2
	}

	remoteRef := "origin/" + trunk
	if _, err := runGit(trunkWorktree.Path, "merge", "--ff-only", remoteRef); err != nil {
		fmt.Fprintf(stderr, "wt ship: attempted `git -C %s merge --ff-only %s`, git refused: %v\nEither local %s has diverged from %s — reconcile by hand in %s (e.g. `git rebase %s`) — or %s doesn't exist because %s has never been pushed — run `git push -u origin %s` from %s first. Then re-run `wt ship`.\n", trunkWorktree.Path, remoteRef, err, trunk, remoteRef, trunkWorktree.Path, remoteRef, remoteRef, trunk, trunk, mainCheckout)
		return 1
	}

	removal, code := doneFlow("wt ship", rm, keep)
	if code != 0 {
		return code
	}

	if _, err := runGit(mainCheckout, "push", "origin", trunk); err != nil {
		fmt.Fprintf(stderr, "wt ship: merged locally, but `git push origin %s` failed: %v\nResolve and push manually from %s.\n", trunk, err, mainCheckout)
		return 1
	}
	if removal != nil {
		emitRemovalEvent("wt ship", "ship", *removal)
	}

	return 0
}

// doneFlow implements the offline merge-and-teardown algorithm shared by
// `wt done` and, after its network bracket, `wt ship`. cmdName prefixes
// error messages so they read correctly from either caller.
func doneFlow(cmdName string, rm, keep bool) (*removalDetails, int) {
	ctx, code := loadRepo(cmdName)
	if code != 0 {
		return nil, code
	}
	mainCheckout, trunk, trunkWorktree, cur := ctx.mainCheckout, ctx.trunk, ctx.trunkWorktree, ctx.cur

	if cur == nil {
		fmt.Fprintf(stderr, "%s: must be run from a non-trunk feature worktree. `wt switch <name>` into one first.\n", cmdName)
		return nil, 1
	}
	if cur.IsMain {
		fmt.Fprintf(stderr, "%s: must be run from a non-trunk feature worktree, not the main checkout (%s): tearing down would `git worktree remove` the main checkout and run `.wt/destroy` against it. Work happens in linked worktrees — `wt switch <name>` into one, then re-run `%s` there.\n", cmdName, cur.Path, cmdName)
		return nil, 1
	}
	if cur.Path == trunkWorktree.Path {
		fmt.Fprintf(stderr, "%s: must be run from a non-trunk feature worktree (currently in %s, which has trunk %s checked out). `wt switch <name>` into one first.\n", cmdName, cur.Path, trunk)
		return nil, 1
	}
	if cur.Detached || cur.Branch == "" {
		fmt.Fprintf(stderr, "%s: %s is on a detached HEAD; checkout a branch first (e.g. `git -C %s checkout -b <name>`), then re-run `%s`.\n", cmdName, cur.Path, cur.Path, cmdName)
		return nil, 1
	}
	name := cur.Branch

	if dirty, dirt, err := isDirty(cur.Path); err != nil {
		fmt.Fprintf(stderr, "%s: attempted `git status --porcelain` in %s, git refused: %v\n", cmdName, cur.Path, err)
		return nil, 2
	} else if dirty {
		fmt.Fprintf(stderr, "%s: %s has uncommitted changes, refusing to merge:\n%s\nCommit or stash the changes, then re-run `%s`.\n", cmdName, cur.Path, dirt, cmdName)
		return nil, 1
	}

	if dirty, dirt, err := isDirty(trunkWorktree.Path); err != nil {
		fmt.Fprintf(stderr, "%s: attempted `git status --porcelain` in %s, git refused: %v\n", cmdName, trunkWorktree.Path, err)
		return nil, 2
	} else if dirty {
		fmt.Fprintf(stderr, "%s: trunk worktree %s has uncommitted changes, refusing to merge:\n%s\nCommit or stash there, then re-run `%s`.\n", cmdName, trunkWorktree.Path, dirt, cmdName)
		return nil, 1
	}

	if _, err := runGit(cur.Path, "rebase", trunk); err != nil {
		fmt.Fprintf(stderr, "%s: attempted `git rebase %s` in %s, git stopped with a conflict: %v\nResolve the conflicts, `git add` the fixed files, `git rebase --continue`, then re-run `%s`.\n", cmdName, trunk, cur.Path, err, cmdName)
		return nil, 1
	}

	if _, err := runGit(trunkWorktree.Path, "merge", "--ff-only", name); err != nil {
		fmt.Fprintf(stderr, "%s: attempted `git -C %s merge --ff-only %s`, git refused: %v\nThis shouldn't happen right after a clean rebase onto %s; inspect `git log` in both worktrees.\n", cmdName, trunkWorktree.Path, name, err, trunk)
		return nil, 1
	}

	if effectivePersistent(mainCheckout, cur.Path, name, rm, keep) {
		// The ff-only merge above already fast-forwarded trunk to the
		// feature branch's tip, so feature == trunk here; there's nothing
		// left to rebase onto.
		fmt.Fprintf(stdout, "%s: merged %s into %s; %s is persistent, staying in %s\n", cmdName, name, trunk, name, cur.Path)
		return nil, 0
	}
	removal := newRemovalDetails(mainCheckout, trunk, trunkWorktree.Path, *cur)

	runDestroyHook(cur.Path)

	if err := removeWorktree(mainCheckout, *cur, trunk, false); err != nil {
		fmt.Fprintf(stderr, "%s: merged %s into %s, but `git worktree remove %s` failed: %v\nRemove it manually, then `git branch -d %s`.\n", cmdName, name, trunk, cur.Path, err, name)
		return nil, 1
	}

	// git branch -d judges merged-ness against the HEAD of the directory it
	// runs in, so run it in the trunk worktree, not mainCheckout (which is
	// the trunk worktree in the common case, but not when trunk is checked
	// out in a linked worktree).
	if err := deleteBranch(trunkWorktree.Path, name, trunk); err != nil {
		fmt.Fprintf(stderr, "%s: merged and worktree removed, but `git branch -d %s` refused: %v. Delete manually with `git branch -D %s` if you're sure.\n", cmdName, name, err, name)
	}

	enterWorktree(*trunkWorktree)
	return &removal, 0
}
