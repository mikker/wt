package main

import (
	_ "embed"
	"fmt"
)

//go:embed embedded/shim.zsh
var shimZsh string

// cmdShellenv implements `wt shellenv [zsh]`: prints the shell wrapper
// function for `eval "$(wt shellenv zsh)"`. Defaults to zsh when no shell is
// given.
func cmdShellenv(args []string) int {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Fprintln(stdout, "usage: wt shellenv [zsh]")
			return 0
		}
	}
	if len(args) > 1 {
		fmt.Fprintln(stderr, "wt shellenv: too many arguments. usage: wt shellenv [zsh]")
		return 2
	}

	shell := "zsh"
	if len(args) == 1 {
		shell = args[0]
	}

	switch shell {
	case "zsh":
		fmt.Fprint(stdout, shimZsh)
		return 0
	default:
		fmt.Fprintf(stderr, "wt shellenv: unsupported shell %q; only zsh is supported for now.\n", shell)
		return 2
	}
}
