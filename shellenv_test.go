package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestShellenvSmoke(t *testing.T) {
	for _, test := range []struct {
		shell        string
		registration string
	}{
		{shell: "zsh", registration: "compdef _wt wt"},
		{shell: "bash", registration: "complete -F _wt wt"},
	} {
		t.Run(test.shell, func(t *testing.T) {
			resetStdio(t)
			code := cmdShellenv([]string{test.shell})
			if code != 0 {
				t.Fatalf("cmdShellenv exit = %d, want 0; stderr = %s", code, stderrBuf.String())
			}
			out := stdoutBuf.String()
			for _, want := range []string{"wt()", "WT_SHIM=1", "3>", "command wt", "_wt()", test.registration, "git worktree list --porcelain"} {
				if !strings.Contains(out, want) {
					t.Errorf("shellenv output missing %q:\n%s", want, out)
				}
			}
		})
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

func TestShellenvRealShells(t *testing.T) {
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "wt")
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir, _ = os.Getwd()
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	for _, shell := range []string{"zsh", "bash"} {
		t.Run(shell, func(t *testing.T) {
			shellPath, err := exec.LookPath(shell)
			if err != nil {
				t.Skipf("%s not on PATH", shell)
			}

			mainDir := initRepo(t)
			script := `
set -e
eval "$('` + binPath + `' shellenv ` + shell + `)"
cd '` + mainDir + `'
wt create feature >/dev/null
echo "PWD=$PWD"
echo "WORKTREE=$WORKTREE"
_wt_worktree_names
`
			cmd := exec.Command(shellPath, "-c", script)
			cmd.Env = append(os.Environ(), "PATH="+binDir+":"+os.Getenv("PATH"))
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%s script failed: %v\n%s", shell, err, out)
			}

			wtPath := worktreePath(mainDir, "feature")
			got := string(out)
			if !strings.Contains(got, "PWD="+wtPath) {
				t.Errorf("expected shell to cd into %s, got:\n%s", wtPath, got)
			}
			if !strings.Contains(got, "WORKTREE=feature") {
				t.Errorf("expected WORKTREE=feature exported, got:\n%s", got)
			}
			fields := strings.Fields(got)
			for _, want := range []string{"main", "feature"} {
				if !slices.Contains(fields, want) {
					t.Errorf("completion names = %v, missing %q", fields, want)
				}
			}
		})
	}
}
