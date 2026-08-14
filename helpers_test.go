package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testGit runs git in dir and fails the test on error.
func testGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := runGit(dir, args...)
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return out
}

func setTestIdentity(t *testing.T, dir string) {
	t.Helper()
	testGit(t, dir, "config", "user.name", "Test")
	testGit(t, dir, "config", "user.email", "test@example.com")
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// initRepo creates a fresh repo with an initial commit on branch main. The
// returned path has symlinks resolved (macOS puts TMPDIR under /var, a
// symlink to /private/var) so it matches what os.Getwd() returns after
// chdir, which the shell (and Go) always resolves.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	testGit(t, dir, "init", "-q", "-b", "main")
	setTestIdentity(t, dir)
	writeFile(t, filepath.Join(dir, "README.md"), "hello\n")
	testGit(t, dir, "add", ".")
	testGit(t, dir, "commit", "-q", "-m", "init")
	return dir
}

// setupLinkedTrunkRepo creates a repo where trunk (main) is checked out in a
// linked worktree while the main checkout itself is parked on a different
// branch ("topic") — the "trunk worktree != main checkout" case several
// fixes target. Returns the main checkout path and the trunk worktree path.
func setupLinkedTrunkRepo(t *testing.T) (mainDir, trunkWtPath string) {
	t.Helper()
	mainDir = initRepo(t)
	testGit(t, mainDir, "checkout", "-q", "-b", "topic")

	trunkWtPath = worktreePath(mainDir, "main")
	testGit(t, mainDir, "worktree", "add", "-q", trunkWtPath, "main")

	return mainDir, trunkWtPath
}

// chdir changes the process cwd to dir for the duration of the test.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
}

// stdoutBuf/stderrBuf are set by resetStdio for the currently running test
// to inspect what a cmd function printed.
var stdoutBuf, stderrBuf *bytes.Buffer

// resetStdio redirects the package-level stdin/stdout/stderr for the
// duration of the test, with empty stdin by default.
func resetStdio(t *testing.T) {
	t.Helper()
	stdoutBuf = &bytes.Buffer{}
	stderrBuf = &bytes.Buffer{}
	stdout = stdoutBuf
	stderr = stderrBuf
	stdin = strings.NewReader("")
	t.Cleanup(func() {
		stdout = os.Stdout
		stderr = os.Stderr
		stdin = os.Stdin
	})
}
