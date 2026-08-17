---
layout: default
title: Commands
description: wt commands, arguments, and options.
permalink: /commands/
---

# Commands

<div class="wide-table" markdown="1">

| Command | What it does | Options |
|---|---|---|
| `wt create <name>` | Create and enter a worktree. Alias: `c`. | `--persist`; `-- <command> [args...]` runs a command after setup. |
| `wt switch [<name>]` | Enter an existing worktree, or pick one. Alias: `s`. | Uses `fzf` when available, otherwise a numbered picker. |
| `wt ls` | List worktrees, branch state, dirt, and persistence. | — |
| `wt done` | Rebase onto local trunk, fast-forward trunk, then remove the worktree. | `--keep` preserves it; `--rm` forces removal. |
| `wt ship` | Fetch, fast-forward trunk, run the `done` flow, and push trunk. | `--keep` preserves it; `--rm` forces removal. |
| `wt rm [<name>]` | Remove a worktree without merging. | `--force` discards uncommitted changes. |
| `wt persist` | Toggle persistence for the current worktree. | — |
| `wt init` | Scaffold optional `.wt/` project files and agent pointers. | — |
| `wt prompt` | Print a project-setup prompt for an agent. | — |
| `wt skill` | Print the bundled agent skill. | `--export <skills-dir>` writes `wt/SKILL.md`. |
| `wt shellenv [zsh\|bash]` | Print shell integration. | Defaults to zsh. |

</div>

All commands accept `-h` or `--help`. `wt -v` and `wt --version` print the
installed version.

## Everyday flow

```sh
wt create fix-login
# work and commit
wt done
```

`done` never uses the network. Use `ship` when the request includes syncing
and pushing:

```sh
wt create add-search -- pi
# work and commit
wt ship
```

Both commands stop rather than guess when a worktree is dirty, trunk has
diverged, or a rebase conflicts. Resolve the condition and run the same
command again.
