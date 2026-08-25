package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestActionHeadlineUsesANSIColor(t *testing.T) {
	resetStdio(t)

	actionHeadline("Creating %s", "feature")

	if got, want := stderrBuf.String(), "\x1b[1;36m==> Creating feature\x1b[0m\n"; got != want {
		t.Fatalf("headline = %q, want %q", got, want)
	}
}

func TestRunHookPrintsHeadlineAndScriptOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "setup")
	writeFile(t, path, "#!/bin/sh\necho setting-up\necho installing >&2\n")
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	resetStdio(t)

	if err := runHook(dir, path); err != nil {
		t.Fatal(err)
	}

	if got := stdoutBuf.String(); got != "setting-up\n" {
		t.Fatalf("script stdout = %q", got)
	}
	for _, want := range []string{"\x1b[1;36m==> Running " + path, "installing\n"} {
		if !strings.Contains(stderrBuf.String(), want) {
			t.Errorf("stderr missing %q: %q", want, stderrBuf.String())
		}
	}
}
