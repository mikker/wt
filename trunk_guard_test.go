package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// wt must never drop the trunk: the low-level helpers refuse even if a
// caller's own guards were bypassed.

func TestDeleteBranchRefusesTrunk(t *testing.T) {
	repo := initRepo(t)
	err := deleteBranch(repo, "main", "main")
	if err == nil || !strings.Contains(err.Error(), "refusing to delete trunk branch") {
		t.Fatalf("expected trunk refusal, got %v", err)
	}
	if testGit(t, repo, "branch", "--list", "main") == "" {
		t.Fatal("trunk branch is gone")
	}
}

func TestRemoveWorktreeRefusesTrunk(t *testing.T) {
	mainDir, trunkWtPath := setupLinkedTrunkRepo(t)
	worktrees, err := listWorktrees(mainDir)
	if err != nil {
		t.Fatal(err)
	}
	trunkWt := findWorktreeByName(worktrees, "main")
	if trunkWt == nil {
		t.Fatal("no trunk worktree in fixture")
	}
	err = removeWorktree(mainDir, *trunkWt, "main", true)
	if err == nil || !strings.Contains(err.Error(), "refusing to remove the trunk worktree") {
		t.Fatalf("expected trunk refusal, got %v", err)
	}
	if _, statErr := os.Stat(trunkWtPath); statErr != nil {
		t.Fatalf("trunk worktree is gone: %v", statErr)
	}
}

func TestRmUnresolvableTrunkStops(t *testing.T) {
	// A repo with no main/master and no origin/HEAD: wt rm can't verify the
	// target isn't the trunk, so it must refuse to remove anything.
	dir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	testGit(t, dir, "init", "-q", "-b", "weird")
	setTestIdentity(t, dir)
	testGit(t, dir, "commit", "-q", "--allow-empty", "-m", "init")
	featurePath := worktreePath(dir, "feature")
	testGit(t, dir, "worktree", "add", "-q", "-b", "feature", featurePath, "weird")

	chdir(t, dir)
	resetStdio(t)
	stdin = strings.NewReader("y\n")

	if code := cmdRm([]string{"feature"}); code != 2 {
		t.Fatalf("expected exit 2, got %d\nstderr: %s", code, stderrBuf.String())
	}
	if !strings.Contains(stderrBuf.String(), "could not resolve trunk") {
		t.Fatalf("expected trunk-resolution error, got: %s", stderrBuf.String())
	}
	if _, err := os.Stat(featurePath); err != nil {
		t.Fatalf("worktree was removed despite the refusal: %v", err)
	}
}

func TestOutsideRepoFriendlyError(t *testing.T) {
	dir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	chdir(t, dir)
	resetStdio(t)

	if code := run(nil); code != 2 {
		t.Fatalf("expected exit 2, got %d\nstderr: %s", code, stderrBuf.String())
	}
	got := stderrBuf.String()
	if !strings.Contains(got, "not in a git repository") {
		t.Fatalf("expected friendly message, got: %s", got)
	}
	if strings.Contains(got, "fatal:") {
		t.Fatalf("raw git stderr leaked into the message: %s", got)
	}
}
