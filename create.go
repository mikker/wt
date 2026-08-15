package main

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

const createUsage = "wt create <name> [--persist] [-- command [args...]]"

// cmdCreate implements `wt create <name> [--persist] [-- command [args...]]`.
func cmdCreate(args []string) int {
	createArgs, command := splitCreateCommand(args)
	for _, a := range createArgs {
		if a == "-h" || a == "--help" {
			fmt.Fprintln(stdout, "usage: "+createUsage)
			return 0
		}
	}

	positional, flags, unknown := splitFlags(createArgs, "persist")
	if len(unknown) > 0 {
		fmt.Fprintf(stderr, "wt create: unknown flag %q. usage: %s\n", unknown[0], createUsage)
		return 2
	}
	if len(positional) == 0 {
		fmt.Fprintln(stderr, "wt create: name is required. usage: "+createUsage)
		return 2
	}
	if len(positional) > 1 {
		fmt.Fprintf(stderr, "wt create: too many arguments: %s. usage: %s\n", strings.Join(positional[1:], " "), createUsage)
		return 2
	}
	if command != nil && len(command) == 0 {
		fmt.Fprintln(stderr, "wt create: command is required after --. usage: "+createUsage)
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

	commandCode := 0
	if command != nil {
		commandCode = runCreateCommand(wtPath, name, command)
		if !worktreeExists(mainCheckout, name, wtPath) {
			return commandCode
		}
	}
	emitCd(wtPath)
	emitExportWorktree(name)
	return commandCode
}

func splitCreateCommand(args []string) (createArgs, command []string) {
	for i, arg := range args {
		if arg == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
}

func runCreateCommand(worktreePath, name string, args []string) int {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = worktreePath
	cmd.Env = setEnv(stripEnv(cmd.Environ(), "WT_SHIM"), "WORKTREE", name)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(stderr, "wt create: failed to run %q in %s: %v\n", args[0], worktreePath, err)
		return 1
	}
	return 0
}

func worktreeExists(mainCheckout, name, path string) bool {
	worktrees, err := listWorktrees(mainCheckout)
	if err != nil {
		return false
	}
	worktree := findWorktreeByName(worktrees, name)
	return worktree != nil && resolveSymlinks(worktree.Path) == resolveSymlinks(path)
}
