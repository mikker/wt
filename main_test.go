package main

import (
	"bytes"
	"testing"
)

func TestNoArgsPrintsUsage(t *testing.T) {
	resetStdio(t)

	if code := run(nil); code != 0 {
		t.Fatalf("run(nil) exit = %d, want 0", code)
	}

	var usage bytes.Buffer
	printUsage(&usage)
	if got, want := stdoutBuf.String(), usage.String(); got != want {
		t.Fatalf("run(nil) output = %q, want %q", got, want)
	}
	if got := stderrBuf.String(); got != "" {
		t.Fatalf("run(nil) stderr = %q, want empty", got)
	}
}

func TestVersionFlags(t *testing.T) {
	originalVersion := version
	version = "1.2.3"
	t.Cleanup(func() { version = originalVersion })

	for _, flag := range []string{"-v", "--version"} {
		t.Run(flag, func(t *testing.T) {
			resetStdio(t)

			if code := run([]string{flag}); code != 0 {
				t.Fatalf("run(%q) exit = %d, want 0", flag, code)
			}
			if got, want := stdoutBuf.String(), "wt 1.2.3\n"; got != want {
				t.Fatalf("run(%q) output = %q, want %q", flag, got, want)
			}
			if got := stderrBuf.String(); got != "" {
				t.Fatalf("run(%q) stderr = %q, want empty", flag, got)
			}
		})
	}
}
