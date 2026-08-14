package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSwitchFresh(t *testing.T) {
	dir := initRepo(t)
	chdir(t, dir)
	resetStdio(t)

	code := cmdSwitch([]string{"feature-x"})
	if code != 0 {
		t.Fatalf("cmdSwitch exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}

	wtPath := worktreePath(dir, "feature-x")
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("expected worktree at %s: %v", wtPath, err)
	}
	branch := testGit(t, wtPath, "rev-parse", "--abbrev-ref", "HEAD")
	if branch != "feature-x" {
		t.Errorf("checked-out branch = %q, want feature-x", branch)
	}
	if got := strings.TrimSpace(stdoutBuf.String()); got != wtPath {
		t.Errorf("printed path = %q, want %q", got, wtPath)
	}
}

func TestSwitchExistingBranch(t *testing.T) {
	dir := initRepo(t)
	testGit(t, dir, "branch", "feature-y")
	beforeHead := testGit(t, dir, "rev-parse", "feature-y")

	chdir(t, dir)
	resetStdio(t)

	code := cmdSwitch([]string{"feature-y"})
	if code != 0 {
		t.Fatalf("cmdSwitch exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}

	wtPath := worktreePath(dir, "feature-y")
	afterHead := testGit(t, wtPath, "rev-parse", "HEAD")
	if beforeHead != afterHead {
		t.Errorf("existing branch head moved: before=%s after=%s", beforeHead, afterHead)
	}
}

func TestSwitchExistingWorktree(t *testing.T) {
	dir := initRepo(t)
	chdir(t, dir)
	resetStdio(t)

	if code := cmdSwitch([]string{"feature-z"}); code != 0 {
		t.Fatalf("first cmdSwitch exit = %d; stderr = %s", code, stderrBuf.String())
	}
	wtPath := worktreePath(dir, "feature-z")

	resetStdio(t)
	code := cmdSwitch([]string{"feature-z"})
	if code != 0 {
		t.Fatalf("second cmdSwitch exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}
	if got := strings.TrimSpace(stdoutBuf.String()); got != wtPath {
		t.Errorf("printed path = %q, want %q", got, wtPath)
	}
}

func TestSwitchCreateHookFailure(t *testing.T) {
	dir := initRepo(t)
	hookPath := filepath.Join(dir, ".wt", "create")
	writeFile(t, hookPath, "#!/bin/sh\nexit 1\n")
	if err := os.Chmod(hookPath, 0o755); err != nil {
		t.Fatal(err)
	}
	testGit(t, dir, "add", ".")
	testGit(t, dir, "commit", "-q", "-m", "add failing create hook")

	chdir(t, dir)
	resetStdio(t)

	code := cmdSwitch([]string{"broken"})
	if code != 1 {
		t.Fatalf("cmdSwitch exit = %d, want 1; stderr = %s", code, stderrBuf.String())
	}

	wtPath := worktreePath(dir, "broken")
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("expected worktree left in place at %s: %v", wtPath, err)
	}
	if !strings.Contains(stderrBuf.String(), ".wt/create") {
		t.Errorf("expected hook-failure message, got %q", stderrBuf.String())
	}
}

func TestSwitchTrunkDWIMGuard(t *testing.T) {
	bare := t.TempDir()
	if _, err := runGit(bare, "init", "-q", "--bare", "-b", "main"); err != nil {
		t.Fatal(err)
	}

	dir := initRepo(t)
	testGit(t, dir, "remote", "add", "origin", bare)
	testGit(t, dir, "push", "-q", "origin", "main")
	testGit(t, dir, "remote", "set-head", "origin", "main")

	// Simulate trunk resolving via origin/HEAD with no local branch of that
	// name: park on another branch, then delete local main. origin/HEAD
	// (and the origin/main tracking ref it points to) survive the delete.
	testGit(t, dir, "checkout", "-q", "-b", "wip")
	testGit(t, dir, "branch", "-D", "main")
	if branchExists(dir, "main") {
		t.Fatal("test setup: local main should be gone")
	}

	chdir(t, dir)
	resetStdio(t)

	code := cmdSwitch([]string{"newfeature"})
	if code != 2 {
		t.Fatalf("cmdSwitch exit = %d, want 2; stderr = %s", code, stderrBuf.String())
	}
	if !strings.Contains(stderrBuf.String(), "origin/HEAD") {
		t.Errorf("expected origin/HEAD DWIM-guard message, got %q", stderrBuf.String())
	}

	wtPath := worktreePath(dir, "newfeature")
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Fatalf("worktree should not have been created, stat err = %v", err)
	}
}

func TestSwitchPersistFlag(t *testing.T) {
	dir := initRepo(t)
	chdir(t, dir)
	resetStdio(t)

	code := cmdSwitch([]string{"feature-p", "--persist"})
	if code != 0 {
		t.Fatalf("cmdSwitch exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}
	if !gitConfigBool(dir, "branch.feature-p.wt-persist") {
		t.Errorf("expected branch.feature-p.wt-persist=true after --persist")
	}
}
