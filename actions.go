package main

import "fmt"

const (
	colorRed     = "\x1b[31m"
	colorGreen   = "\x1b[32m"
	colorYellow  = "\x1b[33m"
	colorCyan    = "\x1b[36m"
	colorMagenta = "\x1b[35m"
	colorBold    = "\x1b[1m"
	actionColor  = "\x1b[1;36m"
	colorReset   = "\x1b[0m"
)

// actionHeadline makes mutating commands easy to follow. Action output goes
// to stderr so stdout remains usable for command results and the bare-binary
// worktree path protocol.
func actionHeadline(format string, args ...any) {
	fmt.Fprintf(stderr, actionColor+"==> "+format+colorReset+"\n", args...)
}

// colorCell wraps ANSI sequences in tabwriter escape markers. With
// tabwriter.StripEscape this keeps colored tables aligned while preserving
// the terminal control sequences in the output.
func colorCell(color, value string) string {
	const tabwriterEscape = "\xff"
	return tabwriterEscape + color + tabwriterEscape + value + tabwriterEscape + colorReset + tabwriterEscape
}
