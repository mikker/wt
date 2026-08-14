package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseConfig(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".wt", "config"), "# comment\npersistent = true\nother = 1\n\n")

	cfg := parseConfig(dir)
	if cfg["persistent"] != "true" {
		t.Errorf("cfg[persistent] = %q, want true", cfg["persistent"])
	}
	if cfg["other"] != "1" {
		t.Errorf("cfg[other] = %q, want 1", cfg["other"])
	}
}

func TestParseConfigMissingFile(t *testing.T) {
	dir := t.TempDir()
	cfg := parseConfig(dir)
	if len(cfg) != 0 {
		t.Errorf("expected empty config for missing file, got %v", cfg)
	}
}

func TestProjectPersistent(t *testing.T) {
	dir := t.TempDir()
	if projectPersistent(dir) {
		t.Errorf("expected false with no .wt/config")
	}
	writeFile(t, filepath.Join(dir, ".wt", "config"), "persistent = true\n")
	if !projectPersistent(dir) {
		t.Errorf("expected true after persistent = true")
	}
}

func TestRunCreateHook_AbsentIsNoop(t *testing.T) {
	dir := t.TempDir()
	if err := runCreateHook(dir, "/base"); err != nil {
		t.Errorf("expected nil error for missing hook, got %v", err)
	}
}

func TestRunCreateHook_NotExecutableIsSkipped(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".wt", "create"), "#!/bin/sh\nexit 1\n")
	// deliberately not chmod +x
	if err := runCreateHook(dir, "/base"); err != nil {
		t.Errorf("expected nil error for non-executable hook, got %v", err)
	}
}

// TestRunHookStripsWTShim locks in that hooks never see a leaked WT_SHIM=1:
// they inherit the process environment but not fd 3, so a hook that shells
// out to `wt` while WT_SHIM=1 leaked would otherwise hit the silent
// fd-3-write-failure case.
func TestRunHookStripsWTShim(t *testing.T) {
	dir := t.TempDir()
	hookPath := filepath.Join(dir, "hook.sh")
	outPath := filepath.Join(dir, "out.txt")
	writeFile(t, hookPath, "#!/bin/sh\necho \"${WT_SHIM:-unset}\" > "+outPath+"\n")
	if err := os.Chmod(hookPath, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("WT_SHIM", "1")

	if err := runHook(dir, hookPath); err != nil {
		t.Fatalf("runHook: %v", err)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if want := "unset"; strings.TrimSpace(string(got)) != want {
		t.Errorf("hook saw WT_SHIM=%q, want %q", strings.TrimSpace(string(got)), want)
	}
}
