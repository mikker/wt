package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// cmdSwitch implements `wt switch [<name>] [--persist]` and, with no name,
// the fzf pick that bare `wt` also uses.
func cmdSwitch(args []string) int {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Fprintln(stdout, "usage: wt switch [<name>] [--persist]")
			return 0
		}
	}

	positional, flags, unknown := splitFlags(args, "persist")
	if len(unknown) > 0 {
		fmt.Fprintf(stderr, "wt switch: unknown flag %q. usage: wt switch [<name>] [--persist]\n", unknown[0])
		return 2
	}
	if len(positional) > 1 {
		fmt.Fprintf(stderr, "wt switch: too many arguments: %s. usage: wt switch [<name>] [--persist]\n", strings.Join(positional[1:], " "))
		return 2
	}
	var name string
	if len(positional) == 1 {
		name = positional[0]
	}
	persist := flags["persist"]

	worktrees, _, code := loadWorktrees("wt switch")
	if code != 0 {
		return code
	}
	mainCheckout := worktrees[0].Path

	if name == "" {
		selected, err := pickWorktree(worktrees)
		if err != nil {
			fmt.Fprintf(stderr, "wt switch: %v\n", err)
			return 2
		}
		enterWorktree(*selected)
		return 0
	}

	if existing := findWorktreeByName(worktrees, name); existing != nil {
		enterWorktree(*existing)
		return 0
	}

	trunk, err := resolveTrunk(mainCheckout)
	if err != nil {
		fmt.Fprintf(stderr, "wt switch: %v\n", err)
		return 2
	}

	wtPath := worktreePath(mainCheckout, name)
	if branchExists(mainCheckout, name) {
		if _, err := runGit(mainCheckout, "worktree", "add", wtPath, name); err != nil {
			fmt.Fprintf(stderr, "wt switch: attempted `git worktree add %s %s` (existing branch), git refused: %v. Check `git worktree list` and `git branch` for conflicts, then retry.\n", wtPath, name, err)
			return 2
		}
	} else {
		if !branchExists(mainCheckout, trunk) {
			// trunk came from refs/remotes/origin/HEAD, but no local branch
			// of that name exists. `git worktree add <path> -b <name>
			// <trunk>` would fail to resolve <trunk> as a local ref and
			// DWIM to the remote-tracking branch instead, silently ignoring
			// -b and leaving the new worktree on a branch named <trunk>,
			// not <name>.
			fmt.Fprintf(stderr, "wt switch: trunk %q comes from origin/HEAD but no local branch exists; create it with `git branch %s origin/%s` and retry.\n", trunk, trunk, trunk)
			return 2
		}
		if _, err := runGit(mainCheckout, "worktree", "add", wtPath, "-b", name, trunk); err != nil {
			fmt.Fprintf(stderr, "wt switch: attempted `git worktree add %s -b %s %s`, git refused: %v. Check `git worktree list` and `git branch` for conflicts, then retry.\n", wtPath, name, trunk, err)
			return 2
		}
	}

	if persist {
		key := "branch." + name + ".wt-persist"
		if err := setGitConfig(mainCheckout, key, "true"); err != nil {
			fmt.Fprintf(stderr, "wt switch: worktree created, but `git config %s true` failed: %v. Run it manually, or `wt persist` later.\n", key, err)
		}
	}

	if err := runCreateHook(wtPath, mainCheckout); err != nil {
		fmt.Fprintf(stderr, "wt switch: attempted `.wt/create %s` in %s, it failed: %v. The worktree was left in place; fix the issue and re-run `.wt/create %s` manually from %s, or `wt rm %s` to abandon it.\n", mainCheckout, wtPath, err, mainCheckout, wtPath, name)
		emitCd(wtPath)
		emitExportWorktree(name)
		return 1
	}

	emitCd(wtPath)
	emitExportWorktree(name)
	return 0
}

// enterWorktree cds into w, exporting or unsetting WORKTREE as appropriate.
func enterWorktree(w Worktree) {
	emitCd(w.Path)
	if w.IsMain {
		emitUnsetWorktree()
	} else {
		emitExportWorktree(w.Branch)
	}
}

// findWorktreeByName finds a worktree by its checked-out branch name, which
// is the worktree "name" by convention (branch name == worktree name).
func findWorktreeByName(worktrees []Worktree, name string) *Worktree {
	for i := range worktrees {
		if worktrees[i].Branch == name {
			return &worktrees[i]
		}
	}
	return nil
}

// pickWorktree lets the user pick a worktree via fzf, or a numbered stdin
// prompt if fzf isn't on PATH.
func pickWorktree(worktrees []Worktree) (*Worktree, error) {
	if len(worktrees) == 0 {
		return nil, fmt.Errorf("no worktrees found")
	}
	if _, err := exec.LookPath("fzf"); err == nil {
		return pickWithFzf(worktrees)
	}
	return pickWithPrompt(worktrees)
}

func pickWithFzf(worktrees []Worktree) (*Worktree, error) {
	var input strings.Builder
	for _, w := range worktrees {
		fmt.Fprintln(&input, formatPickLine(w))
	}
	cmd := exec.Command("fzf")
	cmd.Stdin = strings.NewReader(input.String())
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("fzf selection cancelled or failed: %w", err)
	}
	return matchPick(worktrees, strings.TrimSpace(string(out)))
}

func pickWithPrompt(worktrees []Worktree) (*Worktree, error) {
	for i, w := range worktrees {
		fmt.Fprintf(stderr, "%d) %s\n", i+1, formatPickLine(w))
	}
	fmt.Fprint(stderr, "Select a worktree: ")
	reader := bufio.NewReader(stdin)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return nil, fmt.Errorf("failed to read selection: %w", err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || n < 1 || n > len(worktrees) {
		return nil, fmt.Errorf("invalid selection %q: enter a number between 1 and %d", strings.TrimSpace(line), len(worktrees))
	}
	return &worktrees[n-1], nil
}

func formatPickLine(w Worktree) string {
	branch := w.Branch
	if branch == "" {
		branch = "(detached)"
	}
	return w.Path + "\t" + branch
}

func matchPick(worktrees []Worktree, line string) (*Worktree, error) {
	path := strings.SplitN(line, "\t", 2)[0]
	for i := range worktrees {
		if worktrees[i].Path == path {
			return &worktrees[i], nil
		}
	}
	return nil, fmt.Errorf("selection %q did not match a known worktree", line)
}
