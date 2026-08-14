package main

import (
	"strings"
	"testing"
)

func TestLsBasic(t *testing.T) {
	dir := initRepo(t)
	chdir(t, dir)
	resetStdio(t)
	if code := cmdSwitch([]string{"feature"}); code != 0 {
		t.Fatalf("setup switch failed: %d %s", code, stderrBuf.String())
	}

	wtPath := worktreePath(dir, "feature")
	writeFile(t, wtPath+"/new.txt", "hi\n")
	testGit(t, wtPath, "add", ".")
	testGit(t, wtPath, "commit", "-q", "-m", "feature work")
	writeFile(t, wtPath+"/untracked.txt", "x")

	resetStdio(t)
	code := cmdLs(nil)
	if code != 0 {
		t.Fatalf("cmdLs exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}

	out := stdoutBuf.String()
	if !strings.Contains(out, "feature") {
		t.Errorf("expected output to mention the feature branch, got:\n%s", out)
	}
	if !strings.Contains(out, "dirty") {
		t.Errorf("expected a dirty marker, got:\n%s", out)
	}
	if !strings.Contains(out, "main") {
		t.Errorf("expected the main checkout to be listed, got:\n%s", out)
	}
	// feature is 1 commit ahead of main and 0 behind.
	lines := strings.Split(strings.TrimSpace(out), "\n")
	found := false
	for _, l := range lines {
		if strings.HasPrefix(l, "feature") {
			found = true
			fields := strings.Fields(l)
			// NAME BRANCH DIRTY AHEAD BEHIND [PERSISTENT]
			if len(fields) < 5 {
				t.Fatalf("unexpected ls line format: %q", l)
			}
		}
	}
	if !found {
		t.Errorf("no line for feature worktree in output:\n%s", out)
	}
}

func TestLsPersistentMarker(t *testing.T) {
	dir := initRepo(t)
	chdir(t, dir)
	resetStdio(t)
	if code := cmdSwitch([]string{"feature", "--persist"}); code != 0 {
		t.Fatalf("setup switch failed: %d %s", code, stderrBuf.String())
	}

	resetStdio(t)
	code := cmdLs(nil)
	if code != 0 {
		t.Fatalf("cmdLs exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}
	out := stdoutBuf.String()
	if !strings.Contains(out, "persistent") {
		t.Errorf("expected persistent marker in output:\n%s", out)
	}
}
