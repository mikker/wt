package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupDoneRepo creates a trunk repo, switches into a "feature" worktree,
// and adds one commit there. Returns the main checkout path and the
// worktree path; cwd is left inside the worktree.
func setupDoneRepo(t *testing.T) (mainDir, wtPath string) {
	t.Helper()
	mainDir = initRepo(t)
	chdir(t, mainDir)
	resetStdio(t)
	if code := cmdSwitch([]string{"feature"}); code != 0 {
		t.Fatalf("setup switch failed: %d %s", code, stderrBuf.String())
	}
	wtPath = worktreePath(mainDir, "feature")
	writeFile(t, filepath.Join(wtPath, "feature.txt"), "feature work\n")
	testGit(t, wtPath, "add", ".")
	testGit(t, wtPath, "commit", "-q", "-m", "feature work")
	chdir(t, wtPath)
	return mainDir, wtPath
}

func TestDoneHappyPath(t *testing.T) {
	mainDir, wtPath := setupDoneRepo(t)

	resetStdio(t)
	code := cmdDone(nil)
	if code != 0 {
		t.Fatalf("cmdDone exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}

	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Fatalf("worktree should be removed, stat err = %v", err)
	}
	if branchExists(mainDir, "feature") {
		t.Errorf("branch `feature` should be deleted after merge")
	}
	if _, err := os.Stat(filepath.Join(mainDir, "feature.txt")); err != nil {
		t.Errorf("trunk should contain merged feature.txt: %v", err)
	}
	if got := strings.TrimSpace(stdoutBuf.String()); got != mainDir {
		t.Errorf("printed cd target = %q, want trunk path %q", got, mainDir)
	}
}

func TestDoneDirtyFeatureStop(t *testing.T) {
	_, wtPath := setupDoneRepo(t)
	writeFile(t, filepath.Join(wtPath, "dirty.txt"), "x")

	resetStdio(t)
	code := cmdDone(nil)
	if code != 1 {
		t.Fatalf("cmdDone exit = %d, want 1; stderr = %s", code, stderrBuf.String())
	}
	if !strings.Contains(stderrBuf.String(), "dirty.txt") {
		t.Errorf("expected dirt to be shown, got %q", stderrBuf.String())
	}
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("worktree should still exist: %v", err)
	}
}

func TestDoneDirtyTrunkStop(t *testing.T) {
	mainDir, _ := setupDoneRepo(t)
	writeFile(t, filepath.Join(mainDir, "dirty.txt"), "x")

	resetStdio(t)
	code := cmdDone(nil)
	if code != 1 {
		t.Fatalf("cmdDone exit = %d, want 1; stderr = %s", code, stderrBuf.String())
	}
	if !strings.Contains(stderrBuf.String(), "dirty.txt") {
		t.Errorf("expected trunk dirt to be shown, got %q", stderrBuf.String())
	}
}

func TestDoneRebaseConflictStop(t *testing.T) {
	mainDir, wtPath := setupDoneRepo(t)

	// Diverge trunk and feature on the same line to force a conflict.
	writeFile(t, filepath.Join(mainDir, "README.md"), "trunk change\n")
	testGit(t, mainDir, "commit", "-am", "trunk change")
	writeFile(t, filepath.Join(wtPath, "README.md"), "feature change\n")
	testGit(t, wtPath, "commit", "-am", "feature change")

	resetStdio(t)
	code := cmdDone(nil)
	if code != 1 {
		t.Fatalf("cmdDone exit = %d, want 1; stderr = %s", code, stderrBuf.String())
	}
	msg := stderrBuf.String()
	for _, want := range []string{"rebase", "--continue", "git add"} {
		if !strings.Contains(msg, want) {
			t.Errorf("expected resume instructions to mention %q, got %q", want, msg)
		}
	}
	if _, err := os.Stat(filepath.Join(wtPath, ".git")); err != nil {
		t.Fatalf("worktree should still exist mid-rebase: %v", err)
	}
	// git leaves evidence a rebase is in progress.
	out := testGit(t, wtPath, "status")
	if !strings.Contains(out, "rebase") && !strings.Contains(out, "conflict") {
		t.Errorf("expected git status to show a rebase/conflict in progress, got %q", out)
	}
}

func TestDonePersistentBranchConfig(t *testing.T) {
	mainDir, wtPath := setupDoneRepo(t)
	testGit(t, mainDir, "config", "branch.feature.wt-persist", "true")

	resetStdio(t)
	code := cmdDone(nil)
	if code != 0 {
		t.Fatalf("cmdDone exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}

	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("persistent worktree should stay: %v", err)
	}
	if !branchExists(mainDir, "feature") {
		t.Errorf("persistent branch should not be deleted")
	}
	featureHead := testGit(t, wtPath, "rev-parse", "HEAD")
	trunkHead := testGit(t, mainDir, "rev-parse", "main")
	if featureHead != trunkHead {
		t.Errorf("feature HEAD = %s, want rebased onto trunk HEAD %s", featureHead, trunkHead)
	}
	if _, err := os.Stat(filepath.Join(mainDir, "feature.txt")); err != nil {
		t.Errorf("trunk should contain merged feature.txt: %v", err)
	}
}

func TestDonePersistentProjectConfig(t *testing.T) {
	mainDir := initRepo(t)
	writeFile(t, filepath.Join(mainDir, ".wt", "config"), "persistent = true\n")
	testGit(t, mainDir, "add", ".")
	testGit(t, mainDir, "commit", "-q", "-m", "add wt config")

	chdir(t, mainDir)
	resetStdio(t)
	if code := cmdSwitch([]string{"feature"}); code != 0 {
		t.Fatalf("setup switch failed: %d %s", code, stderrBuf.String())
	}
	wtPath := worktreePath(mainDir, "feature")
	writeFile(t, filepath.Join(wtPath, "feature.txt"), "work\n")
	testGit(t, wtPath, "add", ".")
	testGit(t, wtPath, "commit", "-q", "-m", "feature work")
	chdir(t, wtPath)

	resetStdio(t)
	code := cmdDone(nil)
	if code != 0 {
		t.Fatalf("cmdDone exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("persistent (project config) worktree should stay: %v", err)
	}
	if !branchExists(mainDir, "feature") {
		t.Errorf("persistent branch should not be deleted")
	}
}

func TestDoneRmOverridesPersistent(t *testing.T) {
	mainDir, wtPath := setupDoneRepo(t)
	testGit(t, mainDir, "config", "branch.feature.wt-persist", "true")

	resetStdio(t)
	code := cmdDone([]string{"--rm"})
	if code != 0 {
		t.Fatalf("cmdDone --rm exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Fatalf("--rm should tear down despite persist config, stat err = %v", err)
	}
	if branchExists(mainDir, "feature") {
		t.Errorf("--rm should delete the branch despite persist config")
	}
}

func TestDoneKeepOverridesEphemeral(t *testing.T) {
	mainDir, wtPath := setupDoneRepo(t)

	resetStdio(t)
	code := cmdDone([]string{"--keep"})
	if code != 0 {
		t.Fatalf("cmdDone --keep exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("--keep should leave the worktree in place: %v", err)
	}
	if !branchExists(mainDir, "feature") {
		t.Errorf("--keep should not delete the branch")
	}
}

func TestDoneRmAndKeepConflict(t *testing.T) {
	resetStdio(t)
	code := cmdDone([]string{"--rm", "--keep"})
	if code != 2 {
		t.Fatalf("cmdDone --rm --keep exit = %d, want 2; stderr = %s", code, stderrBuf.String())
	}
}

func TestDoneRefusesFromMainCheckoutOnFeatureBranch(t *testing.T) {
	mainDir, _ := setupLinkedTrunkRepo(t)
	// mainDir is the main checkout, but parked on "topic" — a non-trunk
	// branch. It must still be refused: teardown would `git worktree
	// remove` the main checkout itself.
	chdir(t, mainDir)
	resetStdio(t)

	code := cmdDone(nil)
	if code != 1 {
		t.Fatalf("cmdDone exit = %d, want 1; stderr = %s", code, stderrBuf.String())
	}
	if !strings.Contains(stderrBuf.String(), "main checkout") {
		t.Errorf("expected main-checkout refusal message, got %q", stderrBuf.String())
	}
}

func TestDoneFromFeatureWorktreeWithLinkedTrunk(t *testing.T) {
	mainDir, trunkWtPath := setupLinkedTrunkRepo(t)

	chdir(t, mainDir)
	resetStdio(t)
	if code := cmdSwitch([]string{"feature"}); code != 0 {
		t.Fatalf("setup switch failed: %d %s", code, stderrBuf.String())
	}
	wtPath := worktreePath(mainDir, "feature")
	writeFile(t, filepath.Join(wtPath, "feature.txt"), "feature work\n")
	testGit(t, wtPath, "add", ".")
	testGit(t, wtPath, "commit", "-q", "-m", "feature work")
	chdir(t, wtPath)

	resetStdio(t)
	code := cmdDone(nil)
	if code != 0 {
		t.Fatalf("cmdDone exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}

	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Fatalf("worktree should be removed, stat err = %v", err)
	}
	// Locks in running `git branch -d` in the trunk worktree, not the main
	// checkout: the main checkout is parked on "topic", which doesn't
	// contain feature's commits, so `branch -d` run there would (wrongly)
	// refuse.
	if branchExists(mainDir, "feature") {
		t.Errorf("branch `feature` should be deleted after merge")
	}
	if _, err := os.Stat(filepath.Join(trunkWtPath, "feature.txt")); err != nil {
		t.Errorf("trunk worktree should contain merged feature.txt: %v", err)
	}
	if got := strings.TrimSpace(stdoutBuf.String()); got != trunkWtPath {
		t.Errorf("printed cd target = %q, want trunk worktree path %q", got, trunkWtPath)
	}
	// The main checkout itself should be untouched.
	branch := testGit(t, mainDir, "rev-parse", "--abbrev-ref", "HEAD")
	if branch != "topic" {
		t.Errorf("main checkout branch = %q, want topic (untouched)", branch)
	}
}

func TestDoneDetachedHeadStop(t *testing.T) {
	mainDir, wtPath := setupDoneRepo(t)
	head := testGit(t, wtPath, "rev-parse", "HEAD")
	testGit(t, wtPath, "checkout", "-q", head)

	resetStdio(t)
	code := cmdDone(nil)
	if code != 1 {
		t.Fatalf("cmdDone exit = %d, want 1; stderr = %s", code, stderrBuf.String())
	}
	if !strings.Contains(stderrBuf.String(), "detached") {
		t.Errorf("expected detached-HEAD message, got %q", stderrBuf.String())
	}
	// No rebase should have started.
	out := testGit(t, wtPath, "status")
	if strings.Contains(out, "rebase") {
		t.Errorf("rebase should not have started on detached HEAD, got status %q", out)
	}
	if !branchExists(mainDir, "feature") {
		t.Errorf("feature branch should be untouched")
	}
}

func TestDoneOnTrunkStop(t *testing.T) {
	mainDir := initRepo(t)
	chdir(t, mainDir)
	resetStdio(t)

	code := cmdDone(nil)
	if code != 1 {
		t.Fatalf("cmdDone exit = %d, want 1; stderr = %s", code, stderrBuf.String())
	}
	if !strings.Contains(stderrBuf.String(), "non-trunk") {
		t.Errorf("expected non-trunk message, got %q", stderrBuf.String())
	}
}
