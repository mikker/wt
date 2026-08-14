package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupShipRepo creates a trunk repo with a bare "origin" remote (already
// pushed), switches into a "feature" worktree, and adds one commit there.
// cwd is left inside the worktree.
func setupShipRepo(t *testing.T) (mainDir, bareDir, wtPath string) {
	t.Helper()
	mainDir = initRepo(t)
	bareDir = t.TempDir()
	if resolved, err := filepath.EvalSymlinks(bareDir); err == nil {
		bareDir = resolved
	}
	testGit(t, bareDir, "init", "-q", "--bare", "-b", "main")
	testGit(t, mainDir, "remote", "add", "origin", bareDir)
	testGit(t, mainDir, "push", "-q", "-u", "origin", "main")

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
	return mainDir, bareDir, wtPath
}

// pushUpstreamCommit simulates a teammate pushing directly to origin: clones
// the bare repo to a scratch dir, commits, and pushes.
func pushUpstreamCommit(t *testing.T, bareDir, filename, content string) {
	t.Helper()
	clone := t.TempDir()
	testGit(t, clone, "clone", "-q", bareDir, ".")
	setTestIdentity(t, clone)
	writeFile(t, filepath.Join(clone, filename), content)
	testGit(t, clone, "add", ".")
	testGit(t, clone, "commit", "-q", "-m", "upstream: "+filename)
	testGit(t, clone, "push", "-q", "origin", "main")
}

func TestShipHappyPath(t *testing.T) {
	mainDir, bareDir, wtPath := setupShipRepo(t)
	pushUpstreamCommit(t, bareDir, "upstream.txt", "from teammate\n")

	resetStdio(t)
	code := cmdShip(nil)
	if code != 0 {
		t.Fatalf("cmdShip exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}

	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Fatalf("worktree should be removed, stat err = %v", err)
	}
	if branchExists(mainDir, "feature") {
		t.Errorf("branch `feature` should be deleted after ship")
	}
	if _, err := os.Stat(filepath.Join(mainDir, "upstream.txt")); err != nil {
		t.Errorf("local trunk should have upstream's commit: %v", err)
	}
	if _, err := os.Stat(filepath.Join(mainDir, "feature.txt")); err != nil {
		t.Errorf("local trunk should have feature's commit: %v", err)
	}

	localHead := testGit(t, mainDir, "rev-parse", "main")
	remoteHead := testGit(t, mainDir, "rev-parse", "origin/main")
	if localHead != remoteHead {
		t.Errorf("origin/main = %s, want pushed local trunk %s", remoteHead, localHead)
	}

	// The feature branch itself was never pushed.
	branches := testGit(t, bareDir, "branch", "--list")
	if strings.Contains(branches, "feature") {
		t.Errorf("origin should not have a feature branch, got %q", branches)
	}
}

func TestShipDivergedTrunkStop(t *testing.T) {
	mainDir, bareDir, wtPath := setupShipRepo(t)
	pushUpstreamCommit(t, bareDir, "upstream.txt", "from teammate\n")
	bareHeadBefore := testGit(t, bareDir, "rev-parse", "main")

	// Local trunk also has a commit origin doesn't have: a real divergence.
	writeFile(t, filepath.Join(mainDir, "local-only.txt"), "local\n")
	testGit(t, mainDir, "add", ".")
	testGit(t, mainDir, "commit", "-q", "-m", "local-only trunk commit")
	localHeadBefore := testGit(t, mainDir, "rev-parse", "main")

	resetStdio(t)
	code := cmdShip(nil)
	if code != 1 {
		t.Fatalf("cmdShip exit = %d, want 1; stderr = %s", code, stderrBuf.String())
	}
	if !strings.Contains(stderrBuf.String(), "diverged") {
		t.Errorf("expected diverged-trunk message, got %q", stderrBuf.String())
	}
	// Nothing should have been pushed or merged: origin (bare repo) and the
	// local trunk branch are both exactly where they were before the stop
	// (fetch is allowed to have updated the local origin/main tracking ref).
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("feature worktree should still exist: %v", err)
	}
	if got := testGit(t, bareDir, "rev-parse", "main"); got != bareHeadBefore {
		t.Errorf("origin main = %s, want unchanged %s", got, bareHeadBefore)
	}
	if got := testGit(t, mainDir, "rev-parse", "main"); got != localHeadBefore {
		t.Errorf("local main = %s, want unchanged %s", got, localHeadBefore)
	}
}

func TestShipNoOriginRemote(t *testing.T) {
	mainDir := initRepo(t)
	chdir(t, mainDir)
	resetStdio(t)
	if code := cmdSwitch([]string{"feature"}); code != 0 {
		t.Fatalf("setup switch failed: %d %s", code, stderrBuf.String())
	}
	chdir(t, worktreePath(mainDir, "feature"))

	resetStdio(t)
	code := cmdShip(nil)
	if code != 2 {
		t.Fatalf("cmdShip exit = %d, want 2; stderr = %s", code, stderrBuf.String())
	}
	if !strings.Contains(stderrBuf.String(), "origin") {
		t.Errorf("expected an origin-related error, got %q", stderrBuf.String())
	}
}
