package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

//go:embed embedded/events.md
var eventsHelp string

const worktreeRemovedEvent = "worktree.removed"

type eventRepository struct {
	MainCheckout  string `json:"main_checkout"`
	Trunk         string `json:"trunk"`
	TrunkWorktree string `json:"trunk_worktree"`
}

type eventWorktree struct {
	Name   string `json:"name"`
	Branch string `json:"branch"`
	Path   string `json:"path"`
}

type lifecycleEvent struct {
	Version    int             `json:"version"`
	Event      string          `json:"event"`
	Operation  string          `json:"operation"`
	Repository eventRepository `json:"repository"`
	Worktree   eventWorktree   `json:"worktree"`
}

type removalDetails struct {
	Repository eventRepository
	Worktree   eventWorktree
}

func newRemovalDetails(mainCheckout, trunk, trunkWorktree string, worktree Worktree) removalDetails {
	name := worktree.Branch
	if name == "" {
		name = filepath.Base(worktree.Path)
	}
	return removalDetails{
		Repository: eventRepository{
			MainCheckout:  mainCheckout,
			Trunk:         trunk,
			TrunkWorktree: trunkWorktree,
		},
		Worktree: eventWorktree{
			Name:   name,
			Branch: worktree.Branch,
			Path:   worktree.Path,
		},
	}
}

func emitRemovalEvent(cmdName, operation string, removal removalDetails) {
	handler := os.Getenv("WT_EVENT_HANDLER")
	if handler == "" {
		return
	}

	event := lifecycleEvent{
		Version:    1,
		Event:      worktreeRemovedEvent,
		Operation:  operation,
		Repository: removal.Repository,
		Worktree:   removal.Worktree,
	}
	var input bytes.Buffer
	if err := json.NewEncoder(&input).Encode(event); err != nil {
		fmt.Fprintf(stderr, "%s: operation already succeeded, but encoding the lifecycle event failed: %v\n", cmdName, err)
		return
	}

	cmd := exec.Command(handler)
	cmd.Dir = removal.Repository.TrunkWorktree
	cmd.Env = stripEnv(cmd.Environ(), "WT_SHIM")
	if removal.Repository.TrunkWorktree == removal.Repository.MainCheckout {
		cmd.Env = stripEnv(cmd.Env, "WORKTREE")
	} else {
		cmd.Env = setEnv(cmd.Env, "WORKTREE", removal.Repository.Trunk)
	}
	cmd.Stdin = &input
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(stderr, "%s: operation already succeeded, but WT_EVENT_HANDLER %q failed: %v\n", cmdName, handler, err)
	}
}
