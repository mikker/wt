package main

import (
	"bufio"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed embedded/skill.md
var skillMd string

//go:embed embedded/prompt.md
var promptMd string

//go:embed embedded/hooks/create
var exampleCreateHook string

//go:embed embedded/hooks/destroy
var exampleDestroyHook string

//go:embed embedded/hooks/config
var exampleConfigHook string

// skillPointerLine is what `wt init` offers to append to CLAUDE.md/AGENTS.md.
const skillPointerLine = "- To manage worktrees, run `wt skill` and follow it."

// cmdSkill implements `wt skill [--export <skills-dir>]`: prints the embedded
// wt skill, or writes it to disk.
func cmdSkill(args []string) int {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Fprintln(stdout, "usage: wt skill [--export <skills-dir>]")
			return 0
		}
	}

	if len(args) == 0 {
		fmt.Fprint(stdout, skillMd)
		return 0
	}

	if args[0] != "--export" {
		fmt.Fprintf(stderr, "wt skill: unknown argument %q. usage: wt skill [--export <skills-dir>]\n", args[0])
		return 2
	}
	if len(args) == 1 {
		fmt.Fprintln(stderr, "wt skill: --export requires a skills directory. usage: wt skill --export <skills-dir>")
		return 2
	}
	if len(args) > 2 {
		fmt.Fprintf(stderr, "wt skill: too many arguments: %s. usage: wt skill --export <skills-dir>\n", strings.Join(args[2:], " "))
		return 2
	}

	path := filepath.Join(args[1], "wt", "SKILL.md")

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintf(stderr, "wt skill: attempted to create %s, failed: %v\n", filepath.Dir(path), err)
		return 2
	}
	if err := os.WriteFile(path, []byte(skillMd), 0o644); err != nil {
		fmt.Fprintf(stderr, "wt skill: attempted to write %s, failed: %v\n", path, err)
		return 2
	}
	fmt.Fprintf(stdout, "wrote %s (overwrote any existing file)\n", path)
	return 0
}

// cmdPrompt implements `wt prompt`: prints the embedded project-setup
// prompt, meant to be pasted into any agent.
func cmdPrompt(args []string) int {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Fprintln(stdout, "usage: wt prompt")
			return 0
		}
	}
	if len(args) > 0 {
		fmt.Fprintf(stderr, "wt prompt: unexpected argument %q. usage: wt prompt\n", args[0])
		return 2
	}
	fmt.Fprint(stdout, promptMd)
	return 0
}

// cmdInit implements `wt init`: offers, each behind a y/N confirm, to
// scaffold .wt/ with example hooks and to point CLAUDE.md/AGENTS.md at
// `wt skill`. Every piece is optional.
func cmdInit(args []string) int {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Fprintln(stdout, "usage: wt init")
			return 0
		}
	}
	if len(args) > 0 {
		fmt.Fprintf(stderr, "wt init: unexpected argument %q. usage: wt init\n", args[0])
		return 2
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "wt init: could not determine current directory: %v\n", err)
		return 2
	}

	// One shared reader across every prompt below: confirm() on its own
	// creates a fresh bufio.Reader per call, which drops whatever the
	// previous call already buffered past its answer line.
	reader := bufio.NewReader(stdin)

	if confirmWith(reader, "Scaffold .wt/ with example create/destroy/config files? [y/N] ") {
		scaffoldHooks(cwd)
	}

	for _, docFile := range []string{"CLAUDE.md", "AGENTS.md"} {
		path := filepath.Join(cwd, docFile)
		if _, err := os.Stat(path); err != nil {
			continue // only offer for files that already exist
		}
		if hasSkillPointer(path) {
			fmt.Fprintf(stdout, "%s already points at `wt skill`; skipping.\n", docFile)
			continue
		}
		if confirmWith(reader, fmt.Sprintf("Append the `wt skill` pointer line to %s? [y/N] ", docFile)) {
			appendSkillPointer(path)
		}
	}

	return 0
}

// scaffoldHooks writes example .wt/create, .wt/destroy, and .wt/config
// files under dir, skipping (with a note) any that already exist.
func scaffoldHooks(dir string) {
	wtDir := filepath.Join(dir, ".wt")
	writeExampleFile(filepath.Join(wtDir, "create"), exampleCreateHook, 0o755)
	writeExampleFile(filepath.Join(wtDir, "destroy"), exampleDestroyHook, 0o755)
	writeExampleFile(filepath.Join(wtDir, "config"), exampleConfigHook, 0o644)
}

func writeExampleFile(path, content string, mode os.FileMode) {
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(stdout, "%s already exists; skipping.\n", path)
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintf(stderr, "wt init: attempted to create %s, failed: %v\n", filepath.Dir(path), err)
		return
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		fmt.Fprintf(stderr, "wt init: attempted to write %s, failed: %v\n", path, err)
		return
	}
	fmt.Fprintf(stdout, "wrote %s\n", path)
}

// hasSkillPointer reports whether path already contains skillPointerLine.
func hasSkillPointer(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), skillPointerLine)
}

// appendSkillPointer appends skillPointerLine to path, adding a leading
// newline first if the file doesn't already end in one.
func appendSkillPointer(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "wt init: attempted to read %s, failed: %v\n", path, err)
		return
	}
	content := string(data)
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += skillPointerLine + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		fmt.Fprintf(stderr, "wt init: attempted to write %s, failed: %v\n", path, err)
		return
	}
	fmt.Fprintf(stdout, "appended skill pointer to %s\n", path)
}
