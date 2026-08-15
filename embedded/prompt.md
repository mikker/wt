You are setting up this project for `wt`, a CLI that manages ephemeral git
worktrees (spawns them, runs project hooks, merges work back to trunk, tears
them down). `wt` is already installed. Your job is to make a freshly created
worktree immediately runnable, with no manual steps.

Do this:

1. Read the hook contract: run `wt --help` and `wt help hooks`. The short
   version — `.wt/` is an optional directory, committed to the repo:
   - `.wt/create` runs after `git worktree add`, cwd = the new worktree,
     `$1` = absolute path of the main checkout. Must be executable.
   - `.wt/destroy` runs before `git worktree remove`, cwd = the worktree
     being removed. Must be executable. Failure warns but never blocks
     removal.
   - `.wt/config` holds `key = value` lines (currently just
     `persistent = true`).

2. Inspect this project and write `.wt/create` so a brand-new worktree can
   run the app/tests immediately. Typical things it needs to do:
   - Install dependencies (`bundle install`, `npm ci`, `go mod download`,
     etc.) if they aren't shared across worktrees already.
   - Copy secrets or keys that exist in the main checkout but aren't in git
     (e.g. `config/master.key`, `.env`) — read them from `$1`, the main
     checkout path passed as the first argument.
   - Symlink or otherwise redirect any state you want shared rather than
     duplicated per worktree (e.g. a `node_modules` symlink back to the main
     checkout, a shared `.env`, a shared local database).
   - Anything else `git worktree add` doesn't give you for free.

   Make `.wt/create` executable (`chmod +x .wt/create`).

3. Only write `.wt/destroy` if teardown needs real work beyond
   `git worktree remove` — e.g. killing a background server the create hook
   started, dropping a per-worktree database. If there's nothing to clean up,
   skip it entirely; don't write a no-op script. Make it executable if you
   do write it.

4. Prove it works end-to-end:
   - `wt create tmp-<something>` to create a throwaway worktree.
   - Verify the app/tests actually run there (whatever "runs" means for this
     project — boot the server, run the test suite, etc.).
   - `wt rm tmp-<something>` to tear it down again.
   - Fix `.wt/create`/`.wt/destroy` and repeat until this is clean.

5. If `.wt/create` takes long enough that spinning up a fresh worktree per
   task would be annoying (slow installs, long migrations), consider adding
   `persistent = true` to `.wt/config` so `wt done`/`wt ship` rebase the
   worktree in place instead of tearing it down after every merge.

When you're done, the project should support: create a worktree, have it
just work, merge back with `wt done`/`wt ship`, and never require a human to
manually patch up a worktree before it's usable.
