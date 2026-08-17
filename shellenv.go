package main

import (
	_ "embed"
	"fmt"
)

//go:embed embedded/shim.zsh
var shimZsh string

//go:embed embedded/shim.bash
var shimBash string

const shellenvUsage = "usage: wt shellenv [zsh|bash]"

// cmdShellenv prints the requested shell wrapper. It defaults to zsh for
// compatibility with the original shell integration.
func cmdShellenv(args []string) int {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Fprintln(stdout, shellenvUsage)
			return 0
		}
	}
	if len(args) > 1 {
		fmt.Fprintf(stderr, "wt shellenv: too many arguments. %s\n", shellenvUsage)
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
	case "bash":
		fmt.Fprint(stdout, shimBash)
		return 0
	default:
		fmt.Fprintf(stderr, "wt shellenv: unsupported shell %q; supported shells are zsh and bash.\n", shell)
		return 2
	}
}
