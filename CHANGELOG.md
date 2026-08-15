# Changelog

## Unreleased

- Renamed and expanded the exported agent skill to `wt`, supporting requests such as `/wt ship`, `/wt merge`, and `/wt create a new workspace`; exporting now takes an explicit skills directory.

## 0.2

- Added `-v` and `--version` flags for printing the installed version.
- Bare `wt` now prints usage notes; use `wt switch` without a name to pick a worktree.

## 0.1

- Released ephemeral Git worktree creation, cleanup, regrouping, and shipping.
- Automated macOS, Linux, and Windows releases and Homebrew distribution.
