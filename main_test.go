package main

import "testing"

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
