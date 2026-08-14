package main

import (
	"fmt"
	"text/tabwriter"
)

// cmdLs implements `wt ls`.
func cmdLs(args []string) int {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Fprintln(stdout, "usage: wt ls")
			return 0
		}
	}
	if len(args) > 0 {
		fmt.Fprintf(stderr, "wt ls: unexpected argument %q. usage: wt ls\n", args[0])
		return 2
	}

	worktrees, _, code := loadWorktrees("wt ls")
	if code != 0 {
		return code
	}
	mainCheckout := worktrees[0].Path

	trunk, err := resolveTrunk(mainCheckout)
	if err != nil {
		fmt.Fprintf(stderr, "wt ls: %v\n", err)
		return 2
	}

	tw := tabwriter.NewWriter(stdout, 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tBRANCH\tDIRTY\tAHEAD\tBEHIND\tPERSISTENT")
	for _, w := range worktrees {
		name := w.Branch
		if w.IsMain {
			name = w.Branch + " (main)"
		}
		branch := w.Branch
		if branch == "" {
			branch = "(detached)"
		}

		dirty := ""
		if d, _, err := isDirty(w.Path); err == nil && d {
			dirty = "dirty"
		}

		ahead, behind := "-", "-"
		if !w.IsMain && w.Branch != "" && w.Branch != trunk {
			a, b, err := aheadBehind(mainCheckout, trunk, w.Branch)
			if err == nil {
				ahead, behind = fmt.Sprintf("%d", a), fmt.Sprintf("%d", b)
			}
		}

		persistent := ""
		if isPersistent(mainCheckout, w) {
			persistent = "persistent"
		}

		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", name, branch, dirty, ahead, behind, persistent)
	}
	tw.Flush()
	return 0
}

// isPersistent reports effective persistence for display: branch config or
// project config.
func isPersistent(mainCheckout string, w Worktree) bool {
	if w.Branch != "" && gitConfigBool(mainCheckout, "branch."+w.Branch+".wt-persist") {
		return true
	}
	return projectPersistent(w.Path)
}
