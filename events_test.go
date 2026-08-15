package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type eventCapture struct {
	eventPath       string
	cwdPath         string
	environmentPath string
	worktreeEnvPath string
	shimEnvPath     string
	remoteHeadPath  string
}

func installEventHandler(t *testing.T) eventCapture {
	t.Helper()
	dir := t.TempDir()
	capture := eventCapture{
		eventPath:       filepath.Join(dir, "event.json"),
		cwdPath:         filepath.Join(dir, "cwd"),
		environmentPath: filepath.Join(dir, "environment"),
		worktreeEnvPath: filepath.Join(dir, "worktree-environment"),
		shimEnvPath:     filepath.Join(dir, "shim-environment"),
		remoteHeadPath:  filepath.Join(dir, "remote-head"),
	}
	handler := filepath.Join(dir, "handler")
	writeFile(t, handler, `#!/bin/sh
cat > "$WT_TEST_EVENT_FILE"
pwd > "$WT_TEST_CWD_FILE"
printf '%s' "$WT_TEST_INHERITED" > "$WT_TEST_ENV_FILE"
if [ "${WORKTREE+x}" = x ]; then
  printf 'set:%s' "$WORKTREE" > "$WT_TEST_WORKTREE_ENV_FILE"
else
  printf unset > "$WT_TEST_WORKTREE_ENV_FILE"
fi
if [ "${WT_SHIM+x}" = x ]; then
  printf 'set:%s' "$WT_SHIM" > "$WT_TEST_SHIM_ENV_FILE"
else
  printf unset > "$WT_TEST_SHIM_ENV_FILE"
fi
if [ -n "$WT_TEST_REMOTE" ]; then
  git --git-dir="$WT_TEST_REMOTE" rev-parse main > "$WT_TEST_REMOTE_HEAD_FILE"
fi
`)
	if err := os.Chmod(handler, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WT_EVENT_HANDLER", handler)
	t.Setenv("WT_TEST_EVENT_FILE", capture.eventPath)
	t.Setenv("WT_TEST_CWD_FILE", capture.cwdPath)
	t.Setenv("WT_TEST_ENV_FILE", capture.environmentPath)
	t.Setenv("WT_TEST_WORKTREE_ENV_FILE", capture.worktreeEnvPath)
	t.Setenv("WT_TEST_SHIM_ENV_FILE", capture.shimEnvPath)
	t.Setenv("WT_TEST_REMOTE_HEAD_FILE", capture.remoteHeadPath)
	t.Setenv("WT_TEST_INHERITED", "inherited-value")
	t.Setenv("WORKTREE", "feature")
	return capture
}

func assertRemovalEvent(t *testing.T, capture eventCapture, operation, mainDir, trunkDir, wtPath string) {
	t.Helper()
	data, err := os.ReadFile(capture.eventPath)
	if err != nil {
		t.Fatalf("read captured event: %v", err)
	}
	wantJSON := fmt.Sprintf("{\"version\":1,\"event\":\"worktree.removed\",\"operation\":%q,\"repository\":{\"main_checkout\":%q,\"trunk\":\"main\",\"trunk_worktree\":%q},\"worktree\":{\"name\":\"feature\",\"branch\":\"feature\",\"path\":%q}}\n", operation, mainDir, trunkDir, wtPath)
	if got := string(data); got != wantJSON {
		t.Errorf("event JSON = %s, want %s", got, wantJSON)
	}

	var event lifecycleEvent
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("event is not parseable JSON: %v", err)
	}
	want := lifecycleEvent{
		Version:   1,
		Event:     "worktree.removed",
		Operation: operation,
		Repository: eventRepository{
			MainCheckout:  mainDir,
			Trunk:         "main",
			TrunkWorktree: trunkDir,
		},
		Worktree: eventWorktree{Name: "feature", Branch: "feature", Path: wtPath},
	}
	if !reflect.DeepEqual(event, want) {
		t.Errorf("decoded event = %#v, want %#v", event, want)
	}

	assertFileContent(t, capture.cwdPath, trunkDir+"\n")
	assertFileContent(t, capture.environmentPath, "inherited-value")
	if trunkDir == mainDir {
		assertFileContent(t, capture.worktreeEnvPath, "unset")
	} else {
		assertFileContent(t, capture.worktreeEnvPath, "set:main")
	}
	assertFileContent(t, capture.shimEnvPath, "unset")
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if got := string(data); got != want {
		t.Errorf("%s = %q, want %q", path, got, want)
	}
}

func assertNoEvent(t *testing.T, capture eventCapture) {
	t.Helper()
	if _, err := os.Stat(capture.eventPath); !os.IsNotExist(err) {
		t.Fatalf("event handler should not have run, stat err = %v", err)
	}
}

func TestDoneRemovalEvent(t *testing.T) {
	capture := installEventHandler(t)
	mainDir, wtPath := setupDoneRepo(t)

	resetStdio(t)
	if code := cmdDone(nil); code != 0 {
		t.Fatalf("cmdDone exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}
	assertRemovalEvent(t, capture, "done", mainDir, mainDir, wtPath)
}

func TestEventHandlerStripsShellShimEnvironment(t *testing.T) {
	capture := installEventHandler(t)
	t.Setenv("WT_SHIM", "1")
	dir := t.TempDir()
	removal := newRemovalDetails(dir, "main", dir, Worktree{
		Path:   filepath.Join(filepath.Dir(dir), "removed"),
		Branch: "feature",
	})

	emitRemovalEvent("wt done", "done", removal)

	assertFileContent(t, capture.shimEnvPath, "unset")
	assertFileContent(t, capture.worktreeEnvPath, "unset")
}

func TestDoneRemovalEventUsesLinkedTrunkWorktree(t *testing.T) {
	capture := installEventHandler(t)
	mainDir, trunkDir := setupLinkedTrunkRepo(t)
	chdir(t, mainDir)
	resetStdio(t)
	if code := cmdCreate([]string{"feature"}); code != 0 {
		t.Fatalf("setup create failed: %d %s", code, stderrBuf.String())
	}
	wtPath := worktreePath(mainDir, "feature")
	writeFile(t, filepath.Join(wtPath, "feature.txt"), "feature work\n")
	testGit(t, wtPath, "add", ".")
	testGit(t, wtPath, "commit", "-q", "-m", "feature work")
	chdir(t, wtPath)

	resetStdio(t)
	if code := cmdDone(nil); code != 0 {
		t.Fatalf("cmdDone exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}
	assertRemovalEvent(t, capture, "done", mainDir, trunkDir, wtPath)
}

func TestShipRemovalEventRunsAfterPush(t *testing.T) {
	capture := installEventHandler(t)
	mainDir, bareDir, wtPath := setupShipRepo(t)
	t.Setenv("WT_TEST_REMOTE", bareDir)

	resetStdio(t)
	if code := cmdShip(nil); code != 0 {
		t.Fatalf("cmdShip exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}
	assertRemovalEvent(t, capture, "ship", mainDir, mainDir, wtPath)
	assertFileContent(t, capture.remoteHeadPath, testGit(t, mainDir, "rev-parse", "main")+"\n")
}

func TestRmRemovalEvent(t *testing.T) {
	capture := installEventHandler(t)
	dir := initRepo(t)
	chdir(t, dir)
	resetStdio(t)
	if code := cmdCreate([]string{"feature"}); code != 0 {
		t.Fatalf("setup create failed: %d %s", code, stderrBuf.String())
	}
	wtPath := worktreePath(dir, "feature")
	writeFile(t, filepath.Join(wtPath, "unmerged.txt"), "unmerged\n")
	testGit(t, wtPath, "add", ".")
	testGit(t, wtPath, "commit", "-q", "-m", "unmerged work")

	resetStdio(t)
	stdin = strings.NewReader("y\n")
	if code := cmdRm([]string{"feature"}); code != 0 {
		t.Fatalf("cmdRm exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}
	assertRemovalEvent(t, capture, "rm", dir, dir, wtPath)
	if !strings.Contains(stderrBuf.String(), "branch -D") {
		t.Errorf("expected nonfatal branch-deletion warning, got %q", stderrBuf.String())
	}
}

func TestRmRemovalEventFallsBackToMainCheckoutWithoutTrunkWorktree(t *testing.T) {
	capture := installEventHandler(t)
	mainDir, trunkDir := setupLinkedTrunkRepo(t)
	chdir(t, mainDir)
	resetStdio(t)
	if code := cmdCreate([]string{"feature"}); code != 0 {
		t.Fatalf("setup create failed: %d %s", code, stderrBuf.String())
	}
	wtPath := worktreePath(mainDir, "feature")
	testGit(t, mainDir, "worktree", "remove", trunkDir)

	resetStdio(t)
	stdin = strings.NewReader("y\n")
	if code := cmdRm([]string{"feature"}); code != 0 {
		t.Fatalf("cmdRm exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}
	assertRemovalEvent(t, capture, "rm", mainDir, mainDir, wtPath)
}

func TestNoRemovalEventWhenWorktreeIsKept(t *testing.T) {
	t.Run("done --keep", func(t *testing.T) {
		capture := installEventHandler(t)
		setupDoneRepo(t)
		resetStdio(t)
		if code := cmdDone([]string{"--keep"}); code != 0 {
			t.Fatalf("cmdDone exit = %d; stderr = %s", code, stderrBuf.String())
		}
		assertNoEvent(t, capture)
	})

	t.Run("done persistent", func(t *testing.T) {
		capture := installEventHandler(t)
		mainDir, _ := setupDoneRepo(t)
		testGit(t, mainDir, "config", "branch.feature.wt-persist", "true")
		resetStdio(t)
		if code := cmdDone(nil); code != 0 {
			t.Fatalf("cmdDone exit = %d; stderr = %s", code, stderrBuf.String())
		}
		assertNoEvent(t, capture)
	})

	t.Run("ship --keep", func(t *testing.T) {
		capture := installEventHandler(t)
		setupShipRepo(t)
		resetStdio(t)
		if code := cmdShip([]string{"--keep"}); code != 0 {
			t.Fatalf("cmdShip exit = %d; stderr = %s", code, stderrBuf.String())
		}
		assertNoEvent(t, capture)
	})

	t.Run("ship persistent", func(t *testing.T) {
		capture := installEventHandler(t)
		mainDir, _, _ := setupShipRepo(t)
		testGit(t, mainDir, "config", "branch.feature.wt-persist", "true")
		resetStdio(t)
		if code := cmdShip(nil); code != 0 {
			t.Fatalf("cmdShip exit = %d; stderr = %s", code, stderrBuf.String())
		}
		assertNoEvent(t, capture)
	})
}

func TestNoRemovalEventWhenOperationFails(t *testing.T) {
	t.Run("dirty", func(t *testing.T) {
		capture := installEventHandler(t)
		_, wtPath := setupDoneRepo(t)
		writeFile(t, filepath.Join(wtPath, "dirty"), "dirty")
		resetStdio(t)
		if code := cmdDone(nil); code == 0 {
			t.Fatal("cmdDone unexpectedly succeeded")
		}
		assertNoEvent(t, capture)
	})

	t.Run("rebase", func(t *testing.T) {
		capture := installEventHandler(t)
		mainDir, wtPath := setupDoneRepo(t)
		writeFile(t, filepath.Join(mainDir, "README.md"), "trunk\n")
		testGit(t, mainDir, "commit", "-am", "trunk change")
		writeFile(t, filepath.Join(wtPath, "README.md"), "feature\n")
		testGit(t, wtPath, "commit", "-am", "feature change")
		resetStdio(t)
		if code := cmdDone(nil); code == 0 {
			t.Fatal("cmdDone unexpectedly succeeded")
		}
		assertNoEvent(t, capture)
	})

	t.Run("removal", func(t *testing.T) {
		capture := installEventHandler(t)
		_, wtPath := setupDoneRepo(t)
		hook := filepath.Join(wtPath, ".wt", "destroy")
		writeFile(t, hook, "#!/bin/sh\nprintf dirty > late-dirty\n")
		if err := os.Chmod(hook, 0o755); err != nil {
			t.Fatal(err)
		}
		testGit(t, wtPath, "add", ".wt/destroy")
		testGit(t, wtPath, "commit", "-q", "-m", "add destroy hook")
		resetStdio(t)
		if code := cmdDone(nil); code == 0 {
			t.Fatal("cmdDone unexpectedly succeeded")
		}
		assertNoEvent(t, capture)
	})

	t.Run("fetch", func(t *testing.T) {
		capture := installEventHandler(t)
		_, wtPath := setupDoneRepo(t)
		chdir(t, wtPath)
		resetStdio(t)
		if code := cmdShip(nil); code == 0 {
			t.Fatal("cmdShip unexpectedly succeeded")
		}
		assertNoEvent(t, capture)
	})

	t.Run("push", func(t *testing.T) {
		capture := installEventHandler(t)
		_, bareDir, _ := setupShipRepo(t)
		hook := filepath.Join(bareDir, "hooks", "pre-receive")
		writeFile(t, hook, "#!/bin/sh\nexit 1\n")
		if err := os.Chmod(hook, 0o755); err != nil {
			t.Fatal(err)
		}
		resetStdio(t)
		if code := cmdShip(nil); code == 0 {
			t.Fatal("cmdShip unexpectedly succeeded")
		}
		assertNoEvent(t, capture)
	})
}

func TestEventHandlerFailuresDoNotChangeSuccessfulStatus(t *testing.T) {
	t.Run("spawn", func(t *testing.T) {
		t.Setenv("WT_EVENT_HANDLER", filepath.Join(t.TempDir(), "missing"))
		setupDoneRepo(t)
		resetStdio(t)
		if code := cmdDone(nil); code != 0 {
			t.Fatalf("cmdDone exit = %d, want 0", code)
		}
		if !strings.Contains(stderrBuf.String(), "operation already succeeded") {
			t.Errorf("missing successful-operation warning: %q", stderrBuf.String())
		}
	})

	t.Run("nonzero", func(t *testing.T) {
		dir := t.TempDir()
		handler := filepath.Join(dir, "handler")
		writeFile(t, handler, "#!/bin/sh\nexit 9\n")
		if err := os.Chmod(handler, 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("WT_EVENT_HANDLER", handler)
		setupDoneRepo(t)
		resetStdio(t)
		if code := cmdDone(nil); code != 0 {
			t.Fatalf("cmdDone exit = %d, want 0", code)
		}
		if !strings.Contains(stderrBuf.String(), "operation already succeeded") {
			t.Errorf("missing successful-operation warning: %q", stderrBuf.String())
		}
	})
}

func TestEmptyEventHandlerLeavesOperationUnchanged(t *testing.T) {
	t.Setenv("WT_EVENT_HANDLER", "")
	setupDoneRepo(t)
	resetStdio(t)
	if code := cmdDone(nil); code != 0 {
		t.Fatalf("cmdDone exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}
	if strings.Contains(stderrBuf.String(), "WT_EVENT_HANDLER") {
		t.Errorf("empty handler should be disabled, got %q", stderrBuf.String())
	}
}

func TestHelpEventsDocumentsContract(t *testing.T) {
	resetStdio(t)
	if code := cmdHelp([]string{"events"}); code != 0 {
		t.Fatalf("cmdHelp exit = %d", code)
	}
	for _, want := range []string{"WT_EVENT_HANDLER", "worktree.removed", `"version": 1`, "still exits successfully"} {
		if !strings.Contains(stdoutBuf.String(), want) {
			t.Errorf("event help missing %q", want)
		}
	}
}
