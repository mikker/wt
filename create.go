package main

import (
	"fmt"
	"strings"
)

// cmdCreate implements `wt create <name> [--persist]`.
func cmdCreate(args []string) int {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Fprintln(stdout, "usage: wt create <name> [--persist]")
			return 0
		}
	}

	positional, flags, unknown := splitFlags(args, "persist")
	if len(unknown) > 0 {
		fmt.Fprintf(stderr, "wt create: unknown flag %q. usage: wt create <name> [--persist]\n", unknown[0])
		return 2
	}
	if len(positional) == 0 {
		fmt.Fprintln(stderr, "wt create: name is required. usage: wt create <name> [--persist]")
		return 2
	}
	if len(positional) > 1 {
		fmt.Fprintf(stderr, "wt create: too many arguments: %s. usage: wt create <name> [--persist]\n", strings.Join(positional[1:], " "))
		return 2
	}

	name := positional[0]
	worktrees, _, code := loadWorktrees("wt create")
	if code != 0 {
		return code
	}
	if findWorktreeByName(worktrees, name) != nil {
		fmt.Fprintf(stderr, "wt create: worktree %q already exists; enter it with `wt switch %s`.\n", name, name)
		return 2
	}

	mainCheckout := worktrees[0].Path
	trunk, err := resolveTrunk(mainCheckout)
	if err != nil {
		fmt.Fprintf(stderr, "wt create: %v\n", err)
		return 2
	}

	wtPath := worktreePath(mainCheckout, name)
	if branchExists(mainCheckout, name) {
		if _, err := runGit(mainCheckout, "worktree", "add", wtPath, name); err != nil {
			fmt.Fprintf(stderr, "wt create: attempted `git worktree add %s %s` (existing branch), git refused: %v. Check `git worktree list` and `git branch` for conflicts, then retry.\n", wtPath, name, err)
			return 2
		}
	} else {
		if !branchExists(mainCheckout, trunk) {
			// Avoid Git's DWIM behavior silently checking out the remote trunk
			// instead of creating the requested branch.
			fmt.Fprintf(stderr, "wt create: trunk %q comes from origin/HEAD but no local branch exists; create it with `git branch %s origin/%s` and retry.\n", trunk, trunk, trunk)
			return 2
		}
		if _, err := runGit(mainCheckout, "worktree", "add", wtPath, "-b", name, trunk); err != nil {
			fmt.Fprintf(stderr, "wt create: attempted `git worktree add %s -b %s %s`, git refused: %v. Check `git worktree list` and `git branch` for conflicts, then retry.\n", wtPath, name, trunk, err)
			return 2
		}
	}

	if flags["persist"] {
		key := "branch." + name + ".wt-persist"
		if err := setGitConfig(mainCheckout, key, "true"); err != nil {
			fmt.Fprintf(stderr, "wt create: worktree created, but `git config %s true` failed: %v. Run it manually, or `wt persist` later.\n", key, err)
		}
	}

	if err := runCreateHook(wtPath, mainCheckout); err != nil {
		fmt.Fprintf(stderr, "wt create: attempted `.wt/create %s` in %s, it failed: %v. The worktree was left in place; fix the issue and re-run `.wt/create %s` manually from %s, or `wt rm %s` to abandon it.\n", mainCheckout, wtPath, err, mainCheckout, wtPath, name)
		emitCd(wtPath)
		emitExportWorktree(name)
		return 1
	}

	emitCd(wtPath)
	emitExportWorktree(name)
	return 0
}
