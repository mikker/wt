package main

import (
	"strings"
	"testing"
)

func TestPersistToggle(t *testing.T) {
	dir := initRepo(t)
	chdir(t, dir)
	resetStdio(t)
	if code := cmdCreate([]string{"feature"}); code != 0 {
		t.Fatalf("setup create failed: %d %s", code, stderrBuf.String())
	}
	chdir(t, worktreePath(dir, "feature"))

	resetStdio(t)
	if code := cmdPersist(nil); code != 0 {
		t.Fatalf("cmdPersist exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}
	if !gitConfigBool(dir, "branch.feature.wt-persist") {
		t.Fatalf("expected branch.feature.wt-persist=true after first toggle")
	}
	if !strings.Contains(stdoutBuf.String(), "persistent") {
		t.Errorf("expected output to mention persistent, got %q", stdoutBuf.String())
	}

	resetStdio(t)
	if code := cmdPersist(nil); code != 0 {
		t.Fatalf("cmdPersist exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}
	if gitConfigBool(dir, "branch.feature.wt-persist") {
		t.Fatalf("expected branch.feature.wt-persist=false after second toggle")
	}
	if !strings.Contains(stdoutBuf.String(), "ephemeral") {
		t.Errorf("expected output to mention ephemeral, got %q", stdoutBuf.String())
	}
}

func TestPersistOnTrunkRefused(t *testing.T) {
	dir := initRepo(t)
	chdir(t, dir)
	resetStdio(t)

	code := cmdPersist(nil)
	if code != 2 {
		t.Fatalf("cmdPersist exit = %d, want 2; stderr = %s", code, stderrBuf.String())
	}
	if !strings.Contains(stderrBuf.String(), "trunk") {
		t.Errorf("expected trunk-related message, got %q", stderrBuf.String())
	}
}
