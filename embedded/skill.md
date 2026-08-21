---
name: wt
description: Manage Git worktrees with wt. Use to create, switch, list, remove, persist, merge, or ship worktrees.
---

Use `wt` to carry out the requested worktree operation:

- Create a worktree: `wt create <name>`. To immediately start a command there,
  use `wt create <name> -- <command> [args...]`. Derive a short,
  branch-compatible name from the task when possible; ask for one when the
  request has no naming context.
- Enter an existing worktree: `wt switch <name>`, or `wt switch` to pick one.
- List worktrees: `wt ls`.
- Remove a worktree without merging: `wt rm [<name>]`.
- Toggle persistence for the current worktree: `wt persist`.
- Merge the current worktree into local trunk: `wt done`.
- Sync with the remote, merge, and push trunk: `wt ship`.

By default, worktrees live in the project-local `.wt/worktrees/<name>`
directory. `wt` excludes it through `.git/info/exclude`, so do not add it to
the project's `.gitignore`. A project can override the location with
`worktrees = <path>` in `.wt/config`; relative paths resolve from the main
checkout.

Projects can also commit executable hooks in `.wt/`: `.wt/create` runs in a
new worktree after it is added, with the main checkout's absolute path as
`$1`; `.wt/destroy` runs in a worktree before it is removed. Consult
`wt help hooks` before creating or changing either hook.

When integrating an external tool with successful worktree teardown, consult
`wt help events` for the opt-in `WT_EVENT_HANDLER` JSON event contract.

For merge and ship requests, run the command and handle resolvable stops:

- If it stops on a rebase conflict: inspect, resolve, `git add`, `git rebase
  --continue`, then re-run the same `wt` command.
- If it stops on dirty state: commit or stash as appropriate, then re-run.

Never non-ff merge into trunk. Never push unless the user asked for that
(use `wt ship`, not a manual `git push`).

`wt --help` documents all commands.
