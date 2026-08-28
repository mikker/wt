package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
)

var errReflinkUnsupported = errors.New("reflinks unsupported")

type reflinkStats struct {
	files int
	bytes int64
}

type cloneContext struct {
	sourceRoot    string
	targetRoot    string
	worktreesRoot string
	ignore        *ignore.GitIgnore
	stats         reflinkStats
}

// ignoredEntries returns the paths Git deliberately leaves out of a fresh
// worktree. Git has already collapsed ignored directories to their roots.
func ignoredEntries(root string) ([]string, error) {
	out, err := runGit(root, "status", "--porcelain", "-z", "--ignored")
	if err != nil {
		return nil, err
	}

	var entries []string
	for entry := range strings.SplitSeq(out, "\x00") {
		if path, ok := strings.CutPrefix(entry, "!! "); ok {
			path = strings.TrimSuffix(path, "/")
			if path != "" && path != ".git" {
				entries = append(entries, filepath.FromSlash(path))
			}
		}
	}
	return entries, nil
}

// carryIgnoredFiles reflinks ignored files from the main checkout into a new
// worktree. Unsupported filesystems keep the ordinary, clean worktree Git made.
func carryIgnoredFiles(mainCheckout, worktreeDir, worktreesRoot string, entries []string) error {
	if len(entries) == 0 {
		return nil
	}
	matcher, err := loadCarryIgnore(mainCheckout)
	if err != nil {
		return fmt.Errorf("read .wt/ignore: %w", err)
	}
	entries = slices.DeleteFunc(entries, func(entry string) bool {
		return pathWithin(filepath.Join(mainCheckout, entry), worktreesRoot) ||
			matcher != nil && matcher.MatchesPath(filepath.ToSlash(entry))
	})
	if len(entries) == 0 {
		return nil
	}

	supported, err := probeReflink(mainCheckout, worktreesRoot)
	if err != nil {
		return fmt.Errorf("probe reflink support: %w", err)
	}
	if !supported {
		return nil
	}

	actionHeadline("Cloning ignored files into %s", worktreeDir)
	clone := cloneContext{
		sourceRoot:    mainCheckout,
		targetRoot:    worktreeDir,
		worktreesRoot: worktreesRoot,
		ignore:        matcher,
	}
	for _, entry := range entries {
		source := filepath.Join(mainCheckout, entry)
		target := filepath.Join(worktreeDir, entry)
		if err := clone.tree(source, target); err != nil {
			return fmt.Errorf("clone ignored entry %s: %w", entry, err)
		}
	}
	if clone.stats.files > 0 {
		fmt.Fprintf(stderr, "Cloned %d files (%.1f MiB shared)\n", clone.stats.files, float64(clone.stats.bytes)/(1024*1024))
	}
	return nil
}

func loadCarryIgnore(root string) (*ignore.GitIgnore, error) {
	path := filepath.Join(root, ".wt", "ignore")
	matcher, err := ignore.CompileIgnoreFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return matcher, err
}

func probeReflink(sourceRoot, destinationRoot string) (bool, error) {
	if err := os.MkdirAll(destinationRoot, 0o755); err != nil {
		return false, err
	}
	source, err := os.CreateTemp(sourceRoot, ".wt-reflink-probe-")
	if err != nil {
		return false, err
	}
	defer source.Close()
	sourcePath := source.Name()
	defer os.Remove(sourcePath)
	destination, err := os.CreateTemp(destinationRoot, ".wt-reflink-probe-")
	if err != nil {
		return false, err
	}
	destinationPath := destination.Name()
	if err := destination.Close(); err != nil {
		return false, err
	}
	defer os.Remove(destinationPath)
	if err := os.Remove(destinationPath); err != nil {
		return false, err
	}

	if _, err := source.Write(make([]byte, 4096)); err != nil {
		return false, err
	}
	if err := source.Close(); err != nil {
		return false, err
	}

	if err := reflinkFile(sourcePath, destinationPath, 0o600); err != nil {
		if errors.Is(err, errReflinkUnsupported) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (clone *cloneContext) tree(source, target string) error {
	info, err := os.Lstat(source)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if clone.excluded(source) {
		return nil
	}
	if pathWithin(source, clone.worktreesRoot) {
		return nil
	}
	if _, err := os.Lstat(target); err == nil && !info.IsDir() {
		return nil
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	switch {
	case info.Mode()&os.ModeSymlink != 0:
		link, err := os.Readlink(source)
		if err != nil {
			return err
		}
		link = retargetSymlink(link, clone.sourceRoot, clone.targetRoot)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.Symlink(link, target); err != nil && !os.IsExist(err) {
			return err
		}
		return nil
	case info.IsDir():
		if err := os.MkdirAll(target, info.Mode().Perm()); err != nil {
			return err
		}
		children, err := os.ReadDir(source)
		if err != nil {
			return err
		}
		for _, child := range children {
			if err := clone.tree(filepath.Join(source, child.Name()), filepath.Join(target, child.Name())); err != nil {
				return err
			}
		}
		return nil
	case info.Mode().IsRegular():
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := reflinkFile(source, target, info.Mode().Perm()); err != nil {
			if os.IsExist(err) {
				return nil
			}
			return err
		}
		clone.stats.files++
		clone.stats.bytes += info.Size()
	}
	return nil
}

func (clone *cloneContext) excluded(path string) bool {
	if clone.ignore == nil {
		return false
	}
	relative, err := filepath.Rel(clone.sourceRoot, path)
	return err == nil && clone.ignore.MatchesPath(filepath.ToSlash(relative))
}

func retargetSymlink(link, sourceRoot, targetRoot string) string {
	if !filepath.IsAbs(link) {
		return link
	}
	for _, root := range pathSpellings(sourceRoot) {
		if rel, err := filepath.Rel(root, link); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return filepath.Join(targetRoot, rel)
		}
	}
	return link
}

func pathSpellings(path string) []string {
	paths := []string{path}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return paths
	}
	if resolved != path {
		paths = append(paths, resolved)
	}
	private := string(os.PathSeparator) + "private" + string(os.PathSeparator)
	if rest, ok := strings.CutPrefix(resolved, private); ok {
		alias := string(os.PathSeparator) + rest
		if aliasResolved, err := filepath.EvalSymlinks(alias); err == nil && aliasResolved == resolved {
			paths = append(paths, alias)
		}
	}
	return paths
}

func pathWithin(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
