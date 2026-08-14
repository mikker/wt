package main

import (
	"fmt"
)

// cmdPersist implements `wt persist`: toggle branch.<name>.wt-persist for
// the current worktree's branch and print the new state.
func cmdPersist(args []string) int {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Fprintln(stdout, "usage: wt persist")
			return 0
		}
	}
	if len(args) > 0 {
		fmt.Fprintf(stderr, "wt persist: unexpected argument %q. usage: wt persist\n", args[0])
		return 2
	}

	worktrees, cwd, code := loadWorktrees("wt persist")
	if code != 0 {
		return code
	}
	mainCheckout := worktrees[0].Path

	cur := currentWorktree(worktrees, cwd)
	if cur == nil {
		fmt.Fprintln(stderr, "wt persist: not inside a worktree; cd into the worktree whose branch you want to toggle, or `wt switch <name>` into one.")
		return 2
	}
	if cur.IsMain {
		fmt.Fprintln(stderr, "wt persist: this is the trunk worktree; persistence only applies to feature worktrees created with `wt switch`.")
		return 2
	}

	key := "branch." + cur.Branch + ".wt-persist"
	newState := !gitConfigBool(mainCheckout, key)
	if err := setGitConfig(mainCheckout, key, fmt.Sprintf("%t", newState)); err != nil {
		fmt.Fprintf(stderr, "wt persist: attempted `git config %s %t`, git refused: %v\n", key, newState, err)
		return 2
	}

	if newState {
		fmt.Fprintf(stdout, "%s is now persistent (wt done/ship will rebase in place instead of tearing down; override with --rm)\n", cur.Branch)
	} else {
		fmt.Fprintf(stdout, "%s is now ephemeral (wt done/ship will tear it down; override with --keep)\n", cur.Branch)
	}
	return 0
}
