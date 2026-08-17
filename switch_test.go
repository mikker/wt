package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateFresh(t *testing.T) {
	dir := initRepo(t)
	chdir(t, dir)
	resetStdio(t)

	code := cmdCreate([]string{"feature-x"})
	if code != 0 {
		t.Fatalf("cmdCreate exit = %d, want 0; stderr = %s", code, stderrBuf.String())
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
	if got := testGit(t, dir, "status", "--porcelain"); got != "" {
		t.Errorf("nested worktree dirtied main checkout:\n%s", got)
	}
	exclude, err := os.ReadFile(filepath.Join(dir, ".git", "info", "exclude"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(exclude), "/.wt/worktrees/"; !strings.Contains(got, want) {
		t.Errorf("git info/exclude = %q, want it to contain %q", got, want)
	}
}

func TestCreateUsesConfiguredWorktreesDirectory(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, filepath.Join(dir, ".wt", "config"), "worktrees = .worktrees\n")
	testGit(t, dir, "add", ".wt/config")
	testGit(t, dir, "commit", "-q", "-m", "configure worktrees")
	chdir(t, dir)
	resetStdio(t)

	if code := cmdCreate([]string{"feature"}); code != 0 {
		t.Fatalf("cmdCreate exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}

	wtPath := filepath.Join(dir, ".worktrees", "feature")
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("expected worktree at %s: %v", wtPath, err)
	}
	if got := testGit(t, dir, "status", "--porcelain"); got != "" {
		t.Errorf("configured nested worktree dirtied main checkout:\n%s", got)
	}
	exclude, err := os.ReadFile(filepath.Join(dir, ".git", "info", "exclude"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(exclude), "/.worktrees/"; !strings.Contains(got, want) {
		t.Errorf("git info/exclude = %q, want it to contain %q", got, want)
	}
}

func TestCreateAlias(t *testing.T) {
	dir := initRepo(t)
	chdir(t, dir)
	resetStdio(t)

	if code := run([]string{"c", "feature-c"}); code != 0 {
		t.Fatalf("wt c exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}
	if _, err := os.Stat(worktreePath(dir, "feature-c")); err != nil {
		t.Fatalf("expected alias to create worktree: %v", err)
	}
}

func TestCreateExistingBranch(t *testing.T) {
	dir := initRepo(t)
	testGit(t, dir, "branch", "feature-y")
	beforeHead := testGit(t, dir, "rev-parse", "feature-y")

	chdir(t, dir)
	resetStdio(t)

	code := cmdCreate([]string{"feature-y"})
	if code != 0 {
		t.Fatalf("cmdCreate exit = %d, want 0; stderr = %s", code, stderrBuf.String())
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

	if code := cmdCreate([]string{"feature-z"}); code != 0 {
		t.Fatalf("cmdCreate exit = %d; stderr = %s", code, stderrBuf.String())
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

func TestCreateHookFailure(t *testing.T) {
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

	code := cmdCreate([]string{"broken"})
	if code != 1 {
		t.Fatalf("cmdCreate exit = %d, want 1; stderr = %s", code, stderrBuf.String())
	}

	wtPath := worktreePath(dir, "broken")
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("expected worktree left in place at %s: %v", wtPath, err)
	}
	if !strings.Contains(stderrBuf.String(), ".wt/create") {
		t.Errorf("expected hook-failure message, got %q", stderrBuf.String())
	}
}

func TestCreateTrunkDWIMGuard(t *testing.T) {
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

	code := cmdCreate([]string{"newfeature"})
	if code != 2 {
		t.Fatalf("cmdCreate exit = %d, want 2; stderr = %s", code, stderrBuf.String())
	}
	if !strings.Contains(stderrBuf.String(), "origin/HEAD") {
		t.Errorf("expected origin/HEAD DWIM-guard message, got %q", stderrBuf.String())
	}

	wtPath := worktreePath(dir, "newfeature")
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Fatalf("worktree should not have been created, stat err = %v", err)
	}
}

func TestCreatePersistFlag(t *testing.T) {
	dir := initRepo(t)
	chdir(t, dir)
	resetStdio(t)

	code := cmdCreate([]string{"feature-p", "--persist"})
	if code != 0 {
		t.Fatalf("cmdCreate exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}
	if !gitConfigBool(dir, "branch.feature-p.wt-persist") {
		t.Errorf("expected branch.feature-p.wt-persist=true after --persist")
	}
}

func TestCreateRunsCommandInReadyWorktree(t *testing.T) {
	dir := initRepo(t)
	t.Setenv("WORKTREE", "old-worktree")
	hookPath := filepath.Join(dir, ".wt", "create")
	writeFile(t, hookPath, "#!/bin/sh\nprintf ready > hook-result\n")
	if err := os.Chmod(hookPath, 0o755); err != nil {
		t.Fatal(err)
	}
	testGit(t, dir, "add", ".")
	testGit(t, dir, "commit", "-q", "-m", "add create hook")

	chdir(t, dir)
	resetStdio(t)
	code := cmdCreate([]string{
		"feature-command", "--", "sh", "-c",
		`test "$(cat hook-result)" = ready && printf '%s\n%s\n%s\n' "$PWD" "$WORKTREE" "$1" > command-result`,
		"sh", `Fix issue "xyz"`,
	})
	if code != 0 {
		t.Fatalf("cmdCreate exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}

	wtPath := worktreePath(dir, "feature-command")
	data, err := os.ReadFile(filepath.Join(wtPath, "command-result"))
	if err != nil {
		t.Fatal(err)
	}
	want := wtPath + "\nfeature-command\nFix issue \"xyz\"\n"
	if got := string(data); got != want {
		t.Errorf("command result = %q, want %q", got, want)
	}
}

func TestCreateCommandStripsShellShimEnvironment(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WT_SHIM", "1")
	resetStdio(t)

	code := runCreateCommand(dir, "feature", []string{"sh", "-c", `printf '%s' "${WT_SHIM-unset}" > shim-env`})
	if code != 0 {
		t.Fatalf("runCreateCommand exit = %d; stderr = %s", code, stderrBuf.String())
	}
	data, err := os.ReadFile(filepath.Join(dir, "shim-env"))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != "unset" {
		t.Errorf("WT_SHIM = %q, want unset", got)
	}
}

func TestCreateDoesNotEnterWorktreeRemovedByCommand(t *testing.T) {
	dir := initRepo(t)
	chdir(t, dir)
	resetStdio(t)

	code := cmdCreate([]string{
		"self-removing", "--", "sh", "-c",
		`git -C "$1" worktree remove --force "$PWD"`, "sh", dir,
	})
	if code != 0 {
		t.Fatalf("cmdCreate exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}
	if got := stdoutBuf.String(); got != "" {
		t.Errorf("cmdCreate emitted stale worktree entry target %q", got)
	}
	if _, err := os.Stat(worktreePath(dir, "self-removing")); !os.IsNotExist(err) {
		t.Fatalf("command should have removed its worktree, stat err = %v", err)
	}
}

func TestCreateReturnsCommandExitStatus(t *testing.T) {
	dir := initRepo(t)
	chdir(t, dir)
	resetStdio(t)

	code := cmdCreate([]string{"feature-failing-command", "--", "sh", "-c", "exit 7"})
	if code != 7 {
		t.Fatalf("cmdCreate exit = %d, want 7; stderr = %s", code, stderrBuf.String())
	}
	if got := strings.TrimSpace(stdoutBuf.String()); got != worktreePath(dir, "feature-failing-command") {
		t.Errorf("printed path = %q, want created worktree path", got)
	}
}

func TestCreateRequiresCommandAfterSeparator(t *testing.T) {
	resetStdio(t)

	code := cmdCreate([]string{"feature", "--"})
	if code != 2 {
		t.Fatalf("cmdCreate exit = %d, want 2", code)
	}
	if !strings.Contains(stderrBuf.String(), "command is required after --") {
		t.Errorf("expected missing command guidance, got %q", stderrBuf.String())
	}
}

func TestSwitchMissingWorktreeDoesNotCreate(t *testing.T) {
	dir := initRepo(t)
	chdir(t, dir)
	resetStdio(t)

	code := cmdSwitch([]string{"missing"})
	if code != 2 {
		t.Fatalf("cmdSwitch exit = %d, want 2; stderr = %s", code, stderrBuf.String())
	}
	if !strings.Contains(stderrBuf.String(), "wt create missing") {
		t.Errorf("expected create guidance, got %q", stderrBuf.String())
	}
	if _, err := os.Stat(worktreePath(dir, "missing")); !os.IsNotExist(err) {
		t.Fatalf("switch created a missing worktree, stat err = %v", err)
	}
}

func TestCreateExistingWorktreeRefusesToSwitch(t *testing.T) {
	dir := initRepo(t)
	chdir(t, dir)
	resetStdio(t)

	if code := cmdCreate([]string{"existing"}); code != 0 {
		t.Fatalf("first cmdCreate exit = %d; stderr = %s", code, stderrBuf.String())
	}
	resetStdio(t)

	code := cmdCreate([]string{"existing"})
	if code != 2 {
		t.Fatalf("second cmdCreate exit = %d, want 2; stderr = %s", code, stderrBuf.String())
	}
	if !strings.Contains(stderrBuf.String(), "wt switch existing") {
		t.Errorf("expected switch guidance, got %q", stderrBuf.String())
	}
}
