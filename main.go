package main

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"strings"
)

//go:embed embedded/hooks.md
var hooksHelp string

// version is replaced by GoReleaser at build time.
var version = "dev"

// stdin/stdout/stderr are package-level so tests can redirect them without
// touching the real file descriptors (and without racing os.Stdin/Stdout,
// which fd 3 shim writes bypass entirely).
var (
	stdin  io.Reader = os.Stdin
	stdout io.Writer = os.Stdout
	stderr io.Writer = os.Stderr
)

// commands maps subcommand name to handler. Exit codes: 0 ok, 1
// blocked-but-resolvable, 2 usage/environment error.
var commands = map[string]func(args []string) int{
	"create":   cmdCreate,
	"c":        cmdCreate,
	"switch":   cmdSwitch,
	"s":        cmdSwitch,
	"rm":       cmdRm,
	"ls":       cmdLs,
	"done":     cmdDone,
	"ship":     cmdShip,
	"persist":  cmdPersist,
	"skill":    cmdSkill,
	"prompt":   cmdPrompt,
	"init":     cmdInit,
	"shellenv": cmdShellenv,
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		printUsage(stdout)
		return 0
	}

	name := args[0]
	if name == "-h" || name == "--help" {
		printUsage(stdout)
		return 0
	}
	if name == "-v" || name == "--version" {
		fmt.Fprintf(stdout, "wt %s\n", version)
		return 0
	}
	if name == "help" {
		return cmdHelp(args[1:])
	}

	cmd, ok := commands[name]
	if !ok {
		fmt.Fprintf(stderr, "wt: unknown command %q\n", name)
		printUsage(stderr)
		return 2
	}
	return cmd(args[1:])
}

// cmdHelp implements `wt help [topic]`. Bare `wt help` is the same as `wt
// --help`. Dedicated topics document hooks and lifecycle events; anything
// else is a usage error.
func cmdHelp(args []string) int {
	if len(args) == 0 {
		printUsage(stdout)
		return 0
	}
	if len(args) == 1 && args[0] == "hooks" {
		fmt.Fprint(stdout, hooksHelp)
		return 0
	}
	if len(args) == 1 && args[0] == "events" {
		fmt.Fprint(stdout, eventsHelp)
		return 0
	}
	fmt.Fprintf(stderr, "wt help: unknown help topic %q. Run `wt help` for general usage, `wt help hooks`, or `wt help events`.\n", args[0])
	return 2
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, strings.TrimLeft(`
usage: wt <command> [args]

commands:
  create <name> [--persist] [-- command [args...]]
                                create and enter a worktree (alias: c)
  switch [<name>]               enter an existing worktree (alias: s)
  rm [<name>] [--force]         remove a worktree
  ls                            list worktrees
  done [--rm|--keep]            rebase, merge to trunk, tear down (offline)
  ship [--rm|--keep]            fetch, ff trunk, done, push trunk
  persist                       toggle persistence for the current worktree
  skill [--export <skills-dir>] print (or export) the wt agent skill
  prompt                        print a project-setup prompt for an agent
  init                          scaffold .wt/ and doc pointers for this project
  shellenv [zsh]                print the shell wrapper for eval

Worktrees live at ../<project>-worktrees/<name>, one directory up from the
main checkout; a worktree's name is always its branch name. Without the
shell shim, commands that would cd instead print the destination path so you
can cd yourself.

Run "wt <command> --help" for command-specific help where available.
Run "wt help hooks" for the .wt/create, .wt/destroy, .wt/config contract.
Run "wt help events" for the WT_EVENT_HANDLER lifecycle-event contract.
Run "wt shellenv" to print the shell integration to eval in your rc file.
Run "wt --version" to print the installed version.
`, "\n"))
}

// splitFlags pulls the named boolean "--flag" args out of args, in any
// position, and returns the remaining positional args plus which flags were
// found. Any other "-"-prefixed token is returned in unknown for the caller
// to report as a usage error.
func splitFlags(args []string, names ...string) (positional []string, found map[string]bool, unknown []string) {
	found = make(map[string]bool)
	valid := make(map[string]bool, len(names))
	for _, n := range names {
		valid["--"+n] = true
	}
	for _, a := range args {
		switch {
		case valid[a]:
			found[strings.TrimPrefix(a, "--")] = true
		case strings.HasPrefix(a, "-"):
			unknown = append(unknown, a)
		default:
			positional = append(positional, a)
		}
	}
	return positional, found, unknown
}
