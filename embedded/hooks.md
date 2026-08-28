The .wt/ directory (all optional)

Committed to the repo, so versioned and present in every worktree. `.gwt/`
(the old layout) is never read — no migration, just rename the directory and
drop `env`.

.wt/create
    When: after `git worktree add` and any configured copy-on-write cloning,
    cwd = the new worktree.
    Contract: $1 = absolute path of the main checkout. Make the directory
    runnable. A failing create hook reports the failure but leaves the
    worktree in place — inspect and fix, or `wt rm` to abandon it.

.wt/destroy
    When: before `git worktree remove`, cwd = the worktree being removed.
    Contract: cleanup. Failure warns but never blocks removal.

.wt/config
    When: read by the binary.
    Contract: `key = value` lines, `#` comments. `worktrees = <path>` sets
    the worktree directory; relative paths resolve from the main checkout.
    The default is `.wt/worktrees`. `carry_ignored = true` enables carrying
    ignored files. `persistent = true` keeps worktrees after successful merges.

.wt/ignore
    When: before ignored files are cloned into a new worktree.
    Contract: gitignore-style patterns for ignored files that should stay
    behind. This affects wt's carrying only; it does not change Git status.

When the worktrees directory is inside the main checkout, wt adds it to
`.git/info/exclude` so creating a worktree does not dirty the project or
change its tracked `.gitignore`.

Ignored files stay behind by default. Set `carry_ignored = true` in `.wt/config`
to clone them into new worktrees on APFS and Linux filesystems with reflink
support. They share disk blocks at first but become independent when changed.
Without reflinks, ignored files remain absent and the hook can install or copy
anything the project requires. List selective exclusions in `.wt/ignore`.

`.wt/create` and `.wt/destroy` only run if they exist AND are executable
(`chmod +x`); a non-executable file is silently skipped.

Persistence resolution, first match wins:

  1. --rm / --keep flag on `wt done` / `wt ship`
  2. git config branch.<name>.wt-persist (set by `wt persist`, or
     `wt create --persist` at creation time)
  3. persistent = true in .wt/config
  4. default: ephemeral (torn down after a successful `wt done`/`wt ship`)

See `wt init` to scaffold example hook files, and `wt prompt` for a
ready-made prompt that has an agent write real ones for this project.
