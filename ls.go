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

	tw := tabwriter.NewWriter(stdout, 2, 4, 2, ' ', tabwriter.StripEscape)
	fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
		colorCell(actionColor, "NAME"),
		colorCell(actionColor, "BRANCH"),
		colorCell(actionColor, "DIRTY"),
		colorCell(actionColor, "AHEAD"),
		colorCell(actionColor, "BEHIND"),
		colorCell(actionColor, "PERSISTENT"),
	)
	for _, w := range worktrees {
		name := w.Branch
		if w.IsMain {
			name = colorCell(colorBold+colorGreen, w.Branch+" (main)")
		} else {
			name = colorCell(colorCyan, name)
		}
		branch := w.Branch
		if branch == "" {
			branch = colorCell(colorRed, "(detached)")
		}

		dirty := ""
		if d, _, err := isDirty(w.Path); err == nil && d {
			dirty = colorCell(colorYellow, "dirty")
		}

		ahead, behind := "-", "-"
		if !w.IsMain && w.Branch != "" && w.Branch != trunk {
			a, b, err := aheadBehind(mainCheckout, trunk, w.Branch)
			if err == nil {
				ahead, behind = fmt.Sprintf("%d", a), fmt.Sprintf("%d", b)
				if a > 0 {
					ahead = colorCell(colorGreen, ahead)
				}
				if b > 0 {
					behind = colorCell(colorRed, behind)
				}
			}
		}

		persistent := ""
		if isPersistent(mainCheckout, w) {
			persistent = colorCell(colorMagenta, "persistent")
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
