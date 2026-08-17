---
layout: default
title: Project setup
description: Configure wt hooks and persistent worktrees.
permalink: /projects/
---

# Project setup

Zero configuration works. Run `wt init` when a repository needs dependencies,
secrets, generated files, or teardown work in every new worktree.

| File | Contract |
|---|---|
| `.wt/create` | Executable hook run in the new worktree. `$1` is the absolute main checkout path. |
| `.wt/destroy` | Executable cleanup hook run before worktree removal. Failure warns but does not block removal. |
| `.wt/config` | `key = value` configuration with `#` comments. |

Hooks only run when executable:

```sh
chmod +x .wt/create .wt/destroy
```

## Persistence

Worktrees are removed after a successful `done` or `ship` unless persistence
is enabled. Resolution is first-match-wins:

1. `--rm` or `--keep` on `wt done` / `wt ship`
2. Branch preference set by `wt persist` or `wt create --persist`
3. `persistent = true` in `.wt/config`
4. Ephemeral by default

Persistent worktrees are rebased in place after merging and remain ready for
the next task.
