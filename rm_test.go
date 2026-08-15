package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRmDirtyRefusal(t *testing.T) {
	dir := initRepo(t)
	chdir(t, dir)
	resetStdio(t)
	if code := cmdCreate([]string{"feature"}); code != 0 {
		t.Fatalf("setup create failed: %d %s", code, stderrBuf.String())
	}
	wtPath := worktreePath(dir, "feature")
	writeFile(t, filepath.Join(wtPath, "dirty.txt"), "x")

	resetStdio(t)
	code := cmdRm([]string{"feature"})
	if code != 1 {
		t.Fatalf("cmdRm exit = %d, want 1; stderr = %s", code, stderrBuf.String())
	}
	if !strings.Contains(stderrBuf.String(), "dirty.txt") {
		t.Errorf("expected dirt to be shown, got %q", stderrBuf.String())
	}
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("worktree should still exist: %v", err)
	}
}

func TestRmForce(t *testing.T) {
	dir := initRepo(t)
	chdir(t, dir)
	resetStdio(t)
	if code := cmdCreate([]string{"feature"}); code != 0 {
		t.Fatalf("setup create failed: %d %s", code, stderrBuf.String())
	}
	wtPath := worktreePath(dir, "feature")
	writeFile(t, filepath.Join(wtPath, "dirty.txt"), "x")

	resetStdio(t)
	stdin = strings.NewReader("y\n")
	code := cmdRm([]string{"feature", "--force"})
	if code != 0 {
		t.Fatalf("cmdRm --force exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Fatalf("worktree should be removed, stat err = %v", err)
	}
}

func TestRmUnmergedBranchReport(t *testing.T) {
	dir := initRepo(t)
	chdir(t, dir)
	resetStdio(t)
	if code := cmdCreate([]string{"feature"}); code != 0 {
		t.Fatalf("setup create failed: %d %s", code, stderrBuf.String())
	}
	wtPath := worktreePath(dir, "feature")
	writeFile(t, filepath.Join(wtPath, "new.txt"), "content\n")
	testGit(t, wtPath, "add", ".")
	testGit(t, wtPath, "commit", "-q", "-m", "unmerged work")

	resetStdio(t)
	stdin = strings.NewReader("y\n")
	code := cmdRm([]string{"feature"})
	if code != 0 {
		t.Fatalf("cmdRm exit = %d, want 0 (removal succeeds even though branch -d refuses); stderr = %s", code, stderrBuf.String())
	}
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Fatalf("worktree should be removed, stat err = %v", err)
	}
	if !strings.Contains(stderrBuf.String(), "branch -D") {
		t.Errorf("expected suggestion to force-delete the branch, got %q", stderrBuf.String())
	}
	if !branchExists(dir, "feature") {
		t.Errorf("branch should still exist since `git branch -d` refused")
	}
}

func TestRmDestroyHookFailureNonFatal(t *testing.T) {
	dir := initRepo(t)
	hookPath := filepath.Join(dir, ".wt", "destroy")
	writeFile(t, hookPath, "#!/bin/sh\nexit 1\n")
	if err := os.Chmod(hookPath, 0o755); err != nil {
		t.Fatal(err)
	}
	testGit(t, dir, "add", ".")
	testGit(t, dir, "commit", "-q", "-m", "add failing destroy hook")

	chdir(t, dir)
	resetStdio(t)
	if code := cmdCreate([]string{"feature"}); code != 0 {
		t.Fatalf("setup create failed: %d %s", code, stderrBuf.String())
	}
	wtPath := worktreePath(dir, "feature")

	resetStdio(t)
	stdin = strings.NewReader("y\n")
	code := cmdRm([]string{"feature"})
	if code != 0 {
		t.Fatalf("cmdRm exit = %d, want 0 despite destroy hook failure; stderr = %s", code, stderrBuf.String())
	}
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Fatalf("worktree should be removed, stat err = %v", err)
	}
}

func TestRmRefusesLinkedTrunkWorktree(t *testing.T) {
	_, trunkWtPath := setupLinkedTrunkRepo(t)
	chdir(t, trunkWtPath)
	resetStdio(t)
	stdin = strings.NewReader("y\n")

	code := cmdRm([]string{"main"})
	if code != 2 {
		t.Fatalf("cmdRm exit = %d, want 2; stderr = %s", code, stderrBuf.String())
	}
	if !strings.Contains(stderrBuf.String(), "trunk worktree") {
		t.Errorf("expected trunk-worktree refusal message, got %q", stderrBuf.String())
	}
	if _, err := os.Stat(trunkWtPath); err != nil {
		t.Fatalf("trunk worktree should still exist: %v", err)
	}
}

func TestRmNeverRemovesTrunk(t *testing.T) {
	dir := initRepo(t)
	chdir(t, dir)
	resetStdio(t)
	stdin = strings.NewReader("y\n")

	code := cmdRm([]string{"main"})
	if code != 2 {
		t.Fatalf("cmdRm exit = %d, want 2; stderr = %s", code, stderrBuf.String())
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("trunk worktree should still exist: %v", err)
	}
}
