package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestIgnoredEntriesComeFromGitIgnoreRules(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, filepath.Join(dir, ".gitignore"), "node_modules/\n.env\n")
	writeFile(t, filepath.Join(dir, ".env"), "SECRET=1\n")
	writeFile(t, filepath.Join(dir, "packages", "app", "index.js"), "export default 1\n")
	writeFile(t, filepath.Join(dir, "packages", "app", "node_modules", "dep"), "cached\n")
	testGit(t, dir, "add", ".gitignore", "packages/app/index.js")
	testGit(t, dir, "commit", "-q", "-m", "ignore local files")

	entries, err := ignoredEntries(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(entries, ".env") {
		t.Fatalf("ignoredEntries = %q, want .env", entries)
	}
	if !slices.Contains(entries, filepath.Join("packages", "app", "node_modules")) {
		t.Fatalf("ignoredEntries = %q, want nested node_modules", entries)
	}
}

func TestCreateReflinksIgnoredFilesWhenSupported(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, filepath.Join(dir, ".gitignore"), "cache/\n.env\n")
	writeFile(t, filepath.Join(dir, ".env"), "SECRET=1\n")
	writeFile(t, filepath.Join(dir, "cache", "tool"), "parent\n")
	writeFile(t, filepath.Join(dir, ".wt", "config"), "carry_ignored = true\n")
	testGit(t, dir, "add", ".gitignore", ".wt/config")
	testGit(t, dir, "commit", "-q", "-m", "ignore local files")

	supported, err := probeReflink(dir, worktreesDir(dir))
	if err != nil {
		t.Fatal(err)
	}
	if supported {
		linkTarget := filepath.Join(dir, "cache", "tool")
		if alias, ok := strings.CutPrefix(linkTarget, "/private"); ok {
			if resolved, err := filepath.EvalSymlinks(alias); err == nil && resolved == linkTarget {
				linkTarget = alias
			}
		}
		if err := os.Symlink(linkTarget, filepath.Join(dir, "cache", "tool-link")); err != nil {
			t.Fatal(err)
		}
	}
	chdir(t, dir)
	resetStdio(t)
	if code := cmdCreate([]string{"warm"}); code != 0 {
		t.Fatalf("cmdCreate exit = %d; stderr = %s", code, stderrBuf.String())
	}

	worktree := worktreePath(dir, "warm")
	carried := filepath.Join(worktree, ".env")
	if !supported {
		if _, err := os.Lstat(carried); !os.IsNotExist(err) {
			t.Fatalf("ignored file was copied without reflink support: %v", err)
		}
		return
	}

	if got, err := os.ReadFile(carried); err != nil || string(got) != "SECRET=1\n" {
		t.Fatalf("carried .env = %q, %v", got, err)
	}
	writeFile(t, filepath.Join(worktree, "cache", "tool"), "worktree\n")
	if got, err := os.ReadFile(filepath.Join(dir, "cache", "tool")); err != nil || string(got) != "parent\n" {
		t.Fatalf("source changed through reflink: %q, %v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(worktree, "cache", "tool-link")); err != nil || string(got) != "worktree\n" {
		t.Fatalf("absolute symlink still reaches source worktree: %q, %v", got, err)
	}
}

func TestCloneTreeDoesNotOverwriteExistingFile(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	writeFile(t, filepath.Join(sourceRoot, "cache", "value"), "source\n")
	writeFile(t, filepath.Join(targetRoot, "cache", "value"), "target\n")

	supported, err := probeReflink(sourceRoot, targetRoot)
	if err != nil {
		t.Fatal(err)
	}
	if !supported {
		t.Skip("reflinks unavailable")
	}
	clone := cloneContext{
		sourceRoot:    sourceRoot,
		targetRoot:    targetRoot,
		worktreesRoot: filepath.Join(sourceRoot, ".wt", "worktrees"),
	}
	if err := clone.tree(filepath.Join(sourceRoot, "cache"), filepath.Join(targetRoot, "cache")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(targetRoot, "cache", "value"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "target\n" {
		t.Fatalf("existing target overwritten: %q", got)
	}
}

func TestCreateDoesNotCarryIgnoredFilesByDefault(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, filepath.Join(dir, ".gitignore"), ".env\n")
	writeFile(t, filepath.Join(dir, ".env"), "SECRET=1\n")
	testGit(t, dir, "add", ".gitignore")
	testGit(t, dir, "commit", "-q", "-m", "configure wt")

	chdir(t, dir)
	resetStdio(t)
	if code := cmdCreate([]string{"clean"}); code != 0 {
		t.Fatalf("cmdCreate exit = %d; stderr = %s", code, stderrBuf.String())
	}
	if _, err := os.Stat(filepath.Join(worktreePath(dir, "clean"), ".env")); !os.IsNotExist(err) {
		t.Fatalf(".env was carried without carry_ignored = true: %v", err)
	}
	if strings.Contains(stderrBuf.String(), "Cloning ignored files") {
		t.Fatalf("reflink work ran without carry_ignored = true:\n%s", stderrBuf.String())
	}
}

func TestCreateRespectsCarryIgnorePatterns(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, filepath.Join(dir, ".gitignore"), "cache/\n")
	writeFile(t, filepath.Join(dir, ".wt", "config"), "carry_ignored = true\n")
	writeFile(t, filepath.Join(dir, ".wt", "ignore"), "cache/private/\n*.sock\n!keep.sock\n")
	writeFile(t, filepath.Join(dir, "cache", "private", "secret"), "secret\n")
	writeFile(t, filepath.Join(dir, "cache", "public", "value"), "public\n")
	writeFile(t, filepath.Join(dir, "cache", "drop.sock"), "drop\n")
	writeFile(t, filepath.Join(dir, "cache", "keep.sock"), "keep\n")
	testGit(t, dir, "add", ".gitignore", ".wt/config", ".wt/ignore")
	testGit(t, dir, "commit", "-q", "-m", "configure wt ignore")

	supported, err := probeReflink(dir, worktreesDir(dir))
	if err != nil {
		t.Fatal(err)
	}
	if !supported {
		t.Skip("reflinks unavailable")
	}
	chdir(t, dir)
	resetStdio(t)
	if code := cmdCreate([]string{"selective"}); code != 0 {
		t.Fatalf("cmdCreate exit = %d; stderr = %s", code, stderrBuf.String())
	}

	worktree := worktreePath(dir, "selective")
	for _, path := range []string{"cache/private/secret", "cache/drop.sock"} {
		if _, err := os.Stat(filepath.Join(worktree, filepath.FromSlash(path))); !os.IsNotExist(err) {
			t.Errorf("excluded %s was carried: %v", path, err)
		}
	}
	for _, path := range []string{"cache/public/value", "cache/keep.sock"} {
		if _, err := os.Stat(filepath.Join(worktree, filepath.FromSlash(path))); err != nil {
			t.Errorf("included %s was not carried: %v", path, err)
		}
	}
}
