package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestShellenvZshSmoke(t *testing.T) {
	resetStdio(t)
	code := cmdShellenv([]string{"zsh"})
	if code != 0 {
		t.Fatalf("cmdShellenv exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}
	out := stdoutBuf.String()
	for _, want := range []string{"wt()", "WT_SHIM=1", "3>", "command wt"} {
		if !strings.Contains(out, want) {
			t.Errorf("shellenv output missing %q:\n%s", want, out)
		}
	}
}

func TestShellenvDefaultsToZsh(t *testing.T) {
	resetStdio(t)
	codeDefault := cmdShellenv(nil)
	defaultOut := stdoutBuf.String()

	resetStdio(t)
	codeZsh := cmdShellenv([]string{"zsh"})
	zshOut := stdoutBuf.String()

	if codeDefault != 0 || codeZsh != 0 {
		t.Fatalf("exit codes = %d, %d, want 0, 0", codeDefault, codeZsh)
	}
	if defaultOut != zshOut {
		t.Errorf("bare `wt shellenv` output differs from `wt shellenv zsh`")
	}
}

func TestShellenvUnknownShell(t *testing.T) {
	resetStdio(t)
	code := cmdShellenv([]string{"fish"})
	if code != 2 {
		t.Fatalf("cmdShellenv exit = %d, want 2; stderr = %s", code, stderrBuf.String())
	}
}

// TestShellenvRealZsh is a nice-to-have end-to-end check: it builds the real
// binary, sources the emitted wrapper in an actual zsh, and confirms `wt
// switch` both cds the shell and exports $WORKTREE. Skipped if zsh isn't on
// PATH.
func TestShellenvRealZsh(t *testing.T) {
	zshPath, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh not on PATH")
	}

	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "wt")
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir, _ = os.Getwd()
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	mainDir := initRepo(t)

	script := `
set -e
eval "$('` + binPath + `' shellenv zsh)"
cd '` + mainDir + `'
wt switch feature >/dev/null
echo "PWD=$PWD"
echo "WORKTREE=$WORKTREE"
`
	cmd := exec.Command(zshPath, "-c", script)
	cmd.Env = append(os.Environ(), "PATH="+binDir+":"+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("zsh script failed: %v\n%s", err, out)
	}

	wtPath := worktreePath(mainDir, "feature")
	got := string(out)
	if !strings.Contains(got, "PWD="+wtPath) {
		t.Errorf("expected shell to cd into %s, got:\n%s", wtPath, got)
	}
	if !strings.Contains(got, "WORKTREE=feature") {
		t.Errorf("expected WORKTREE=feature exported, got:\n%s", got)
	}
}
