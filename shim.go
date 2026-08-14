package main

import (
	"fmt"
	"os"
	"strings"
)

// The shell shim protocol.
//
// A binary can't change its parent shell's cwd, so `wt shellenv zsh` (a
// later milestone) installs a zsh function `wt()` that runs this binary
// with WT_SHIM=1 and fd 3 redirected to a pipe it reads back.
//
// When WT_SHIM=1 is set, wt writes zsh directives to fd 3, one per line,
// for the wrapper to `eval`. Directives in use:
//
//	builtin cd -- '<path>'     cd the parent shell into <path>
//	export WORKTREE='<name>'   mark the parent shell as inside worktree <name>
//	unset WORKTREE             clear that marker (e.g. back at the main checkout)
//
// Single quotes inside values are escaped POSIX-style (close quote,
// backslash-escaped quote, reopen quote) so the directive stays valid.
//
// Without WT_SHIM (bare binary, scripts, agents), commands that would cd
// instead print the target path to stdout so the caller can cd itself, and
// the WORKTREE export/unset directives are simply skipped.
const shimFD = 3

var shimFile *os.File

// shimActive reports whether wt is running under the shell shim.
func shimActive() bool {
	return os.Getenv("WT_SHIM") == "1"
}

func shimOut() *os.File {
	if shimFile == nil {
		shimFile = os.NewFile(shimFD, "shim")
	}
	return shimFile
}

// quoteZsh single-quotes s for safe interpolation into a zsh directive.
func quoteZsh(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// emitDirective writes line to fd 3 for the shell wrapper to eval, and
// reports whether the write succeeded. It can fail if WT_SHIM=1 leaked into
// an environment without the wrapper actually holding fd 3 open (e.g. a
// hook or subshell that inherited the env var but not the pipe).
func emitDirective(line string) bool {
	_, err := fmt.Fprintln(shimOut(), line)
	return err == nil
}

// emitCd cds the parent shell into path when the shim is active, otherwise
// prints path to stdout. If the shim is (supposedly) active but the fd-3
// write fails, falls back to printing path to stdout so the caller still
// learns the destination instead of silently losing it.
func emitCd(path string) {
	if shimActive() && emitDirective("builtin cd -- "+quoteZsh(path)) {
		return
	}
	fmt.Fprintln(stdout, path)
}

// emitExportWorktree exports WORKTREE=name in the parent shell. No-op
// without the shim.
func emitExportWorktree(name string) {
	if shimActive() {
		emitDirective("export WORKTREE=" + quoteZsh(name))
	}
}

// emitUnsetWorktree unsets WORKTREE in the parent shell. No-op without the
// shim.
func emitUnsetWorktree() {
	if shimActive() {
		emitDirective("unset WORKTREE")
	}
}
