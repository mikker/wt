package main

import (
	"path/filepath"
	"testing"
)

func TestResolveTrunk_OriginHead(t *testing.T) {
	bare := t.TempDir()
	if _, err := runGit(bare, "init", "-q", "--bare", "-b", "main"); err != nil {
		t.Fatal(err)
	}

	dir := initRepo(t)
	testGit(t, dir, "remote", "add", "origin", bare)
	testGit(t, dir, "push", "-q", "origin", "main")
	testGit(t, dir, "remote", "set-head", "origin", "main")

	// A branch named "master" also exists locally to prove origin/HEAD
	// wins over the fallback chain.
	testGit(t, dir, "branch", "master")

	trunk, err := resolveTrunk(dir)
	if err != nil {
		t.Fatalf("resolveTrunk: %v", err)
	}
	if trunk != "main" {
		t.Errorf("resolveTrunk = %q, want %q", trunk, "main")
	}
}

func TestResolveTrunk_MainOnly(t *testing.T) {
	dir := initRepo(t)

	trunk, err := resolveTrunk(dir)
	if err != nil {
		t.Fatalf("resolveTrunk: %v", err)
	}
	if trunk != "main" {
		t.Errorf("resolveTrunk = %q, want %q", trunk, "main")
	}
}

func TestResolveTrunk_MasterOnly(t *testing.T) {
	dir := t.TempDir()
	testGit(t, dir, "init", "-q", "-b", "master")
	setTestIdentity(t, dir)
	writeFile(t, filepath.Join(dir, "f"), "1\n")
	testGit(t, dir, "add", ".")
	testGit(t, dir, "commit", "-q", "-m", "init")

	trunk, err := resolveTrunk(dir)
	if err != nil {
		t.Fatalf("resolveTrunk: %v", err)
	}
	if trunk != "master" {
		t.Errorf("resolveTrunk = %q, want %q", trunk, "master")
	}
}

func TestResolveTrunk_Neither(t *testing.T) {
	dir := t.TempDir()
	testGit(t, dir, "init", "-q", "-b", "trunk")
	setTestIdentity(t, dir)
	writeFile(t, filepath.Join(dir, "f"), "1\n")
	testGit(t, dir, "add", ".")
	testGit(t, dir, "commit", "-q", "-m", "init")

	_, err := resolveTrunk(dir)
	if err == nil {
		t.Fatal("resolveTrunk: expected error, got nil")
	}
}

func TestParseWorktrees(t *testing.T) {
	porcelain := "worktree /repo\n" +
		"HEAD abc123\n" +
		"branch refs/heads/main\n" +
		"\n" +
		"worktree /repo-worktrees/feature\n" +
		"HEAD def456\n" +
		"branch refs/heads/feature\n" +
		"\n" +
		"worktree /repo-worktrees/detached\n" +
		"HEAD 789abc\n" +
		"detached\n"

	worktrees := parseWorktrees(porcelain)
	if len(worktrees) != 3 {
		t.Fatalf("got %d worktrees, want 3", len(worktrees))
	}
	if !worktrees[0].IsMain || worktrees[0].Branch != "main" {
		t.Errorf("worktrees[0] = %+v, want IsMain=true Branch=main", worktrees[0])
	}
	if worktrees[1].IsMain || worktrees[1].Branch != "feature" {
		t.Errorf("worktrees[1] = %+v, want IsMain=false Branch=feature", worktrees[1])
	}
	if !worktrees[2].Detached || worktrees[2].Branch != "" {
		t.Errorf("worktrees[2] = %+v, want Detached=true Branch=\"\"", worktrees[2])
	}
}

func TestWorktreePath(t *testing.T) {
	got := worktreePath("/dev/tuna", "login-fix")
	want := filepath.FromSlash("/dev/tuna-worktrees/login-fix")
	if got != want {
		t.Errorf("worktreePath = %q, want %q", got, want)
	}
}
