package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillPrintsFrontmatterAndBody(t *testing.T) {
	resetStdio(t)
	code := cmdSkill(nil)
	if code != 0 {
		t.Fatalf("cmdSkill exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}
	out := stdoutBuf.String()
	if !strings.HasPrefix(out, "---\nname: regroup\n") {
		t.Errorf("expected yaml frontmatter at the top, got:\n%s", out)
	}
	if !strings.Contains(out, "wt done") || !strings.Contains(out, "--continue") {
		t.Errorf("expected the skill body to mention wt done and --continue, got:\n%s", out)
	}
}

func TestSkillExportDefaultPath(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	resetStdio(t)

	code := cmdSkill([]string{"--export"})
	if code != 0 {
		t.Fatalf("cmdSkill --export exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}

	want := filepath.Join(dir, ".claude", "skills", "regroup", "SKILL.md")
	data, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("expected %s to exist: %v", want, err)
	}
	if string(data) != skillMd {
		t.Errorf("exported content differs from embedded skill.md")
	}
	if !strings.Contains(stdoutBuf.String(), defaultSkillExportPath) {
		t.Errorf("expected confirmation to mention %s, got %q", defaultSkillExportPath, stdoutBuf.String())
	}
}

func TestSkillExportCustomPath(t *testing.T) {
	dir := t.TempDir()
	custom := filepath.Join(dir, "nested", "dir", "SKILL.md")
	resetStdio(t)

	code := cmdSkill([]string{"--export", custom})
	if code != 0 {
		t.Fatalf("cmdSkill --export %s exit = %d, want 0; stderr = %s", custom, code, stderrBuf.String())
	}
	data, err := os.ReadFile(custom)
	if err != nil {
		t.Fatalf("expected %s to exist: %v", custom, err)
	}
	if string(data) != skillMd {
		t.Errorf("exported content differs from embedded skill.md")
	}
}

func TestPromptPrints(t *testing.T) {
	resetStdio(t)
	code := cmdPrompt(nil)
	if code != 0 {
		t.Fatalf("cmdPrompt exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}
	out := stdoutBuf.String()
	for _, want := range []string{"wt --help", "wt help hooks", ".wt/create", ".wt/destroy", "wt switch", "wt rm", "persistent = true"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected prompt output to mention %q, got:\n%s", want, out)
		}
	}
}

func TestInitScaffoldsHooksExecutable(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	resetStdio(t)
	stdin = strings.NewReader("y\nn\nn\n")

	if code := cmdInit(nil); code != 0 {
		t.Fatalf("cmdInit exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}

	for _, name := range []string{"create", "destroy", "config"} {
		path := filepath.Join(dir, ".wt", name)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("expected %s to be written: %v", path, err)
		}
		if name != "config" && info.Mode().Perm()&0o111 == 0 {
			t.Errorf("%s should be executable, got mode %v", path, info.Mode())
		}
	}
}

func TestInitSkipsExistingHookFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".wt", "create"), "#!/bin/sh\n# my custom hook\n")
	chdir(t, dir)
	resetStdio(t)
	stdin = strings.NewReader("y\n")

	if code := cmdInit(nil); code != 0 {
		t.Fatalf("cmdInit exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}

	data, err := os.ReadFile(filepath.Join(dir, ".wt", "create"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "#!/bin/sh\n# my custom hook\n" {
		t.Errorf("existing .wt/create should not be overwritten, got:\n%s", data)
	}
	if !strings.Contains(stdoutBuf.String(), "already exists; skipping") {
		t.Errorf("expected a skip note for the existing file, got:\n%s", stdoutBuf.String())
	}
	// destroy and config didn't exist, so they should have been written.
	if _, err := os.Stat(filepath.Join(dir, ".wt", "destroy")); err != nil {
		t.Errorf("expected .wt/destroy to be written: %v", err)
	}
}

func TestInitAppendsPointerLineIdempotently(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "CLAUDE.md"), "# Project notes\n")
	chdir(t, dir)

	resetStdio(t)
	stdin = strings.NewReader("n\ny\n") // skip scaffold, append to CLAUDE.md
	if code := cmdInit(nil); code != 0 {
		t.Fatalf("cmdInit exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}

	data, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	first := string(data)
	if !strings.Contains(first, skillPointerLine) {
		t.Fatalf("expected CLAUDE.md to contain the skill pointer line, got:\n%s", first)
	}
	if strings.Count(first, skillPointerLine) != 1 {
		t.Fatalf("expected exactly one pointer line, got:\n%s", first)
	}

	// Re-run: should detect the existing line and not offer/append again.
	resetStdio(t)
	stdin = strings.NewReader("n\ny\n")
	if code := cmdInit(nil); code != 0 {
		t.Fatalf("second cmdInit exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}
	data, err = os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), skillPointerLine) != 1 {
		t.Fatalf("expected pointer line to still appear exactly once after re-running init, got:\n%s", data)
	}
	if !strings.Contains(stdoutBuf.String(), "already points at") {
		t.Errorf("expected a skip note on the second run, got:\n%s", stdoutBuf.String())
	}
}

func TestInitDoesNotOfferForAbsentDocFiles(t *testing.T) {
	dir := t.TempDir() // no CLAUDE.md, no AGENTS.md
	chdir(t, dir)
	resetStdio(t)
	stdin = strings.NewReader("n\n") // only the scaffold prompt should appear

	if code := cmdInit(nil); code != 0 {
		t.Fatalf("cmdInit exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}
	if strings.Contains(stderrBuf.String(), "CLAUDE.md") || strings.Contains(stderrBuf.String(), "AGENTS.md") {
		t.Errorf("should not have prompted about absent doc files, got stderr:\n%s", stderrBuf.String())
	}
}

func TestHelpListsAllCommands(t *testing.T) {
	resetStdio(t)
	code := cmdHelp(nil)
	if code != 0 {
		t.Fatalf("cmdHelp exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}
	out := stdoutBuf.String()
	for name := range commands {
		if !strings.Contains(out, name) {
			t.Errorf("wt help output missing command %q:\n%s", name, out)
		}
	}
}

func TestHelpHooksMentionsCreateDestroyConfig(t *testing.T) {
	resetStdio(t)
	code := cmdHelp([]string{"hooks"})
	if code != 0 {
		t.Fatalf("cmdHelp hooks exit = %d, want 0; stderr = %s", code, stderrBuf.String())
	}
	out := stdoutBuf.String()
	for _, want := range []string{".wt/create", ".wt/destroy", ".wt/config", "persistent"} {
		if !strings.Contains(out, want) {
			t.Errorf("wt help hooks missing %q:\n%s", want, out)
		}
	}
}

func TestHelpUnknownTopic(t *testing.T) {
	resetStdio(t)
	code := cmdHelp([]string{"bogus"})
	if code != 2 {
		t.Fatalf("cmdHelp bogus exit = %d, want 2; stderr = %s", code, stderrBuf.String())
	}
}
