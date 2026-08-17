# Changelog

## Unreleased

## 0.5

- Worktrees now live inside `.wt/worktrees` by default; set `worktrees = <path>` in `.wt/config` to choose another location.

## 0.4

- Added `wt create <name> -- <command> [args...]` to run a command in a new worktree as soon as its create hook completes.
- Added opt-in `WT_EVENT_HANDLER` JSON notifications after successful worktree removal by `wt done`, `wt ship`, or `wt rm`.

## 0.3

- Renamed and expanded the exported agent skill to `wt`, supporting requests such as `/wt ship`, `/wt merge`, and `/wt create a new workspace`; exporting now takes an explicit skills directory.
- Added `wt create` (alias `wt c`); `wt switch` now only enters existing worktrees and never creates one implicitly.

## 0.2

- Added `-v` and `--version` flags for printing the installed version.
- Bare `wt` now prints usage notes; use `wt switch` without a name to pick a worktree.

## 0.1

- Released ephemeral Git worktree creation, cleanup, regrouping, and shipping.
- Automated macOS, Linux, and Windows releases and Homebrew distribution.
