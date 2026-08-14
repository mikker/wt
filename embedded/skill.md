---
name: regroup
description: Merge worktree work back to trunk with wt done/wt ship, resolving rebase conflicts and dirty state as they come up.
---

Run `wt done` (or `wt ship` when asked to sync with the remote).

- If it stops on a rebase conflict: inspect, resolve, `git add`, `git rebase
  --continue`, then re-run the same `wt` command.
- If it stops on dirty state: commit or stash as appropriate, then re-run.

Never non-ff merge into trunk. Never push unless the user asked for that
(use `wt ship`, not a manual `git push`).

`wt --help` documents all commands.
