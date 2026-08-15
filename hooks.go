package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// runCreateHook runs .wt/create if present and executable, with cwd set to
// worktreeDir and $1 = baseDir (the main checkout's absolute path). Returns
// an error only when the hook exists, is executable, and fails; the caller
// decides what to do (per the plan: report the failure but leave the
// worktree in place).
func runCreateHook(worktreeDir, baseDir string) error {
	return runHook(worktreeDir, filepath.Join(worktreeDir, ".wt", "create"), baseDir)
}

// runDestroyHook runs .wt/destroy if present and executable, with cwd set
// to dir. Failure is never fatal: it's reported as a warning on stderr.
func runDestroyHook(dir string) {
	if err := runHook(dir, filepath.Join(dir, ".wt", "destroy")); err != nil {
		fmt.Fprintf(stderr, "warning: .wt/destroy failed: %v\n", err)
	}
}

func runHook(cwd, path string, args ...string) error {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return nil // hook absent: nothing to do
	}
	if info.Mode()&0o111 == 0 {
		return nil // not executable: skip per contract
	}
	cmd := exec.Command(path, args...)
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	// Hooks inherit the process environment, including WT_SHIM=1 if wt is
	// running under the shell shim — but not fd 3, which the shim doesn't
	// pass down to child processes. A hook that shells out to `wt` while
	// WT_SHIM=1 leaks would hit the silent fd-3-write-failure case, so strip
	// it here.
	cmd.Env = stripEnv(os.Environ(), "WT_SHIM")
	return cmd.Run()
}

// parseConfig reads .wt/config (key = value per line, "#" comments, blank
// lines ignored) from dir. A missing file returns an empty map, not an
// error.
func parseConfig(dir string) map[string]string {
	cfg := make(map[string]string)
	f, err := os.Open(filepath.Join(dir, ".wt", "config"))
	if err != nil {
		return cfg
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		cfg[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return cfg
}

// projectPersistent reports whether .wt/config sets `persistent = true` in
// dir.
func projectPersistent(dir string) bool {
	return parseConfig(dir)["persistent"] == "true"
}
